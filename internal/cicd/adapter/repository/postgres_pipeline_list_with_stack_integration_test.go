//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 주기 동기화는 조직과 무관하게 스택에 묶인 파이프라인만 돈다. 스택 없이 만든
// 파이프라인은 들일 CI 서버가 없다.
func TestPostgresPipelineRepository_ListWithStack(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	stackID := "stack-" + uuid.NewString()
	_, err := pool.Exec(ctx,
		`INSERT INTO stacks (id, name, template_id, org_id, cluster_id) VALUES ($1, $2, $3, $4, $5)`,
		stackID, "Sync Stack", "gitlab-allinone-v1", orgID, clusterID)
	require.NoError(t, err)

	repo := NewPostgresPipelineRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	newPipeline := func(name, stack string) *domain.Pipeline {
		return &domain.Pipeline{
			ID: "pipeline-" + uuid.NewString(), Name: name, TemplateID: "web-backend-v1",
			OrgID: orgID, ClusterID: clusterID, Namespace: "sync-ns", AppType: domain.AppTypeBackend,
			GitRepoURL: "https://github.com/cloud-nullus/draft", Status: domain.PipelineStatusActive,
			CreatedAt: now, StackID: stack,
		}
	}
	withStack := newPipeline("with-stack", stackID)
	standalone := newPipeline("standalone", "")
	require.NoError(t, repo.Create(ctx, withStack))
	require.NoError(t, repo.Create(ctx, standalone))

	got, err := repo.ListWithStack(ctx)
	require.NoError(t, err)
	ids := map[string]string{}
	for _, p := range got {
		ids[p.ID] = p.StackID
	}
	assert.Equal(t, stackID, ids[withStack.ID])
	assert.NotContains(t, ids, standalone.ID)
}
