package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 주기 동기화는 조직과 무관하게 스택에 묶인 파이프라인만 돈다.
func TestMemoryPipelineRepository_ListWithStack(t *testing.T) {
	repo := NewMemoryPipelineRepository()
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, &domain.Pipeline{ID: "pip_a", OrgID: "org-1", StackID: "stk_1"}))
	require.NoError(t, repo.Create(ctx, &domain.Pipeline{ID: "pip_b", OrgID: "org-2", StackID: "stk_2"}))
	require.NoError(t, repo.Create(ctx, &domain.Pipeline{ID: "pip_c", OrgID: "org-1"}))

	got, err := repo.ListWithStack(ctx)
	require.NoError(t, err)

	ids := make([]string, 0, len(got))
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	assert.ElementsMatch(t, []string{"pip_a", "pip_b"}, ids)
}
