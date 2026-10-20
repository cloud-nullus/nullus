package repository

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// MemoryDependencyScanResultRepository 는 port.DependencyScanResultRepository 의 메모리 구현이다.
type MemoryDependencyScanResultRepository struct {
	mu   sync.RWMutex
	rows map[string]*domain.DependencyScanResult
}

// NewMemoryDependencyScanResultRepository 는 빈 저장소를 만든다.
func NewMemoryDependencyScanResultRepository() *MemoryDependencyScanResultRepository {
	return &MemoryDependencyScanResultRepository{rows: make(map[string]*domain.DependencyScanResult)}
}

// Upsert 는 같은 ID 면 덮어쓴다.
func (r *MemoryDependencyScanResultRepository) Upsert(_ context.Context, result *domain.DependencyScanResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[result.ID] = cloneDependencyScanResult(result)
	return nil
}

// Delete 는 기록을 지운다. 없으면 아무 일도 하지 않는다.
func (r *MemoryDependencyScanResultRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, strings.TrimSpace(id))
	return nil
}

// ListByPipelineID 는 최신 결과부터 돌려준다.
func (r *MemoryDependencyScanResultRepository) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.DependencyScanResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pipelineID = strings.TrimSpace(pipelineID)
	out := make([]*domain.DependencyScanResult, 0)
	for _, row := range r.rows {
		if row.PipelineID == pipelineID {
			out = append(out, cloneDependencyScanResult(row))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].ScannedAt.Equal(out[j].ScannedAt) {
			return out[i].ScannedAt.After(out[j].ScannedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// cloneDependencyScanResult 는 포인터 필드까지 복제한다. 공유하면 호출부가 고친 값이 저장된 기록까지 바꾼다.
func cloneDependencyScanResult(in *domain.DependencyScanResult) *domain.DependencyScanResult {
	if in == nil {
		return nil
	}
	cp := *in
	cp.Counts = clonePtr(in.Counts)
	cp.Targets = clonePtr(in.Targets)
	cp.DBUpdatedAt = clonePtr(in.DBUpdatedAt)
	cp.ReportRef = clonePtr(in.ReportRef)
	return &cp
}
