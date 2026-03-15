package usecase

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/google/uuid"
)

// ClusterUseCase handles Cluster business logic.
type ClusterUseCase struct {
	clusterRepo port.ClusterRepository
}

// NewClusterUseCase creates a new ClusterUseCase.
func NewClusterUseCase(clusterRepo port.ClusterRepository) *ClusterUseCase {
	return &ClusterUseCase{clusterRepo: clusterRepo}
}

// RegisterClusterInput holds the input for registering a cluster.
type RegisterClusterInput struct {
	Name     string
	Type     domain.ClusterType
	Endpoint string
	OrgID    string
}

// UpdateClusterInput holds the input for updating a cluster.
type UpdateClusterInput struct {
	Name     string
	Endpoint string
}

// RegisterCluster registers a new cluster with pending connection status.
func (uc *ClusterUseCase) RegisterCluster(ctx context.Context, input RegisterClusterInput) (*domain.Cluster, error) {
	now := time.Now().UTC()
	cluster := &domain.Cluster{
		ID:               uuid.New().String(),
		Name:             input.Name,
		Type:             input.Type,
		Endpoint:         input.Endpoint,
		ConnectionStatus: domain.ConnectionStatusPending,
		OrgID:            input.OrgID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := uc.clusterRepo.Create(ctx, cluster); err != nil {
		return nil, fmt.Errorf("registering cluster: %w", err)
	}

	return cluster, nil
}

// GetCluster retrieves a cluster by ID.
func (uc *ClusterUseCase) GetCluster(ctx context.Context, id string) (*domain.Cluster, error) {
	cluster, err := uc.clusterRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting cluster: %w", err)
	}
	if cluster == nil {
		return nil, &shareddomain.AppError{
			Code:       "CLUSTER_NOT_FOUND",
			HTTPStatus: http.StatusNotFound,
			Message:    "Cluster not found",
			Detail:     fmt.Sprintf("cluster with id %q does not exist", id),
			Retryable:  false,
		}
	}
	return cluster, nil
}

// ListClusters returns all clusters for the given organization.
func (uc *ClusterUseCase) ListClusters(ctx context.Context, orgID string) ([]*domain.Cluster, error) {
	clusters, err := uc.clusterRepo.List(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing clusters: %w", err)
	}
	return clusters, nil
}

// UpdateCluster updates a cluster's mutable fields.
func (uc *ClusterUseCase) UpdateCluster(ctx context.Context, id string, input UpdateClusterInput) (*domain.Cluster, error) {
	cluster, err := uc.GetCluster(ctx, id)
	if err != nil {
		return nil, err
	}

	cluster.Name = input.Name
	cluster.Endpoint = input.Endpoint
	cluster.UpdatedAt = time.Now().UTC()

	if err := uc.clusterRepo.Update(ctx, cluster); err != nil {
		return nil, fmt.Errorf("updating cluster: %w", err)
	}

	return cluster, nil
}

// DeleteCluster removes a cluster by ID.
func (uc *ClusterUseCase) DeleteCluster(ctx context.Context, id string) error {
	_, err := uc.GetCluster(ctx, id)
	if err != nil {
		return err
	}

	if err := uc.clusterRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting cluster: %w", err)
	}

	return nil
}

// VerifyCluster marks a cluster connection status as connected.
func (uc *ClusterUseCase) VerifyCluster(ctx context.Context, id string) (*domain.Cluster, error) {
	cluster, err := uc.GetCluster(ctx, id)
	if err != nil {
		return nil, err
	}

	cluster.ConnectionStatus = domain.ConnectionStatusConnected
	cluster.UpdatedAt = time.Now().UTC()

	if err := uc.clusterRepo.Update(ctx, cluster); err != nil {
		return nil, fmt.Errorf("verifying cluster: %w", err)
	}

	return cluster, nil
}

func (uc *ClusterUseCase) SaveKubeconfig(ctx context.Context, id string, kubeconfig []byte) error {
	if _, err := uc.GetCluster(ctx, id); err != nil {
		return err
	}
	if err := uc.clusterRepo.SaveKubeconfig(ctx, id, kubeconfig); err != nil {
		return fmt.Errorf("saving kubeconfig: %w", err)
	}
	return nil
}

func (uc *ClusterUseCase) GetKubeconfig(ctx context.Context, id string) ([]byte, error) {
	if _, err := uc.GetCluster(ctx, id); err != nil {
		return nil, err
	}
	kubeconfig, err := uc.clusterRepo.GetKubeconfig(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting kubeconfig: %w", err)
	}
	return kubeconfig, nil
}
