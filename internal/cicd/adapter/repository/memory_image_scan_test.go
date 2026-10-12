package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 같은 실행을 여러 번 동기화해도 기록이 늘지 않아야 한다.
func TestMemoryImageScanResultRepository_UpsertIsIdempotent(t *testing.T) {
	repo := NewMemoryImageScanResultRepository()
	ctx := context.Background()
	r := &domain.ImageScanResult{ID: "scan_1", PipelineID: "pip_1", GateResult: domain.GateResultPass, ScannedAt: time.Now()}

	require.NoError(t, repo.Upsert(ctx, r))
	r2 := *r
	r2.GateResult = domain.GateResultBlock
	require.NoError(t, repo.Upsert(ctx, &r2))

	got, err := repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, domain.GateResultBlock, got[0].GateResult)
}

// 대시보드는 최신 결과를 먼저 본다.
func TestMemoryImageScanResultRepository_ListsNewestFirst(t *testing.T) {
	repo := NewMemoryImageScanResultRepository()
	ctx := context.Background()
	base := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Upsert(ctx, &domain.ImageScanResult{ID: "old", PipelineID: "pip_1", ScannedAt: base}))
	require.NoError(t, repo.Upsert(ctx, &domain.ImageScanResult{ID: "new", PipelineID: "pip_1", ScannedAt: base.Add(time.Hour)}))
	require.NoError(t, repo.Upsert(ctx, &domain.ImageScanResult{ID: "other", PipelineID: "pip_2", ScannedAt: base}))

	got, err := repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "new", got[0].ID)
	assert.Equal(t, "old", got[1].ID)
}

// 포인터 필드를 공유하면 호출부가 고친 값이 저장된 기록까지 바꾼다.
func TestMemoryImageScanResultRepository_DoesNotShareState(t *testing.T) {
	repo := NewMemoryImageScanResultRepository()
	ctx := context.Background()
	counts := &domain.SeverityCounts{Critical: 1}
	require.NoError(t, repo.Upsert(ctx, &domain.ImageScanResult{ID: "s", PipelineID: "p", Counts: counts, ScannedAt: time.Now()}))

	counts.Critical = 99
	got, err := repo.ListByPipelineID(ctx, "p")
	require.NoError(t, err)
	got[0].Counts.Critical = 42

	again, err := repo.ListByPipelineID(ctx, "p")
	require.NoError(t, err)
	assert.Equal(t, 1, again[0].Counts.Critical)
}
