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
