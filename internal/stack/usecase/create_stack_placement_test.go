package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func newPlacementCreateStack(opts ...CreateStackOption) *CreateStack {
	return NewCreateStack(stackrepo.NewMemoryStackRepository(), stackrepo.NewMemoryTemplateRepository(), opts...)
}

// 스택은 플랫폼과 다른 네임스페이스에 선다. 같은 곳에 세우면 설치는 Helm 소유권
// 충돌로 실패하고, 삭제는 플랫폼 리소스를 건드린다.
func TestCreateStack_DefaultsToStackScopedNamespace(t *testing.T) {
	uc := newPlacementCreateStack()

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "gitea-jenkins-v1",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "nullus-gitea-jenkins-v1", out.Stack.Namespace)
	assert.NotEqual(t, domain.DefaultStackNamespace, out.Stack.Namespace)
}

// 2026-08-20 운영 사고의 마지막 방어선. 사용자가 직접 골라도 막는다.
func TestCreateStack_RejectsPlatformNamespace(t *testing.T) {
	uc := newPlacementCreateStack(WithPlatformNamespace("nullus"))

	_, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "any-stack",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nullus")
}

func TestCreateStack_AllowsOtherNamespacesWhenPlatformNamespaceKnown(t *testing.T) {
	uc := newPlacementCreateStack(WithPlatformNamespace("nullus"))

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "any-stack",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "team-a",
	})

	require.NoError(t, err)
	assert.Equal(t, "team-a", out.Stack.Namespace)
}
