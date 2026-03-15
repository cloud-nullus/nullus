package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

// MemoryOrgRepository is an in-memory implementation of port.OrgRepository.
// Intended for development and testing only.
type MemoryOrgRepository struct {
	mu   sync.RWMutex
	orgs map[string]*domain.Organization
}

// NewMemoryOrgRepository constructs an empty MemoryOrgRepository.
func NewMemoryOrgRepository() *MemoryOrgRepository {
	return &MemoryOrgRepository{
		orgs: make(map[string]*domain.Organization),
	}
}

// Create stores a new organization.
func (r *MemoryOrgRepository) Create(_ context.Context, org *domain.Organization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orgs[org.ID]; ok {
		return fmt.Errorf("organization %q already exists", org.ID)
	}
	cp := *org
	r.orgs[org.ID] = &cp
	return nil
}

// GetByID retrieves an organization by ID. Returns nil, nil when not found.
func (r *MemoryOrgRepository) GetByID(_ context.Context, id string) (*domain.Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	org, ok := r.orgs[id]
	if !ok {
		return nil, nil
	}
	cp := *org
	return &cp, nil
}

func (r *MemoryOrgRepository) List(_ context.Context, limit, offset int) ([]*domain.Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orgs := make([]*domain.Organization, 0, len(r.orgs))
	for _, org := range r.orgs {
		cp := *org
		orgs = append(orgs, &cp)
	}

	sort.Slice(orgs, func(i, j int) bool {
		if orgs[i].CreatedAt.Equal(orgs[j].CreatedAt) {
			return orgs[i].ID < orgs[j].ID
		}
		return orgs[i].CreatedAt.Before(orgs[j].CreatedAt)
	})

	if offset < 0 {
		offset = 0
	}
	if offset >= len(orgs) {
		return []*domain.Organization{}, nil
	}

	orgs = orgs[offset:]
	if limit > 0 && limit < len(orgs) {
		orgs = orgs[:limit]
	}

	return orgs, nil
}

// Update replaces a stored organization.
func (r *MemoryOrgRepository) Update(_ context.Context, org *domain.Organization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orgs[org.ID]; !ok {
		return fmt.Errorf("organization %q not found", org.ID)
	}
	cp := *org
	r.orgs[org.ID] = &cp
	return nil
}

// GetBySlug retrieves an organization by slug. Returns nil, nil when not found.
func (r *MemoryOrgRepository) GetBySlug(_ context.Context, slug string) (*domain.Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, org := range r.orgs {
		if org.Slug == slug {
			cp := *org
			return &cp, nil
		}
	}
	return nil, nil
}
