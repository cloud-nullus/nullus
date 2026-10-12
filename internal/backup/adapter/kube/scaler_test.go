package kube

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

func i32(v int32) *int32 { return &v }

func deployment(name string, spec, status int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "nullus"},
		Spec:       appsv1.DeploymentSpec{Replicas: i32(spec)},
		Status:     appsv1.DeploymentStatus{Replicas: status},
	}
}

func statefulSet(name string, spec, status int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "nullus"},
		Spec:       appsv1.StatefulSetSpec{Replicas: i32(spec)},
		Status:     appsv1.StatefulSetStatus{Replicas: status},
	}
}

func TestList_두_종류를_모두_모은다(t *testing.T) {
	c := fake.NewSimpleClientset(
		deployment("gitlab", 3, 3),
		statefulSet("openbao", 1, 1),
	)
	s := NewWorkloadScaler(c)

	got, err := s.List(context.Background(), "nullus")
	require.NoError(t, err)
	require.Len(t, got, 2)

	kinds := map[string]int32{}
	for _, w := range got {
		kinds[w.Kind+"/"+w.Name] = w.Replicas
	}
	assert.Equal(t, int32(3), kinds["Deployment/gitlab"])
	assert.Equal(t, int32(1), kinds["StatefulSet/openbao"])
}

func TestList_replicas_가_nil_이면_0_으로_읽는다(t *testing.T) {
	// nil 을 1 로 오해하면 정지 계획이 실제와 어긋난다.
	d := deployment("no-replicas", 0, 0)
	d.Spec.Replicas = nil
	s := NewWorkloadScaler(fake.NewSimpleClientset(d))

	got, err := s.List(context.Background(), "nullus")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int32(0), got[0].Replicas)
}

func TestList_빈_네임스페이스(t *testing.T) {
	got, err := NewWorkloadScaler(fake.NewSimpleClientset()).List(context.Background(), "nullus")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestScale_지원하지_않는_종류는_거부한다(t *testing.T) {
	// DaemonSet 은 replica 개념이 없다. 조용히 무시하면 그 파드가 볼륨을
	// 잡은 채로 남아 정지 백업이 깨진다.
	s := NewWorkloadScaler(fake.NewSimpleClientset())
	err := s.Scale(context.Background(), domain.QuiesceTarget{
		Kind: "DaemonSet", Namespace: "nullus", Name: "otel-agent",
	}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DaemonSet")
}

func TestWaitStopped_파드가_없으면_즉시_반환(t *testing.T) {
	c := fake.NewSimpleClientset(deployment("gitlab", 0, 0))
	s := NewWorkloadScaler(c)

	done := make(chan error, 1)
	go func() {
		done <- s.WaitStopped(context.Background(), "nullus",
			[]domain.QuiesceTarget{{Kind: "Deployment", Namespace: "nullus", Name: "gitlab"}})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("status.Replicas 가 0 인데 기다렸다")
	}
}

func TestWaitStopped_대상이_없으면_즉시_반환(t *testing.T) {
	s := NewWorkloadScaler(fake.NewSimpleClientset())
	require.NoError(t, s.WaitStopped(context.Background(), "nullus", nil))
}

// 파드가 남아 있는 채로 복사하면 정지 백업의 의미가 없다.
func TestWaitStopped_파드가_남아_있으면_시간초과로_실패한다(t *testing.T) {
	c := fake.NewSimpleClientset(deployment("gitlab", 0, 2)) // spec=0 인데 아직 2개 떠 있다
	s := NewWorkloadScaler(c)
	s.waitTimeout = 300 * time.Millisecond

	err := s.WaitStopped(context.Background(), "nullus",
		[]domain.QuiesceTarget{{Kind: "Deployment", Namespace: "nullus", Name: "gitlab"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "남은 replica 2")
	assert.Contains(t, err.Error(), "쓰기가 멈추지 않은")
}

func TestWaitStopped_컨텍스트_취소(t *testing.T) {
	c := fake.NewSimpleClientset(deployment("gitlab", 0, 1))
	s := NewWorkloadScaler(c)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := s.WaitStopped(ctx, "nullus",
		[]domain.QuiesceTarget{{Kind: "Deployment", Namespace: "nullus", Name: "gitlab"}})
	require.Error(t, err)
}

func TestWaitStopped_없는_워크로드는_오류다(t *testing.T) {
	// 조회 실패를 "0개 남았다" 로 오해하면 살아 있는 파드 위에서 복사한다.
	s := NewWorkloadScaler(fake.NewSimpleClientset())
	err := s.WaitStopped(context.Background(), "nullus",
		[]domain.QuiesceTarget{{Kind: "Deployment", Namespace: "nullus", Name: "없음"}})
	require.Error(t, err)
}

// 오퍼레이터가 만든 StatefulSet 은 ownerReferences 를 갖는다. 그 표시를 목록에 싣는다 — 복구의
// 재개 단계가 그것을 건너뛸 근거다(오퍼레이터가 CR 에서 다시 만든다).
func TestList_소유자가_있는_워크로드를_표시한다(t *testing.T) {
	owned := statefulSet("prometheus-kps", 1, 1)
	owned.OwnerReferences = []metav1.OwnerReference{{APIVersion: "monitoring.coreos.com/v1", Kind: "Prometheus", Name: "kps"}}
	c := fake.NewSimpleClientset(deployment("gitlab", 2, 2), owned)

	got, err := NewWorkloadScaler(c).List(context.Background(), "nullus")
	require.NoError(t, err)
	byName := map[string]domain.Workload{}
	for _, w := range got {
		byName[w.Name] = w
	}
	assert.True(t, byName["prometheus-kps"].OwnerManaged)
	assert.False(t, byName["gitlab"].OwnerManaged)
}

// 리소스 단계가 replicas 를 세운 직후 재개가 같은 객체를 조정하면 "the object has been modified" 가
// 난다 — 2026-10-12 리허설에서 minio·prometheus-operator 가 그랬다. 최신 것을 다시 읽어 재시도하면
// 지나가는 충돌이다. 수동 조치가 필요하다는 경보를 낼 일이 아니다.
// scaleReactors 는 fake 클라이언트에 scale 하위리소스를 가르친다 — 기본 반응은 GetScale 에 Deployment
// 객체를 돌려주어 타입 변환에서 죽는다. onUpdate 가 오류를 돌려주면 그 호출은 실패한다.
func scaleReactors(c *fake.Clientset, onUpdate func() error) {
	c.PrependReactor("get", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		g := action.(k8stesting.GetAction)
		return true, &autoscalingv1.Scale{ObjectMeta: metav1.ObjectMeta{Name: g.GetName(), Namespace: g.GetNamespace()}}, nil
	})
	c.PrependReactor("update", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		if err := onUpdate(); err != nil {
			return true, nil, err
		}
		return true, action.(k8stesting.UpdateAction).GetObject(), nil
	})
}

func TestScale_충돌이면_다시_읽어_재시도한다(t *testing.T) {
	c := fake.NewSimpleClientset(deployment("minio", 0, 0))
	conflicts := 0
	scaleReactors(c, func() error {
		if conflicts < 2 {
			conflicts++
			return apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: "deployments"}, "minio",
				errors.New("the object has been modified; please apply your changes to the latest version and try again"))
		}
		return nil
	})
	s := NewWorkloadScaler(c)

	err := s.Scale(context.Background(), domain.QuiesceTarget{Kind: "Deployment", Namespace: "nullus", Name: "minio"}, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, conflicts, "충돌 두 번은 재시도로 넘긴다")
}

func TestScale_충돌이_계속되면_오류다(t *testing.T) {
	c := fake.NewSimpleClientset(deployment("minio", 0, 0))
	scaleReactors(c, func() error {
		return apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: "deployments"}, "minio", errors.New("modified"))
	})
	err := NewWorkloadScaler(c).Scale(context.Background(), domain.QuiesceTarget{Kind: "Deployment", Namespace: "nullus", Name: "minio"}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minio")
}
