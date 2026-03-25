package repository

import (
	"context"
	"sort"
	"sync"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type MemoryResourceDefaultRepository struct {
	mu    sync.RWMutex
	items map[string]*domain.ResourceDefault
}

func NewMemoryResourceDefaultRepository() *MemoryResourceDefaultRepository {
	return &MemoryResourceDefaultRepository{items: map[string]*domain.ResourceDefault{}}
}

func (r *MemoryResourceDefaultRepository) List(_ context.Context) ([]*domain.ResourceDefault, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.items))
	for k := range r.items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]*domain.ResourceDefault, 0, len(keys))
	for _, k := range keys {
		copy := *r.items[k]
		out = append(out, &copy)
	}

	return out, nil
}

func (r *MemoryResourceDefaultRepository) Upsert(_ context.Context, resource *domain.ResourceDefault) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	copy := *resource
	r.items[resource.ToolKey] = &copy
	return nil
}
