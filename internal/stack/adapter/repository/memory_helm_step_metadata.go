package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// MemoryHelmStepMetadataRepository is an in-memory implementation of the Helm step metadata repository.
type MemoryHelmStepMetadataRepository struct {
	mu    sync.RWMutex
	items map[string]*domain.HelmStepMetadata
}

// NewMemoryHelmStepMetadataRepository constructs an empty in-memory repository.
func NewMemoryHelmStepMetadataRepository() *MemoryHelmStepMetadataRepository {
	return &MemoryHelmStepMetadataRepository{items: make(map[string]*domain.HelmStepMetadata)}
}

func (r *MemoryHelmStepMetadataRepository) Create(_ context.Context, item *domain.HelmStepMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[item.StepName]; ok {
		return fmt.Errorf("helm step metadata %q already exists", item.StepName)
	}
	cp := *item
	r.items[item.StepName] = &cp
	return nil
}

func (r *MemoryHelmStepMetadataRepository) Update(_ context.Context, item *domain.HelmStepMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[item.StepName]; !ok {
		return fmt.Errorf("helm step metadata %q not found", item.StepName)
	}
	cp := *item
	r.items[item.StepName] = &cp
	return nil
}

func (r *MemoryHelmStepMetadataRepository) Delete(_ context.Context, stepName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[stepName]; !ok {
		return fmt.Errorf("helm step metadata %q not found", stepName)
	}
	delete(r.items, stepName)
	return nil
}

func (r *MemoryHelmStepMetadataRepository) GetByStep(_ context.Context, stepName string) (*domain.HelmStepMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[stepName]
	if !ok {
		return nil, fmt.Errorf("helm step metadata %q not found", stepName)
	}
	cp := *item
	return &cp, nil
}

func (r *MemoryHelmStepMetadataRepository) List(_ context.Context) ([]*domain.HelmStepMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]*domain.HelmStepMetadata, 0, len(r.items))
	for _, item := range r.items {
		cp := *item
		items = append(items, &cp)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].SortOrder == items[j].SortOrder {
			return items[i].StepName < items[j].StepName
		}
		return items[i].SortOrder < items[j].SortOrder
	})
	return items, nil
}

func (r *MemoryHelmStepMetadataRepository) Seed(items ...*domain.HelmStepMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range items {
		if item == nil {
			continue
		}
		cp := *item
		if cp.CreatedAt.IsZero() {
			cp.CreatedAt = time.Now()
		}
		if cp.UpdatedAt.IsZero() {
			cp.UpdatedAt = cp.CreatedAt
		}
		r.items[cp.StepName] = &cp
	}
}
