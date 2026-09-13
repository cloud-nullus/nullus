package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func TestMemoryImageScanResultRepository_Delete(t *testing.T) {
	repo := NewMemoryImageScanResultRepository()
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, &domain.ImageScanResult{ID: "scan_1", PipelineID: "pip_1", ScannedAt: time.Now()}))
	require.NoError(t, repo.Upsert(ctx, &domain.ImageScanResult{ID: "scan_2", PipelineID: "pip_1", ScannedAt: time.Now()}))

	require.NoError(t, repo.Delete(ctx, "scan_1"))
	require.NoError(t, repo.Delete(ctx, "scan_missing"), "없는 기록을 지우는 것은 오류가 아니다 — 동기화가 반복해서 부른다")

	got, err := repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "scan_2", got[0].ID)
}
