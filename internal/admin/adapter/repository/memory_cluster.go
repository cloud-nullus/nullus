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
	kubeconf map[string][]byte
}

// NewMemoryClusterRepository constructs an empty MemoryClusterRepository.
func NewMemoryClusterRepository() *MemoryClusterRepository {
	return &MemoryClusterRepository{
		clusters: make(map[string]*domain.Cluster),
		kubeconf: make(map[string][]byte),
	}
}

// cloneCluster returns a deep copy of a Cluster, including the slice fields.
// Without this the shared backing array would leak between the store and
// callers (observed when a discovery Update() mutated NodeArchitectures in
// place and the mutation appeared in a prior GetByID result).
func cloneCluster(c *domain.Cluster) *domain.Cluster {
	cp := *c
	if len(c.Types) > 0 {
		cp.Types = append([]domain.ClusterType(nil), c.Types...)
	} else {
		cp.Types = nil
	}
	if len(c.NodeArchitectures) > 0 {
		cp.NodeArchitectures = append([]string(nil), c.NodeArchitectures...)
	} else {
		cp.NodeArchitectures = nil
	}
	return &cp
}

// Create stores a new cluster.
func (r *MemoryClusterRepository) Create(_ context.Context, cluster *domain.Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clusters[cluster.ID]; ok {
		return fmt.Errorf("cluster %q already exists", cluster.ID)
	}
	stored := cloneCluster(cluster)
	stored.NodeArchitectures = domain.NormalizeNodeArchitectures(stored.NodeArchitectures)
	r.clusters[cluster.ID] = stored
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
	return cloneCluster(c), nil
}

// List returns all clusters for the given orgID. Passing an empty orgID returns all clusters.
func (r *MemoryClusterRepository) List(_ context.Context, orgID string) ([]*domain.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Cluster, 0, len(r.clusters))
	for _, c := range r.clusters {
		if orgID == "" || c.OrgID == orgID {
			result = append(result, cloneCluster(c))
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
	stored := cloneCluster(cluster)
	stored.NodeArchitectures = domain.NormalizeNodeArchitectures(stored.NodeArchitectures)
	r.clusters[cluster.ID] = stored
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
	delete(r.kubeconf, id)
	return nil
}

func (r *MemoryClusterRepository) SaveKubeconfig(_ context.Context, id string, kubeconfig []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clusters[id]; !ok {
		return fmt.Errorf("cluster %q not found", id)
	}
	cp := make([]byte, len(kubeconfig))
	copy(cp, kubeconfig)
	r.kubeconf[id] = cp
	return nil
}

func (r *MemoryClusterRepository) GetKubeconfig(_ context.Context, id string) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.kubeconf[id]
	if !ok {
		return nil, nil
	}
	cp := make([]byte, len(cfg))
	copy(cp, cfg)
	return cp, nil
}
