package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

// OrgRepository defines the interface for organization persistence.
type OrgRepository interface {
	Create(ctx context.Context, org *domain.Organization) error
	GetByID(ctx context.Context, id string) (*domain.Organization, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Organization, error)
	Update(ctx context.Context, org *domain.Organization) error
	GetBySlug(ctx context.Context, slug string) (*domain.Organization, error)
}

type ResourceProfileRepository interface {
	List(ctx context.Context, orgID string) ([]*domain.OrgResourceProfile, error)
	Create(ctx context.Context, profile *domain.OrgResourceProfile) error
	Update(ctx context.Context, profile *domain.OrgResourceProfile) (bool, error)
	Delete(ctx context.Context, orgID, id string) error
}

// ClusterRepository defines the interface for cluster persistence.
type ClusterRepository interface {
	Create(ctx context.Context, cluster *domain.Cluster) error
	GetByID(ctx context.Context, id string) (*domain.Cluster, error)
	List(ctx context.Context, orgID string) ([]*domain.Cluster, error)
	Update(ctx context.Context, cluster *domain.Cluster) error
	Delete(ctx context.Context, id string) error
	SaveKubeconfig(ctx context.Context, id string, kubeconfig []byte) error
	GetKubeconfig(ctx context.Context, id string) ([]byte, error)
}

// ClusterDiscoverer talks to a live Kubernetes cluster and returns facts we
// persist back onto the Cluster aggregate (server version, node arch set).
// Defined as a port so ClusterUseCase can be unit tested without a real
// cluster and the Pre-Deploy Gate can depend on the admin module only
// through well-defined interfaces.
type ClusterDiscoverer interface {
	Discover(ctx context.Context, kubeconfig []byte) (*domain.ClusterDiscoveryInfo, error)
}

// UserRepository defines the interface for user persistence.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	SearchByEmail(ctx context.Context, email string) (*domain.User, error)
	ListByOrg(ctx context.Context, orgID string) ([]*domain.User, error)
	AddMember(ctx context.Context, orgID, userID string, role domain.Role) error
	IsMember(ctx context.Context, orgID, userID string) (bool, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id string) error
}

// TokenSourceRepository defines persistence for token rotation metadata/events.
type TokenSourceRepository interface {
	ListSources(ctx context.Context, orgID string) ([]*domain.TokenSource, error)
	ListEvents(ctx context.Context, tokenSourceID string) ([]*domain.TokenRotationEvent, error)
	GetSource(ctx context.Context, tokenSourceID string) (*domain.TokenSource, error)
	UpdateSourceStatus(ctx context.Context, tokenSourceID, status string) error
	InsertEvent(ctx context.Context, event *domain.TokenRotationEvent) error
}
