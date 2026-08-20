package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 2026-08-20 운영 사고.
//
// 스택을 지웠는데 PVC 가 남았고(파드가 아직 물고 있어 삭제가 타임아웃), 경고 한
// 줄만 남긴 채 삭제가 성공으로 끝났다. 사용자는 깨끗이 지워진 줄 알고 같은
// 네임스페이스에 다시 설치했고, 옛 데이터베이스를 물려받은 PostgreSQL 의
// 비밀번호가 새 Secret 과 어긋나 Gitea 가 28P01 로 기동하지 못했다.
func TestRemainingPVCMessage_SaysWhatItBreaksAndHowToFix(t *testing.T) {
	message := remainingPVCMessage("nullus-demo", []string{"data-nullus-postgresql-0", "gitea-shared-storage"})

	assert.Contains(t, message, "nullus-demo")
	assert.Contains(t, message, "data-nullus-postgresql-0")
	assert.Contains(t, message, "gitea-shared-storage")
	// 무엇이 깨지는지
	assert.Contains(t, message, "옛 데이터베이스")
	// 어떻게 고치는지
	assert.Contains(t, message, "kubectl -n nullus-demo delete pvc data-nullus-postgresql-0 gitea-shared-storage")
}

func TestParseResourceNames_ReadsKubectlNameOutput(t *testing.T) {
	names := parseResourceNames("persistentvolumeclaim/data-nullus-postgresql-0\npersistentvolumeclaim/gitea-shared-storage\n")

	assert.Equal(t, []string{"data-nullus-postgresql-0", "gitea-shared-storage"}, names)
}

func TestParseResourceNames_EmptyOutputYieldsNothing(t *testing.T) {
	assert.Empty(t, parseResourceNames("   \n"))
}

// PVC 는 그것을 마운트한 파드가 살아 있는 동안 finalizer 로 남는다. 첫 삭제는
// 파드가 빠지기 전에 끝나 타임아웃이 나고, 예전에는 거기서 포기했다.
func TestDeleteStack_RetriesUntilVolumesAreGone(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-pvc-retry",
		Name:      "demo-stack",
		ClusterID: "cluster-pvc-retry",
		Namespace: "nullus-demo-stack",
		State:     domain.StateCompleted,
	})
	uc := NewDeleteStack(repo, &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\n")},
		func([]byte) port.HelmInstaller { return &fakeHelmInstaller{} })
	uc.listResourcesFunc = func(context.Context, []byte, string) ([]namespacedResource, error) { return nil, nil }
	uc.deleteResourceFunc = func(context.Context, []byte, string, string) error { return nil }

	// kubectl 을 갈아끼울 수 없으므로 재시도 함수가 컨텍스트 취소에 즉시 반응하는지
	// 본다 — 이것이 없으면 삭제가 최대 1분을 붙잡고 있게 된다.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		uc.retryDeletePersistentVolumeClaims(ctx, []byte("apiVersion: v1\n"), "nullus-demo-stack", "stk-pvc-retry")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("취소된 컨텍스트에서 곧바로 돌아오지 않았다")
	}
}
