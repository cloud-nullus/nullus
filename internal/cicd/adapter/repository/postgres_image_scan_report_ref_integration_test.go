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

func TestPostgresImageScanResultRepository_ReportRefAndGetByID(t *testing.T) {
	t.Parallel()
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	pipelineID := "pipeline-" + uuid.NewString()
	require.NoError(t, NewPostgresPipelineRepository(pool).Create(ctx, &domain.Pipeline{
		ID: pipelineID, Name: "scan-ref", TemplateID: "web-backend-v1", OrgID: orgID, ClusterID: clusterID,
		Namespace: "scan-ref", AppType: domain.AppTypeBackend, GitRepoURL: "https://github.com/cloud-nullus/draft",
		Status: domain.PipelineStatusActive, CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}))

	repo := NewPostgresImageScanResultRepository(pool)
	counts := domain.SeverityCounts{High: 2}
	ref := &domain.ScanReportRef{JobName: "app", Branch: "main", BuildID: "run-1", BuildNumber: 7,
		StageID: "11", StageName: "image-scan", Artifact: "trivy-report", Path: "trivy-report.json"}
	require.NoError(t, repo.Upsert(ctx, &domain.ImageScanResult{
		ID: "scan_ref_1", PipelineID: pipelineID, ScanSource: "central", Scanner: "trivy",
		Counts: &counts, GateResult: domain.GateResultWarn, ReportRef: ref,
		ScannedAt: time.Now().UTC().Truncate(time.Microsecond),
	}))

	got, err := repo.GetByID(ctx, "scan_ref_1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, ref, got.ReportRef)
	assert.Equal(t, &counts, got.Counts)

	list, err := repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, ref, list[0].ReportRef)

	missing, err := repo.GetByID(ctx, "scan_missing")
	require.NoError(t, err)
	assert.Nil(t, missing)
}
