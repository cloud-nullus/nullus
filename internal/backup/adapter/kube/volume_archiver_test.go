package kube

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

func pvc(name, size, sc string) *corev1.PersistentVolumeClaim {
	p := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "nullus"},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			},
		},
	}
	if sc != "" {
		p.Spec.StorageClassName = &sc
	}
	return p
}

// 설계 §1.5 — 알려진 도구 목록을 순회하면 카탈로그가 늘 때마다 뒤처진다.
// GitLab·Gitea 는 설치기가 크기를 지정하지 않아 차트 기본값을 따르므로,
// 열거만이 실제 상태를 따라간다.
func TestListPVCs_이름을_모르는_볼륨도_잡아낸다(t *testing.T) {
	c := fake.NewSimpleClientset(
		pvc("data-gitlab", "20Gi", "local-path"),
		pvc("repo-data-gitea-0", "10Gi", ""),
		pvc("차트가-멋대로-만든-볼륨", "1Gi", "local-path"),
	)
	a := NewVolumeArchiver(c, nil, "")

	got, err := a.ListPVCs(context.Background(), "nullus")
	require.NoError(t, err)
	require.Len(t, got, 3, "설치기가 모르는 PVC 도 포함돼야 한다")

	byName := map[string]domain.VolumeSpec{}
	for _, v := range got {
		byName[v.Name] = v
	}
	assert.Equal(t, int64(20*1024*1024*1024), byName["data-gitlab"].SizeBytes)
	assert.Equal(t, "local-path", byName["data-gitlab"].StorageClass)
	assert.Empty(t, byName["repo-data-gitea-0"].StorageClass, "미지정은 빈 문자열")
}

func TestListPVCs_빈_네임스페이스(t *testing.T) {
	got, err := NewVolumeArchiver(fake.NewSimpleClientset(), nil, "").ListPVCs(context.Background(), "nullus")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestEnsurePVC_매니페스트대로_만든다(t *testing.T) {
	c := fake.NewSimpleClientset()
	a := NewVolumeArchiver(c, nil, "")

	spec := domain.VolumeSpec{Name: "data-gitlab", SizeBytes: 20 * 1024 * 1024 * 1024, StorageClass: "local-path"}
	require.NoError(t, a.EnsurePVC(context.Background(), "nullus", spec, ""))

	created, err := c.CoreV1().PersistentVolumeClaims("nullus").Get(context.Background(), "data-gitlab", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "local-path", *created.Spec.StorageClassName)
	q := created.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.Equal(t, spec.SizeBytes, q.Value())
}

func TestEnsurePVC_이미_있으면_건드리지_않는다(t *testing.T) {
	// 기존 PVC 를 덮어쓰면 그 안의 데이터가 날아간다.
	existing := pvc("data-gitlab", "50Gi", "fast")
	c := fake.NewSimpleClientset(existing)
	a := NewVolumeArchiver(c, nil, "")

	require.NoError(t, a.EnsurePVC(context.Background(), "nullus",
		domain.VolumeSpec{Name: "data-gitlab", SizeBytes: 1, StorageClass: "slow"}, ""))

	got, err := c.CoreV1().PersistentVolumeClaims("nullus").Get(context.Background(), "data-gitlab", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "fast", *got.Spec.StorageClassName, "기존 것이 그대로여야 한다")
}

func TestEnsurePVC_StorageClass_미지정(t *testing.T) {
	c := fake.NewSimpleClientset()
	require.NoError(t, NewVolumeArchiver(c, nil, "").EnsurePVC(context.Background(), "nullus",
		domain.VolumeSpec{Name: "v", SizeBytes: 1024}, ""))

	got, err := c.CoreV1().PersistentVolumeClaims("nullus").Get(context.Background(), "v", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Nil(t, got.Spec.StorageClassName, "미지정이면 클러스터 기본값을 쓴다")
}

func TestHelperPodName_63자를_넘지_않는다(t *testing.T) {
	// 쿠버네티스 이름 상한이다. 넘으면 파드 생성이 거부되고 그 PVC 만
	// 조용히 백업에서 빠진다.
	long := strings.Repeat("a", 100)
	name := helperPodName(long)
	assert.LessOrEqual(t, len(name), 63)
	assert.NotEqual(t, "-", name[len(name)-1:], "이름은 하이픈으로 끝날 수 없다")
}

func TestHelperPodName_짧은_이름(t *testing.T) {
	assert.Equal(t, "nullus-backup-data", helperPodName("data"))
}

func TestNewVolumeArchiver_기본_이미지는_번들에_있는_것(t *testing.T) {
	// 에어갭에서 별도 반입이 필요 없어야 한다 (airgap/images/images.txt).
	a := NewVolumeArchiver(fake.NewSimpleClientset(), nil, "")
	assert.Equal(t, "busybox:1.37", a.image)
}

// 헬퍼 파드가 안 뜨면 조용히 빈 아카이브를 남기는 대신 실패해야 한다 —
// 빈 아카이브는 "백업이 있다" 는 가장 비싼 착각을 만든다.
func TestArchive_헬퍼_파드가_안_뜨면_실패한다(t *testing.T) {
	c := fake.NewSimpleClientset(pvc("v", "1Gi", ""))
	a := NewVolumeArchiver(c, nil, "")

	// fake clientset 은 파드를 Running 으로 올려주지 않는다. 짧은 컨텍스트로
	// 대기 루프를 끊어 "기다리다 실패한다" 는 경로를 확인한다.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var sb strings.Builder
	n, err := a.Archive(ctx, "nullus", "v", &sb)
	require.Error(t, err)
	assert.Zero(t, n, "실패했으면 바이트 수도 0 이어야 한다")
	assert.Empty(t, sb.String())
}

func TestArchive_헬퍼_파드를_반드시_정리한다(t *testing.T) {
	// 남은 파드가 PVC 를 잡고 있으면 다음 백업의 정지가 헛돈다.
	c := fake.NewSimpleClientset(pvc("v", "1Gi", ""))
	a := NewVolumeArchiver(c, nil, "")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _ = a.Archive(ctx, "nullus", "v", &strings.Builder{})

	pods, err := c.CoreV1().Pods("nullus").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, pods.Items, "실패해도 헬퍼 파드는 남지 않아야 한다")
}

func TestCountingWriter(t *testing.T) {
	var sb strings.Builder
	c := &countingWriter{w: &sb}
	_, _ = c.Write([]byte("hello"))
	_, _ = c.Write([]byte(" world"))
	assert.Equal(t, int64(11), c.n)
	assert.Equal(t, "hello world", sb.String())
}

func TestEnsurePVC_복구_출처를_남긴다(t *testing.T) {
	// 이 표시가 없으면 설치 preflight 가 복구된 볼륨을 "실패한 설치의 잔여" 로
	// 오인해 막는다. 복구 직후 스택을 다시 설치할 수 없게 되는 것이다.
	c := fake.NewSimpleClientset()
	require.NoError(t, NewVolumeArchiver(c, nil, "").EnsurePVC(
		context.Background(), "nullus",
		domain.VolumeSpec{Name: "gitea-shared-storage", SizeBytes: 1024}, "bk-42"))

	pvc, err := c.CoreV1().PersistentVolumeClaims("nullus").
		Get(context.Background(), "gitea-shared-storage", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "bk-42",
		pvc.Annotations[shareddomain.RestoredFromBackupAnnotation])
}

func TestEnsurePVC_출처가_없으면_표시하지_않는다(t *testing.T) {
	// 백업 ID 를 모르는 채로 빈 표시를 남기면, preflight 가 "복구본" 과
	// "출처 불명" 을 구분할 수 없게 된다.
	c := fake.NewSimpleClientset()
	require.NoError(t, NewVolumeArchiver(c, nil, "").EnsurePVC(
		context.Background(), "nullus", domain.VolumeSpec{Name: "v", SizeBytes: 1}, ""))

	pvc, err := c.CoreV1().PersistentVolumeClaims("nullus").
		Get(context.Background(), "v", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, pvc.Annotations, shareddomain.RestoredFromBackupAnnotation)
}
