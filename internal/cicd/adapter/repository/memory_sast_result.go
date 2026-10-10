package repository

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// MemorySASTResultRepository 는 port.SASTResultRepository 의 메모리 구현이다.
type MemorySASTResultRepository struct {
	mu   sync.RWMutex
	rows map[string]*domain.SASTResult
}

// NewMemorySASTResultRepository 는 빈 저장소를 만든다.
func NewMemorySASTResultRepository() *MemorySASTResultRepository {
	return &MemorySASTResultRepository{rows: make(map[string]*domain.SASTResult)}
}

// Upsert 는 같은 ID 면 덮어쓴다.
func (r *MemorySASTResultRepository) Upsert(_ context.Context, result *domain.SASTResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[result.ID] = cloneSASTResult(result)
	return nil
}

// ListByPipelineID 는 최신 분석부터 돌려준다.
func (r *MemorySASTResultRepository) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.SASTResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pipelineID = strings.TrimSpace(pipelineID)
	out := make([]*domain.SASTResult, 0)
	for _, row := range r.rows {
		if row.PipelineID == pipelineID {
			out = append(out, cloneSASTResult(row))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].AnalyzedAt.Equal(out[j].AnalyzedAt) {
			return out[i].AnalyzedAt.After(out[j].AnalyzedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// cloneSASTResult 는 포인터·슬라이스 필드까지 복제한다. 공유하면 호출부가 고친 값이
// 저장된 기록까지 바꾼다.
func cloneSASTResult(in *domain.SASTResult) *domain.SASTResult {
	if in == nil {
		return nil
	}
	cp := *in
	if in.Conditions != nil {
		cp.Conditions = append([]domain.SASTCondition(nil), in.Conditions...)
	}
	if in.Metrics != nil {
		m := domain.SASTMetrics{
			Bugs:                   clonePtr(in.Metrics.Bugs),
			Vulnerabilities:        clonePtr(in.Metrics.Vulnerabilities),
			CodeSmells:             clonePtr(in.Metrics.CodeSmells),
			SecurityHotspots:       clonePtr(in.Metrics.SecurityHotspots),
			Coverage:               clonePtr(in.Metrics.Coverage),
			DuplicatedLinesDensity: clonePtr(in.Metrics.DuplicatedLinesDensity),
			Ncloc:                  clonePtr(in.Metrics.Ncloc),
		}
		cp.Metrics = &m
	}
	return &cp
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
