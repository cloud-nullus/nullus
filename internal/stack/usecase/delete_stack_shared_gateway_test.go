package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 게이트웨이는 이제 스택들이 함께 쓴다. 스택 하나를 지울 때 데이터플레인을
// 걷어 가면 다른 스택의 현관까지 닫힌다 — 밖에서 들어오는 배선(DNS·ingress)이
// 통째로 죽는다.
func newSharedGatewayDeleteStack(t *testing.T) (*DeleteStack, *fakeHelmInstaller, *[]string) {
	t.Helper()

	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-shared-gw",
		Name:      "demo-stack",
		ClusterID: "cluster-shared-gw",
		Namespace: "nullus-demo-stack",
		State:     domain.StateCompleted,
	})
	installer := &fakeHelmInstaller{}
	uc := NewDeleteStack(repo, &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\n")},
		func([]byte) port.HelmInstaller { return installer })

	swept := []string{}
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		swept = append(swept, namespace)
		return nil, nil
	}
	uc.deleteResourceFunc = func(context.Context, []byte, string, string) error { return nil }
	return uc, installer, &swept
}

// 게이트웨이는 스택 것이다. 스택을 지우면 함께 회수돼야 남은 Envoy 가 떠 있지 않다.
func TestDeleteStack_UninstallsStackEnvoyGateway(t *testing.T) {
	uc, installer, _ := newSharedGatewayDeleteStack(t)

	require.NoError(t, uc.Execute(context.Background(), "stk-shared-gw"))

	assert.Contains(t, installer.uninstallCalls, "eg@nullus-demo-stack")
}

// 옛 코드가 eg 를 찾겠다고 플랫폼 네임스페이스까지 훑었다. 그 경로로
// 2026-08-20 에 플랫폼이 지워졌다 — 스택은 자기 자리만 정리한다.
func TestDeleteStack_NeverSweepsPlatformNamespace(t *testing.T) {
	uc, _, swept := newSharedGatewayDeleteStack(t)

	require.NoError(t, uc.Execute(context.Background(), "stk-shared-gw"))

	assert.NotContains(t, *swept, domain.DefaultStackNamespace)
	assert.Contains(t, *swept, "nullus-demo-stack", "자기 네임스페이스는 계속 정리한다")
}

func TestCleanupNamespaces_StaysWithinStackAndDefault(t *testing.T) {
	namespaces := cleanupNamespacesForStack("nullus-demo-stack")

	assert.Equal(t, []string{"nullus-demo-stack", "default"}, namespaces)
	assert.NotContains(t, namespaces, domain.DefaultStackNamespace)
}
