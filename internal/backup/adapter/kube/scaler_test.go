package kube

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

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
