package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 스택 삭제가 클러스터의 마지막 스택인지 볼 때 쓴다. 조직과 무관하게 같은 클러스터의 지우지 않은
// 스택만 돌려준다.
func TestMemoryStackRepository_ListByCluster(t *testing.T) {
	repo := NewMemoryStackRepository()
	ctx := context.Background()
	deleted := time.Now()
	for _, s := range []*domain.Stack{
		{ID: "stk_same_org", OrgID: "org-1", ClusterID: "cluster-a", State: domain.StateCompleted},
		{ID: "stk_other_org", OrgID: "org-2", ClusterID: "cluster-a", State: domain.StateFailed},
		{ID: "stk_other_cluster", OrgID: "org-1", ClusterID: "cluster-b", State: domain.StateCompleted},
		{ID: "stk_deleted", OrgID: "org-1", ClusterID: "cluster-a", State: domain.StateCompleted, DeletedAt: &deleted},
	} {
		require.NoError(t, repo.Create(ctx, s))
	}

	stacks, err := repo.ListByCluster(ctx, "cluster-a")
	require.NoError(t, err)

	ids := make([]string, 0, len(stacks))
	for _, s := range stacks {
		ids = append(ids, s.ID)
	}
	assert.ElementsMatch(t, []string{"stk_same_org", "stk_other_org"}, ids)
}
