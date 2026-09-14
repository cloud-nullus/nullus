package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치 이미지 재스캔은 조직과 무관하게 설치가 끝난 스택만 돈다.
func TestMemoryStackRepository_ListCompleted(t *testing.T) {
	repo := NewMemoryStackRepository()
	ctx := context.Background()
	deleted := time.Now()
	for _, s := range []*domain.Stack{
		{ID: "stk_done_1", OrgID: "org-1", State: domain.StateCompleted},
		{ID: "stk_done_2", OrgID: "org-2", State: domain.StateCompleted},
		{ID: "stk_installing", OrgID: "org-1", State: domain.StateInstalling},
		{ID: "stk_deleted", OrgID: "org-1", State: domain.StateCompleted, DeletedAt: &deleted},
	} {
		require.NoError(t, repo.Create(ctx, s))
	}

	got, err := repo.ListCompleted(ctx)
	require.NoError(t, err)

	ids := make([]string, 0, len(got))
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	assert.ElementsMatch(t, []string{"stk_done_1", "stk_done_2"}, ids)
}
