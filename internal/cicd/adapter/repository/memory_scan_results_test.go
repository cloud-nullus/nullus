package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 메모리 저장소는 DB 와 같게 동작해야 한다 — 최신부터, 포인터를 공유하지 않고, 지운 것은 사라진다.
func TestMemoryDependencyScanResultRepository(t *testing.T) {
	repo := NewMemoryDependencyScanResultRepository()
	ctx := context.Background()
	at := time.Now()
	two := 2
	in := &domain.DependencyScanResult{ID: "b", PipelineID: "pip_1", Counts: &domain.SeverityCounts{High: 1}, Targets: &two, ScannedAt: at}
	require.NoError(t, repo.Upsert(ctx, in))
	require.NoError(t, repo.Upsert(ctx, &domain.DependencyScanResult{ID: "a", PipelineID: "pip_1", ScannedAt: at}))
	require.NoError(t, repo.Upsert(ctx, &domain.DependencyScanResult{ID: "c", PipelineID: "pip_1", ScannedAt: at.Add(time.Minute)}))
	require.NoError(t, repo.Upsert(ctx, &domain.DependencyScanResult{ID: "x", PipelineID: "pip_2", ScannedAt: at}))

	in.Counts.High = 99
	list, err := repo.ListByPipelineID(ctx, " pip_1 ")
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, []string{"c", "a", "b"}, []string{list[0].ID, list[1].ID, list[2].ID})
	assert.Equal(t, 1, list[2].Counts.High, "저장된 기록이 호출부와 포인터를 공유하지 않는다")

	require.NoError(t, repo.Delete(ctx, "c"))
	list, err = repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestMemorySecretScanResultRepository(t *testing.T) {
	repo := NewMemorySecretScanResultRepository()
	ctx := context.Background()
	at := time.Now()
	many := make([]domain.SecretFinding, domain.MaxStoredSecretFindings+3)
	n := len(many)
	require.NoError(t, repo.Upsert(ctx, &domain.SecretScanResult{ID: "b", PipelineID: "pip_1", FindingsCount: &n, Findings: many, ScannedAt: at}))
	require.NoError(t, repo.Upsert(ctx, &domain.SecretScanResult{ID: "a", PipelineID: "pip_1", ScannedAt: at}))
	require.NoError(t, repo.Upsert(ctx, &domain.SecretScanResult{ID: "c", PipelineID: "pip_1", ScannedAt: at.Add(time.Minute)}))

	list, err := repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, []string{"c", "a", "b"}, []string{list[0].ID, list[1].ID, list[2].ID})
	assert.Len(t, list[2].Findings, domain.MaxStoredSecretFindings, "DB 와 같은 상한으로 자른다")
	assert.Equal(t, n, *list[2].FindingsCount)
	assert.NotNil(t, list[1].Rules, "빈 목록은 null 이 아니다")
	assert.NotNil(t, list[1].Findings)

	require.NoError(t, repo.Delete(ctx, "c"))
	list, err = repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
