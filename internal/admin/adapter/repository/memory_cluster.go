package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

// MemoryClusterRepository is an in-memory implementation of port.ClusterRepository.
// Intended for development and testing only.
type MemoryClusterRepository struct {
	mu       sync.RWMutex
	clusters map[string]*domain.Cluster
}

// NewMemoryClusterRepository constructs an empty MemoryClusterRepository.
func NewMemoryClusterRepository() *MemoryClusterRepository {
	return &MemoryClusterRepository{
		clusters: make(map[string]*domain.Cluster),
	}
}

// Create stores a new cluster.
func (r *MemoryClusterRepository) Create(_ context.Context, cluster *domain.Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clusters[cluster.ID]; ok {
		return fmt.Errorf("cluster %q already exists", cluster.ID)
	}
	cp := *cluster
	r.clusters[cluster.ID] = &cp
	return nil
}

// GetByID retrieves a cluster by ID. Returns nil, nil when not found.
func (r *MemoryClusterRepository) GetByID(_ context.Context, id string) (*domain.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.clusters[id]
	if !ok {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}

// List returns all clusters for the given orgID. Passing an empty orgID returns all clusters.
func (r *MemoryClusterRepository) List(_ context.Context, orgID string) ([]*domain.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Cluster, 0, len(r.clusters))
	for _, c := range r.clusters {
		if orgID == "" || c.OrgID == orgID {
			cp := *c
			result = append(result, &cp)
		}
	}
	return result, nil
}

// Update replaces a stored cluster.
func (r *MemoryClusterRepository) Update(_ context.Context, cluster *domain.Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clusters[cluster.ID]; !ok {
		return fmt.Errorf("cluster %q not found", cluster.ID)
	}
	cp := *cluster
	r.clusters[cluster.ID] = &cp
	return nil
}

// Delete removes a cluster by ID.
func (r *MemoryClusterRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clusters[id]; !ok {
		return fmt.Errorf("cluster %q not found", id)
	}
	delete(r.clusters, id)
	return nil
}
