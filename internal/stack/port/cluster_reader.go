package port

import "context"

// ClusterSummary is the minimum subset of Cluster context the Stack module
// needs from the Admin module. Kept separate from admin/domain.Cluster so
// the Stack bounded context never imports admin internals directly.
type ClusterSummary struct {
	ID                string   `json:"id"`
	OrgID             string   `json:"org_id"`
	NodeArchitectures []string `json:"node_architectures"`
}

// ClusterReader is a read-only port the Stack context uses to pull cluster
// facts (e.g. node architectures) that the Pre-Deploy Gate needs when
// cross-checking ToolVersion.ArchSupport. Implemented by an adapter that
// either queries the admin database directly (modular monolith) or calls
// the admin service (microservice split).
type ClusterReader interface {
	GetClusterSummary(ctx context.Context, clusterID string) (*ClusterSummary, error)
}
