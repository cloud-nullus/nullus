package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 같은 실행을 여러 번 동기화해도 기록이 늘지 않고, 최신 분석부터 돌려준다.
func TestMemorySASTResultRepository_UpsertAndList(t *testing.T) {
	repo := NewMemorySASTResultRepository()
	ctx := context.Background()
	base := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)

	require.NoError(t, repo.Upsert(ctx, &domain.SASTResult{ID: "sast_old", PipelineID: "pip_1", GateResult: domain.GateResultPass, AnalyzedAt: base}))
	require.NoError(t, repo.Upsert(ctx, &domain.SASTResult{ID: "sast_new", PipelineID: "pip_1", GateResult: domain.GateResultPass, AnalyzedAt: base.Add(time.Hour)}))
	require.NoError(t, repo.Upsert(ctx, &domain.SASTResult{ID: "sast_new", PipelineID: "pip_1", GateResult: domain.GateResultBlock, AnalyzedAt: base.Add(time.Hour)}))
	require.NoError(t, repo.Upsert(ctx, &domain.SASTResult{ID: "sast_other", PipelineID: "pip_2", AnalyzedAt: base}))

	got, err := repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "sast_new", got[0].ID)
	assert.Equal(t, domain.GateResultBlock, got[0].GateResult)
	assert.Equal(t, "sast_old", got[1].ID)
}

// 돌려준 값을 고쳐도 저장된 기록이 바뀌지 않는다 — DB 저장소에서는 일어나지 않는 이유로
// 테스트가 초록이 되면 안 된다.
func TestMemorySASTResultRepository_ReturnsCopies(t *testing.T) {
	repo := NewMemorySASTResultRepository()
	ctx := context.Background()
	bugs := 1
	require.NoError(t, repo.Upsert(ctx, &domain.SASTResult{
		ID: "sast_1", PipelineID: "pip_1",
		Conditions: []domain.SASTCondition{{Metric: "new_violations", Status: "ERROR"}},
		Metrics:    &domain.SASTMetrics{Bugs: &bugs},
	}))

	got, err := repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	*got[0].Metrics.Bugs = 99
	got[0].Conditions[0].Status = "OK"

	again, err := repo.ListByPipelineID(ctx, "pip_1")
	require.NoError(t, err)
	assert.Equal(t, 1, *again[0].Metrics.Bugs)
	assert.Equal(t, "ERROR", again[0].Conditions[0].Status)
}
