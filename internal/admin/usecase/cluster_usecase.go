package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// ClusterUseCase handles Cluster business logic.
type ClusterUseCase struct {
	clusterRepo port.ClusterRepository
	orgRepo     port.OrgRepository
	discoverer  port.ClusterDiscoverer
	decryptFn   func([]byte) ([]byte, error)
}

func NewClusterUseCase(clusterRepo port.ClusterRepository, opts ...func(*ClusterUseCase)) *ClusterUseCase {
	uc := &ClusterUseCase{clusterRepo: clusterRepo}
	for _, o := range opts {
		o(uc)
	}
	return uc
}

func WithOrgRepo(r port.OrgRepository) func(*ClusterUseCase) {
	return func(uc *ClusterUseCase) { uc.orgRepo = r }
}

// WithDiscoverer injects a ClusterDiscoverer so Register/Update/Refresh
// paths can probe the real cluster for node architectures.
func WithDiscoverer(d port.ClusterDiscoverer) func(*ClusterUseCase) {
	return func(uc *ClusterUseCase) { uc.discoverer = d }
}

// WithKubeconfigDecryptor injects a function that turns the repository's
// encrypted kubeconfig blob into the raw YAML bytes the discoverer can
// consume. Kept as an option so the use case doesn't depend on the crypto
// package directly.
func WithKubeconfigDecryptor(fn func([]byte) ([]byte, error)) func(*ClusterUseCase) {
	return func(uc *ClusterUseCase) { uc.decryptFn = fn }
}

func (uc *ClusterUseCase) GetFirstOrgID(ctx context.Context) (string, error) {
	if uc.orgRepo == nil {
		return "", fmt.Errorf("org repository not configured")
	}
	orgs, err := uc.orgRepo.List(ctx, 1, 0)
	if err != nil {
		return "", err
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("no organizations found")
	}
	return orgs[0].ID, nil
}

// RegisterClusterInput holds the input for registering a cluster.
type RegisterClusterInput struct {
	Name          string
	Type          domain.ClusterType
	Types         []domain.ClusterType
	CloudProvider domain.CloudProvider
	Endpoint      string
	OrgID         string
}

// UpdateClusterInput holds the input for updating a cluster.
type UpdateClusterInput struct {
	Name          string
	Type          domain.ClusterType
	Types         []domain.ClusterType
	CloudProvider domain.CloudProvider
	Endpoint      string
}

// RegisterCluster registers a new cluster with pending connection status.
func (uc *ClusterUseCase) RegisterCluster(ctx context.Context, input RegisterClusterInput) (*domain.Cluster, error) {
	now := time.Now().UTC()
	clusterTypes := domain.NormalizeClusterTypes(input.Types, input.Type)
	cluster := &domain.Cluster{
		ID:               uuid.New().String(),
		Name:             input.Name,
		Type:             domain.ResolvePrimaryClusterType(clusterTypes, input.Type),
		Types:            clusterTypes,
		CloudProvider:    input.CloudProvider,
		Endpoint:         input.Endpoint,
		ConnectionStatus: domain.ConnectionStatusPending,
		OrgID:            input.OrgID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if cluster.CloudProvider == "" {
		cluster.CloudProvider = domain.CloudProviderOnPremise
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
	cluster.Types = domain.NormalizeClusterTypes(cluster.Types, cluster.Type)
	cluster.Type = domain.ResolvePrimaryClusterType(cluster.Types, cluster.Type)
	if cluster.CloudProvider == "" {
		cluster.CloudProvider = domain.CloudProviderOnPremise
	}
	return cluster, nil
}

// ListClusters returns all clusters for the given organization.
func (uc *ClusterUseCase) ListClusters(ctx context.Context, orgID string) ([]*domain.Cluster, error) {
	clusters, err := uc.clusterRepo.List(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing clusters: %w", err)
	}
	for _, cluster := range clusters {
		cluster.Types = domain.NormalizeClusterTypes(cluster.Types, cluster.Type)
		cluster.Type = domain.ResolvePrimaryClusterType(cluster.Types, cluster.Type)
		if cluster.CloudProvider == "" {
			cluster.CloudProvider = domain.CloudProviderOnPremise
		}
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
	if input.Type != "" || len(input.Types) > 0 {
		cluster.Types = domain.NormalizeClusterTypes(input.Types, input.Type)
		cluster.Type = domain.ResolvePrimaryClusterType(cluster.Types, input.Type)
	}
	if input.CloudProvider != "" {
		cluster.CloudProvider = input.CloudProvider
	}
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return &shareddomain.AppError{
				Code:       "CLUSTER_IN_USE",
				HTTPStatus: http.StatusConflict,
				Message:    "Cluster is in use",
				Detail:     "Delete stacks and pipelines linked to this cluster first",
				Retryable:  false,
			}
		}
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

// RefreshDiscovery reads the stored kubeconfig, probes the cluster for its
// server version and node architectures, and persists the result onto the
// Cluster aggregate.
//
// On discovery failure the cluster is marked connection_failed with an empty
// NodeArchitectures slice — the Pre-Deploy Gate interprets empty as "arch
// unknown" and falls back to a warn verdict, prompting the user to refresh.
func (uc *ClusterUseCase) RefreshDiscovery(ctx context.Context, id string) (*domain.Cluster, error) {
	if uc.discoverer == nil {
		return nil, fmt.Errorf("cluster discoverer not configured")
	}
	cluster, err := uc.GetCluster(ctx, id)
	if err != nil {
		return nil, err
	}
	storedKubeconfig, err := uc.clusterRepo.GetKubeconfig(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("reading kubeconfig: %w", err)
	}
	if len(storedKubeconfig) == 0 {
		return nil, &shareddomain.AppError{
			Code:       "KUBECONFIG_NOT_REGISTERED",
			HTTPStatus: http.StatusBadRequest,
			Message:    "Kubeconfig is not registered for this cluster",
			Retryable:  false,
		}
	}

	rawKubeconfig := storedKubeconfig
	if uc.decryptFn != nil {
		rawKubeconfig, err = uc.decryptFn(storedKubeconfig)
		if err != nil {
			return uc.markDiscoveryFailed(ctx, cluster, fmt.Errorf("decrypt kubeconfig: %w", err))
		}
	}

	info, err := uc.discoverer.Discover(ctx, rawKubeconfig)
	if err != nil {
		return uc.markDiscoveryFailed(ctx, cluster, err)
	}

	cluster.ConnectionStatus = domain.ConnectionStatusConnected
	cluster.NodeArchitectures = domain.NormalizeNodeArchitectures(info.NodeArchitectures)
	cluster.UpdatedAt = time.Now().UTC()
	if err := uc.clusterRepo.Update(ctx, cluster); err != nil {
		return nil, fmt.Errorf("persisting discovery: %w", err)
	}
	return cluster, nil
}

// markDiscoveryFailed records a connection_failed status with an empty node
// arch set so the Pre-Deploy Gate can distinguish a stale/unknown cluster
// from a healthy one. Persistence errors during the status write are
// surfaced alongside the original discovery error.
func (uc *ClusterUseCase) markDiscoveryFailed(ctx context.Context, cluster *domain.Cluster, discoveryErr error) (*domain.Cluster, error) {
	cluster.ConnectionStatus = domain.ConnectionStatusConnectionFailed
	cluster.NodeArchitectures = nil
	cluster.UpdatedAt = time.Now().UTC()
	if err := uc.clusterRepo.Update(ctx, cluster); err != nil {
		return cluster, fmt.Errorf("discovery failed: %w (also failed to persist status: %v)", discoveryErr, err)
	}
	return cluster, discoveryErr
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
