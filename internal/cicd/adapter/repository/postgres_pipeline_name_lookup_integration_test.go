//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 이름으로 찾는 자원(CI 자격증명)을 지우기 전에 같은 이름을 쓰는 다른 파이프라인을 찾는다.
// 목록 조회는 최근 100개만 돌려줘, 그보다 오래된 같은 이름 레코드를 놓쳤다.
func TestPostgresPipelineRepository_OtherPipelinesNamed(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	stackID := "stack-" + uuid.NewString()
	otherStack := "stack-" + uuid.NewString()
	for _, id := range []string{stackID, otherStack} {
		_, err := pool.Exec(ctx,
			`INSERT INTO stacks (id, name, template_id, org_id, cluster_id) VALUES ($1, $2, $3, $4, $5)`,
			id, "Stack "+id, "gitea-jenkins-argocd-v1", orgID, clusterID)
		require.NoError(t, err)
	}

	repo := NewPostgresPipelineRepository(pool)
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	create := func(name, stack string, age int) *domain.Pipeline {
		p := &domain.Pipeline{
			ID: "pipeline-" + uuid.NewString(), Name: name, TemplateID: "web-backend-v1",
			OrgID: orgID, ClusterID: clusterID, Namespace: "ns", AppType: domain.AppTypeBackend,
			GitRepoURL: "https://example.com/r", Status: domain.PipelineStatusActive,
			CreatedAt: base.Add(time.Duration(age) * time.Second), StackID: stack,
		}
		require.NoError(t, repo.Create(ctx, p))
		return p
	}

	target := create("api", stackID, 500)
	oldSameName := create(" api ", stackID, 0) // 가장 오래됐다 — 최근 100개 밖
	for i := 0; i < 105; i++ {
		create(fmt.Sprintf("filler-%d", i), stackID, 10+i)
	}
	create("api", otherStack, 600) // 다른 스택은 상관없다

	got, err := repo.OtherPipelinesNamed(ctx, stackID, "api", target.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{oldSameName.ID}, got)

	none, err := repo.OtherPipelinesNamed(ctx, stackID, "filler-3", "")
	require.NoError(t, err)
	assert.Len(t, none, 1)
}
