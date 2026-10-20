package repository

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// MemorySecretScanResultRepository 는 port.SecretScanResultRepository 의 메모리 구현이다.
type MemorySecretScanResultRepository struct {
	mu   sync.RWMutex
	rows map[string]*domain.SecretScanResult
}

// NewMemorySecretScanResultRepository 는 빈 저장소를 만든다.
func NewMemorySecretScanResultRepository() *MemorySecretScanResultRepository {
	return &MemorySecretScanResultRepository{rows: make(map[string]*domain.SecretScanResult)}
}

// Upsert 는 같은 ID 면 덮어쓴다.
func (r *MemorySecretScanResultRepository) Upsert(_ context.Context, result *domain.SecretScanResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[result.ID] = cloneSecretScanResult(result)
	return nil
}

// Delete 는 기록을 지운다. 없으면 아무 일도 하지 않는다.
func (r *MemorySecretScanResultRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, strings.TrimSpace(id))
	return nil
}

// ListByPipelineID 는 최신 결과부터 돌려준다.
func (r *MemorySecretScanResultRepository) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.SecretScanResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pipelineID = strings.TrimSpace(pipelineID)
	out := make([]*domain.SecretScanResult, 0)
	for _, row := range r.rows {
		if row.PipelineID == pipelineID {
			out = append(out, cloneSecretScanResult(row))
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

// cloneSecretScanResult 는 포인터·슬라이스 필드까지 복제한다. 탐지 위치는 DB 와 같은 상한으로 자른다.
func cloneSecretScanResult(in *domain.SecretScanResult) *domain.SecretScanResult {
	if in == nil {
		return nil
	}
	cp := *in
	cp.FindingsCount = clonePtr(in.FindingsCount)
	cp.ReportRef = clonePtr(in.ReportRef)
	cp.Rules = append([]domain.SecretRuleCount{}, in.Rules...)
	findings := in.Findings
	if len(findings) > domain.MaxStoredSecretFindings {
		findings = findings[:domain.MaxStoredSecretFindings]
	}
	cp.Findings = append([]domain.SecretFinding{}, findings...)
	return &cp
}
