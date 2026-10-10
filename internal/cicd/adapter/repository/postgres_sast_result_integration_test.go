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

func TestPostgresSASTResultRepository_RoundTrip(t *testing.T) {
	t.Parallel()
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	pipelineID := "pipeline-" + uuid.NewString()
	require.NoError(t, NewPostgresPipelineRepository(pool).Create(ctx, &domain.Pipeline{
		ID: pipelineID, Name: "sast-result", TemplateID: "web-backend-v1", OrgID: orgID, ClusterID: clusterID,
		Namespace: "sast-result", AppType: domain.AppTypeBackend, GitRepoURL: "https://github.com/cloud-nullus/draft",
		Status: domain.PipelineStatusActive, CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}))
	repo := NewPostgresSASTResultRepository(pool)
	at := time.Now().UTC().Truncate(time.Microsecond)

	zero, three, ncloc := 0, 3, 85
	coverage := 12.5
	full := &domain.SASTResult{
		ID: "sast_dep_1", PipelineID: pipelineID, DeploymentID: "dep_1",
		ProjectKey: "api", AnalysisID: "an-1",
		QualityGateStatus: domain.QualityGateError, GateResult: domain.GateResultBlock,
		Conditions:   []domain.SASTCondition{{Metric: "new_violations", Comparator: "GT", Threshold: "0", Actual: "1", Status: "ERROR"}},
		Metrics:      &domain.SASTMetrics{Bugs: &zero, Vulnerabilities: &three, Coverage: &coverage, Ncloc: &ncloc},
		DashboardURL: "https://sonarqube.example.com/dashboard?id=api",
		AnalyzedAt:   at,
	}
	require.NoError(t, repo.Upsert(ctx, full))

	// 분석하지 못한 실행 — 지표는 모른다. 0 으로 되읽히면 "문제 0건" 으로 보인다.
	failed := &domain.SASTResult{
		ID: "sast_dep_2", PipelineID: pipelineID, DeploymentID: "dep_2", ProjectKey: "api",
		GateResult: domain.GateResultError, AnalyzedAt: at.Add(time.Minute),
	}
	require.NoError(t, repo.Upsert(ctx, failed))

	list, err := repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, failed, list[0], "최신 분석부터")
	assert.Equal(t, full, list[1])

	// 같은 실행을 다시 쓰면 덮어쓴다.
	full.GateResult = domain.GateResultWarn
	require.NoError(t, repo.Upsert(ctx, full))
	list, err = repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, domain.GateResultWarn, list[1].GateResult)

	// 판정 어휘 밖의 값은 DB 도 받지 않는다.
	_, err = pool.Exec(ctx, `UPDATE sast_results SET gate_result = 'maybe' WHERE id = 'sast_dep_1'`)
	assert.Error(t, err)

	// 파이프라인을 지우면 결과도 지워진다.
	_, err = pool.Exec(ctx, `DELETE FROM pipelines WHERE id = $1`, pipelineID)
	require.NoError(t, err)
	list, err = repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	assert.Empty(t, list)
}
