package repository

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// MemoryImageScanResultRepository 는 port.ImageScanResultRepository 의 메모리 구현이다.
type MemoryImageScanResultRepository struct {
	mu   sync.RWMutex
	rows map[string]*domain.ImageScanResult
}

// NewMemoryImageScanResultRepository 는 빈 저장소를 만든다.
func NewMemoryImageScanResultRepository() *MemoryImageScanResultRepository {
	return &MemoryImageScanResultRepository{rows: make(map[string]*domain.ImageScanResult)}
}

// Upsert 는 같은 ID 면 덮어쓴다.
func (r *MemoryImageScanResultRepository) Upsert(_ context.Context, result *domain.ImageScanResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[result.ID] = cloneImageScanResult(result)
	return nil
}

// ListByPipelineID 는 최신 결과부터 돌려준다.
func (r *MemoryImageScanResultRepository) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.ImageScanResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pipelineID = strings.TrimSpace(pipelineID)
	out := make([]*domain.ImageScanResult, 0)
	for _, row := range r.rows {
		if row.PipelineID == pipelineID {
			out = append(out, cloneImageScanResult(row))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ScannedAt.After(out[j].ScannedAt) })
	return out, nil
}

// cloneImageScanResult 는 포인터 필드까지 복제한다. 공유하면 호출부가 고친 값이
// 저장된 기록까지 바꾸고, DB 저장소에서는 일어나지 않는 이유로 테스트가 초록이 된다.
func cloneImageScanResult(in *domain.ImageScanResult) *domain.ImageScanResult {
	if in == nil {
		return nil
	}
	cp := *in
	if in.Counts != nil {
		c := *in.Counts
		cp.Counts = &c
	}
	if in.DBUpdatedAt != nil {
		t := *in.DBUpdatedAt
		cp.DBUpdatedAt = &t
	}
	return &cp
}
