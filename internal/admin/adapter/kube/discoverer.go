package kube

import (
	"context"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

// Discoverer adapts the package-level DiscoverCluster function to the
// port.ClusterDiscoverer interface. It lets the use case depend on an
// interface instead of importing this adapter package directly, keeping the
// Clean Architecture dependency direction (domain -> port <- adapter).
type Discoverer struct{}

// NewDiscoverer constructs a Discoverer.
func NewDiscoverer() *Discoverer { return &Discoverer{} }

// Discover delegates to DiscoverCluster.
func (d *Discoverer) Discover(ctx context.Context, kubeconfig []byte) (*domain.ClusterDiscoveryInfo, error) {
	return DiscoverCluster(ctx, kubeconfig)
}
