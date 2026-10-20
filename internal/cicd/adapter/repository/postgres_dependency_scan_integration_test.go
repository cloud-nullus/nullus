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

func createScanTestPipeline(t *testing.T, ctx context.Context, repo *PostgresPipelineRepository, orgID, clusterID, stackID, name string, stages []string) string {
	t.Helper()
	id := "pipeline-" + uuid.NewString()
	require.NoError(t, repo.Create(ctx, &domain.Pipeline{
		ID: id, Name: name, TemplateID: "web-backend-v1", OrgID: orgID, ClusterID: clusterID, StackID: stackID,
		Namespace: name, AppType: domain.AppTypeBackend, GitRepoURL: "https://github.com/cloud-nullus/draft",
		Status: domain.PipelineStatusActive, CreatedAt: time.Now().UTC().Truncate(time.Microsecond), Stages: stages,
	}))
	return id
}

func TestPostgresDependencyScanResultRepository_RoundTrip(t *testing.T) {
	t.Parallel()
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	pipelineID := createScanTestPipeline(t, ctx, NewPostgresPipelineRepository(pool), orgID, clusterID, "", "dep-scan", nil)
	repo := NewPostgresDependencyScanResultRepository(pool)
	at := time.Now().UTC().Truncate(time.Microsecond)
	dbAt := at.Add(-48 * time.Hour)
	targets := 2

	full := &domain.DependencyScanResult{
		ID: domain.DependencyScanResultID("dep_1"), PipelineID: pipelineID, DeploymentID: "dep_1",
		Scanner: "trivy", ScannerVersion: "0.58.1", DBUpdatedAt: &dbAt,
		Counts:     &domain.SeverityCounts{Critical: 3, High: 5, Medium: 12},
		Targets:    &targets,
		GateResult: domain.GateResultBlock, ReportURI: "https://gitlab.example.com/-/jobs/7/artifacts/raw/dependency-scan-report.json",
		ScannedAt: at,
		ReportRef: &domain.ScanReportRef{JobName: "app", BuildNumber: 7, StageID: "41", Artifact: "dependency-scan-report", Path: "dependency-scan-report.json"},
	}
	require.NoError(t, repo.Upsert(ctx, full))

	// 리포트가 없는 실행 — 건수 · 검사 대상 수를 모른다. 0 으로 되읽히면 "취약점 0건 · 대상 없음" 으로 보인다.
	missing := &domain.DependencyScanResult{
		ID: domain.DependencyScanResultID("dep_2"), PipelineID: pipelineID, DeploymentID: "dep_2",
		Scanner: "trivy", GateResult: domain.GateResultError, ScannedAt: at.Add(time.Minute),
	}
	require.NoError(t, repo.Upsert(ctx, missing))

	// 락파일이 없어 검사 대상이 0 인 실행 — 0 은 모름과 다르다.
	zero := 0
	empty := &domain.DependencyScanResult{
		ID: domain.DependencyScanResultID("dep_0"), PipelineID: pipelineID, DeploymentID: "dep_0",
		Scanner: "trivy", Counts: &domain.SeverityCounts{}, Targets: &zero,
		GateResult: domain.GateResultPass, ScannedAt: at.Add(-time.Minute),
	}
	require.NoError(t, repo.Upsert(ctx, empty))

	list, err := repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, missing, list[0], "최신부터")
	assert.Equal(t, full, list[1])
	assert.Equal(t, empty, list[2])
	require.NotNil(t, list[2].Targets)
	assert.Zero(t, *list[2].Targets)

	// 같은 실행을 다시 쓰면 덮어쓴다.
	full.GateResult = domain.GateResultWarn
	require.NoError(t, repo.Upsert(ctx, full))
	list, err = repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, domain.GateResultWarn, list[1].GateResult)

	// 취소된 실행의 기록은 지운다. 없는 것을 지워도 오류가 아니다.
	require.NoError(t, repo.Delete(ctx, missing.ID))
	require.NoError(t, repo.Delete(ctx, missing.ID))
	list, err = repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// 판정 어휘 밖의 값은 DB 도 받지 않는다.
	_, err = pool.Exec(ctx, `UPDATE dependency_scan_results SET gate_result = 'maybe' WHERE id = $1`, full.ID)
	assert.Error(t, err)

	// 파이프라인을 지우면 결과도 지워진다.
	_, err = pool.Exec(ctx, `DELETE FROM pipelines WHERE id = $1`, pipelineID)
	require.NoError(t, err)
	list, err = repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	assert.Empty(t, list)
}
