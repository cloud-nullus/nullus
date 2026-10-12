package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func TestMemoryImageScanResultRepository_GetByID(t *testing.T) {
	repo := NewMemoryImageScanResultRepository()
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, &domain.ImageScanResult{
		ID: "scan_1", PipelineID: "pip_1", ScannedAt: time.Now(),
		ReportRef: &domain.ScanReportRef{JobName: "app", BuildNumber: 7, Path: "trivy-report.json"},
	}))

	got, err := repo.GetByID(ctx, "scan_1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 7, got.ReportRef.BuildNumber)

	// 복제본을 돌려준다 — 호출부가 고쳐도 저장된 기록은 그대로다.
	got.ReportRef.BuildNumber = 99
	again, _ := repo.GetByID(ctx, "scan_1")
	assert.Equal(t, 7, again.ReportRef.BuildNumber)

	missing, err := repo.GetByID(ctx, "nope")
	require.NoError(t, err)
	assert.Nil(t, missing)
}
