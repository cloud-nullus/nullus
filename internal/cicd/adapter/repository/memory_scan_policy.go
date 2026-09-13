package repository

import (
	"context"
	"strings"
	"sync"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// MemoryScanPolicyRepository 는 port.ScanPolicyRepository 의 메모리 구현이다.
type MemoryScanPolicyRepository struct {
	mu   sync.RWMutex
	rows map[string]domain.ScanPolicy
}

// NewMemoryScanPolicyRepository 는 빈 저장소를 만든다.
func NewMemoryScanPolicyRepository() *MemoryScanPolicyRepository {
	return &MemoryScanPolicyRepository{rows: make(map[string]domain.ScanPolicy)}
}

// Get 은 저장된 정책이다. 저장한 적 없으면 found=false 다.
func (r *MemoryScanPolicyRepository) Get(_ context.Context, stackID string) (domain.ScanPolicy, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.rows[strings.TrimSpace(stackID)]
	return p, ok, nil
}

// Upsert 는 스택의 정책을 저장하거나 덮어쓴다.
func (r *MemoryScanPolicyRepository) Upsert(_ context.Context, stackID string, policy domain.ScanPolicy, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[strings.TrimSpace(stackID)] = policy
	return nil
}
