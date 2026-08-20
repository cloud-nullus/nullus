package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 2026-08-20 운영 사고 재현.
//
// 스택을 플랫폼과 같은 네임스페이스(nullus)에 깔고 지웠더니, 삭제 스윕이 이름에
// "nullus" 가 들어갔다는 이유로 플랫폼 자신의 Deployment·Service 를 지웠다.
// nullus.io 가 전면 503 이 됐고 helm upgrade 는 "deployments.apps nullus-api not
// found" 로 막혔다. 소유자가 분명한 리소스는 이름이 무엇이든 건드리면 안 된다.
func newOwnershipDeleteStack(t *testing.T, stack *domain.Stack) (*DeleteStack, *[]string) {
	t.Helper()

	repo := newFakeStackRepo(stack)
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\n")}
	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return &fakeHelmInstaller{}
	})

	deleted := []string{}
	uc.deleteResourceFunc = func(_ context.Context, _ []byte, namespace, resource string) error {
		deleted = append(deleted, namespace+":"+resource)
		return nil
	}
	return uc, &deleted
}

func ownershipStack() *domain.Stack {
	return &domain.Stack{
		ID:        "stk-ownership",
		Name:      "nullus",
		ClusterID: "cluster-ownership",
		Namespace: "nullus",
		State:     domain.StateCompleted,
	}
}

func TestDeleteStack_KeepsResourcesOwnedByAnotherHelmRelease(t *testing.T) {
	uc, deleted := newOwnershipDeleteStack(t, ownershipStack())
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "nullus" {
			return nil, nil
		}
		return []namespacedResource{
			// 플랫폼 자신. 이름에 스택 이름이 그대로 들어 있다.
			{Ref: "deployment.apps/nullus-api", HelmRelease: "nullus"},
			{Ref: "deployment.apps/nullus-web", HelmRelease: "nullus"},
			{Ref: "service/nullus-keycloak", HelmRelease: "nullus"},
			{Ref: "statefulset.apps/nullus-postgresql", HelmRelease: "nullus"},
		}, nil
	}

	require.NoError(t, uc.Execute(context.Background(), "stk-ownership"))
	assert.Empty(t, *deleted, "다른 릴리스가 소유한 리소스는 하나도 지우면 안 된다")
}

func TestDeleteStack_DeletesResourcesOwnedByStackReleases(t *testing.T) {
	uc, deleted := newOwnershipDeleteStack(t, ownershipStack())
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "nullus" {
			return nil, nil
		}
		return []namespacedResource{
			{Ref: "statefulset.apps/harbor-core", HelmRelease: "harbor"},
			{Ref: "deployment.apps/argo-cd-argocd-server", HelmRelease: "argo-cd"},
		}, nil
	}

	require.NoError(t, uc.Execute(context.Background(), "stk-ownership"))
	assert.Contains(t, *deleted, "nullus:statefulset.apps/harbor-core")
	assert.Contains(t, *deleted, "nullus:deployment.apps/argo-cd-argocd-server")
}

func TestDeleteStack_DeletesResourcesLabeledForThisStack(t *testing.T) {
	uc, deleted := newOwnershipDeleteStack(t, ownershipStack())
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "nullus" {
			return nil, nil
		}
		return []namespacedResource{
			{Ref: "configmap/opensearch-root-ca", StackLabel: "nullus"},
			{Ref: "configmap/other-stack-config", StackLabel: "other-stack"},
		}, nil
	}

	require.NoError(t, uc.Execute(context.Background(), "stk-ownership"))
	assert.Contains(t, *deleted, "nullus:configmap/opensearch-root-ca")
	assert.NotContains(t, *deleted, "nullus:configmap/other-stack-config",
		"다른 스택이 라벨로 소유를 밝힌 리소스는 건드리지 않는다")
}

// 이름에 스택 이름이 들어갔다는 이유만으로 지우던 규칙이 사고의 직접 원인이었다.
func TestDeleteStack_DoesNotDeleteByStackNameSubstringAlone(t *testing.T) {
	uc, deleted := newOwnershipDeleteStack(t, ownershipStack())
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "nullus" {
			return nil, nil
		}
		return []namespacedResource{
			// 소유자를 밝히지 않았고 레거시 잔재 목록에도 없다.
			{Ref: "deployment.apps/nullus-dashboard-extra"},
			{Ref: "service/some-nullus-thing"},
		}, nil
	}

	require.NoError(t, uc.Execute(context.Background(), "stk-ownership"))
	assert.Empty(t, *deleted)
}

// 옛 설치가 남긴 고아(소유자 표시가 없는 것)는 계속 이름으로 정리한다.
func TestDeleteStack_StillCleansUnownedLegacyArtifacts(t *testing.T) {
	stack := ownershipStack()
	stack.ID = "stk-legacy-orphans"
	stack.Name = "gitlab-argocd-v1"
	stack.Namespace = "devsecops"
	uc, deleted := newOwnershipDeleteStack(t, stack)
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "devsecops" {
			return nil, nil
		}
		return []namespacedResource{
			{Ref: "secret/gitlab-gitlab-initial-root-password"},
			{Ref: "pvc/data-nullus-postgresql-0"},
			{Ref: "serviceaccount/default"},
			{Ref: "configmap/kube-root-ca.crt"},
		}, nil
	}

	require.NoError(t, uc.Execute(context.Background(), "stk-legacy-orphans"))
	assert.Contains(t, *deleted, "devsecops:secret/gitlab-gitlab-initial-root-password")
	assert.Contains(t, *deleted, "devsecops:pvc/data-nullus-postgresql-0")
	assert.NotContains(t, *deleted, "devsecops:serviceaccount/default")
	assert.NotContains(t, *deleted, "devsecops:configmap/kube-root-ca.crt")
}

// 마지막 안전망. 플랫폼이 사는 네임스페이스에서는 이름 기반 청소를 아예 하지 않는다.
func TestDeleteStack_SkipsNameBasedSweepInPlatformNamespace(t *testing.T) {
	uc, deleted := newOwnershipDeleteStack(t, ownershipStack())
	uc.SetPlatformNamespace("nullus")
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "nullus" {
			return nil, nil
		}
		return []namespacedResource{
			{Ref: "secret/gitlab-gitlab-initial-root-password"},
			{Ref: "pvc/data-nullus-postgresql-0"},
		}, nil
	}

	require.NoError(t, uc.Execute(context.Background(), "stk-ownership"))
	assert.Empty(t, *deleted, "플랫폼 네임스페이스에서는 고아 청소도 하지 않는다")
}
