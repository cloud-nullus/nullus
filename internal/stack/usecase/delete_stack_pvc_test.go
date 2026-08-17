package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// PVC 는 스택을 지워도 남는다.
//
// helm uninstall 은 PVC 를 지우지 않는다 — StatefulSet 의 volumeClaimTemplate 이
// 만든 것은 아예 릴리스 소유가 아니고, 차트가 직접 만든 것도 대개 남긴다. 그래서
// 라벨 기반 정리(bestEffortDeleteStackLabeledResources)에 pvc 가 들어 있는데도
// 하나도 지워지지 않았다 — 그 PVC 들에는 nullus.io/stack-name 라벨이 없기
// 때문이다. 실측한 라벨은 Helm 차트 것뿐이었다:
//
//	data-harbor-redis-0            release=harbor, heritage=Helm
//	gitea-shared-storage           app.kubernetes.io/managed-by=Helm  ← 릴리스 라벨조차 없다
//
// 릴리스 라벨로도 전부 잡을 수 없으므로 스택 네임스페이스를 통째로 훑는다.
func TestDeleteStack_DeletesPersistentVolumeClaimsInStackNamespace(t *testing.T) {
	rec := newKubectlRecorder()
	uc := newGatewayDeleteStack(t, rec, &domain.Stack{
		ID:        "stk-pvc",
		Name:      "nullus-lite",
		ClusterID: "cluster-pvc",
		Namespace: "nullus-lite",
		State:     domain.StateCompleted,
	})

	require.NoError(t, uc.Execute(context.Background(), "stk-pvc"))

	assert.True(t, rec.has("delete pvc", "-n nullus-lite", "--all"),
		"스택 네임스페이스의 PVC 를 지우지 않으면 설치·삭제를 반복할 때마다 디스크가 쌓인다.\n호출: %v", rec.calls)
}

// 공유 네임스페이스는 통째로 훑지 않는다. cleanupNamespacesForStack 이 함께 도는
// default·nullus·envoy-gateway-system 에는 다른 스택이나 사용자의 PVC 가 있을 수
// 있고, 거기서 --all 로 지우면 남의 데이터를 파기한다.
func TestDeleteStack_DoesNotWipePVCsInSharedNamespaces(t *testing.T) {
	rec := newKubectlRecorder()
	uc := newGatewayDeleteStack(t, rec, &domain.Stack{
		ID:        "stk-pvc-scope",
		Name:      "nullus-lite",
		ClusterID: "cluster-pvc",
		Namespace: "nullus-lite",
		State:     domain.StateCompleted,
	})

	require.NoError(t, uc.Execute(context.Background(), "stk-pvc-scope"))

	for _, call := range rec.calls {
		if !strings.HasPrefix(call, "delete pvc") || !strings.Contains(call, "--all") {
			continue
		}
		assert.Contains(t, call, "-n nullus-lite",
			"PVC 전체 삭제는 스택 네임스페이스로 한정해야 한다: %s", call)
	}
}

// 스택 네임스페이스가 비어 있으면(구성이 망가진 스택) 아무 데도 손대지 않는다.
// 네임스페이스를 모른 채 --all 을 던지면 현재 컨텍스트의 기본 네임스페이스가
// 대상이 된다.
func TestDeleteStack_SkipsPVCDeleteWithoutNamespace(t *testing.T) {
	rec := newKubectlRecorder()
	uc := NewDeleteStack(
		newFakeStackRepo(&domain.Stack{
			ID:        "stk-pvc-nons",
			Name:      "no-namespace",
			ClusterID: "cluster-pvc",
			State:     domain.StateCompleted,
		}),
		&fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")},
		func([]byte) port.HelmInstaller { return &fakeHelmInstaller{} },
		&captureStreamer{},
	)
	uc.runKubectlFunc = rec.run

	require.NoError(t, uc.Execute(context.Background(), "stk-pvc-nons"))

	for _, call := range rec.calls {
		assert.False(t, strings.HasPrefix(call, "delete pvc") && strings.Contains(call, "--all"),
			"네임스페이스를 모르면 PVC 를 지우지 않아야 한다: %s", call)
	}
}

// PVC 삭제는 릴리스를 내린 뒤여야 한다. StatefulSet 이 살아 있는 동안 PVC 를
// 지우면 컨트롤러가 곧바로 다시 만든다 — Gateway 건과 같은 실패 방식이다.
func TestDeleteStack_DeletesPVCsAfterReleasesAreUninstalled(t *testing.T) {
	rec := newKubectlRecorder()
	streamer := &captureStreamer{}
	uc := NewDeleteStack(
		newFakeStackRepo(&domain.Stack{
			ID:        "stk-pvc-order",
			Name:      "nullus-lite",
			ClusterID: "cluster-pvc",
			Namespace: "nullus-lite",
			State:     domain.StateCompleted,
		}),
		&fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")},
		func([]byte) port.HelmInstaller { return &fakeHelmInstaller{} },
		streamer,
	)
	uc.runKubectlFunc = rec.run

	require.NoError(t, uc.Execute(context.Background(), "stk-pvc-order"))

	lastRelease, firstPVC := -1, -1
	for i, entry := range streamer.entries {
		if entry.Step == "deleting_release" {
			lastRelease = i
		}
		if firstPVC < 0 && entry.Step == "deleting_pvc" {
			firstPVC = i
		}
	}
	require.GreaterOrEqual(t, lastRelease, 0, "릴리스 삭제 기록이 없다")
	require.GreaterOrEqual(t, firstPVC, 0, "PVC 삭제 기록이 없다")
	assert.Greater(t, firstPVC, lastRelease,
		"PVC 는 릴리스를 모두 내린 뒤 지워야 StatefulSet 이 다시 만들지 않는다")
}
