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

func TestDeleteStack_KeepsSharedEnvoyGatewayRelease(t *testing.T) {
	uc, installer, _ := newSharedGatewayDeleteStack(t)

	require.NoError(t, uc.Execute(context.Background(), "stk-shared-gw"))

	for _, call := range installer.uninstallCalls {
		assert.NotContains(t, call, "eg@",
			"Envoy Gateway 는 공용이다 — 스택 삭제가 언인스톨하면 다른 스택의 라우팅이 죽는다")
	}
}

func TestDeleteStack_NeverSweepsSharedGatewayNamespace(t *testing.T) {
	uc, _, swept := newSharedGatewayDeleteStack(t)

	require.NoError(t, uc.Execute(context.Background(), "stk-shared-gw"))

	assert.NotContains(t, *swept, domain.SharedGatewayNamespace)
	assert.Contains(t, *swept, "nullus-demo-stack", "자기 네임스페이스는 계속 정리한다")
}

// 옛 코드가 eg 를 찾으려고 플랫폼 네임스페이스까지 훑었다. 게이트웨이가 전용
// 자리로 옮겨간 이상 남의 집을 뒤질 이유가 없다.
func TestCleanupNamespaces_ExcludePlatformAndGatewayNamespaces(t *testing.T) {
	namespaces := cleanupNamespacesForStack("nullus-demo-stack")

	assert.Contains(t, namespaces, "nullus-demo-stack")
	assert.NotContains(t, namespaces, domain.SharedGatewayNamespace)
	assert.NotContains(t, namespaces, domain.DefaultStackNamespace)
}
