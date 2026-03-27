package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClusterRepo is an in-memory mock of port.ClusterRepository.
type mockClusterRepo struct {
	clusters  map[string]*domain.Cluster
	kubeconf  map[string][]byte
	deleteErr error
}

func newMockClusterRepo() *mockClusterRepo {
	return &mockClusterRepo{
		clusters: make(map[string]*domain.Cluster),
		kubeconf: make(map[string][]byte),
	}
}

func (m *mockClusterRepo) Create(_ context.Context, cluster *domain.Cluster) error {
	m.clusters[cluster.ID] = cluster
	return nil
}

func (m *mockClusterRepo) GetByID(_ context.Context, id string) (*domain.Cluster, error) {
	return m.clusters[id], nil
}

func (m *mockClusterRepo) List(_ context.Context, orgID string) ([]*domain.Cluster, error) {
	var result []*domain.Cluster
	for _, c := range m.clusters {
		if c.OrgID == orgID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockClusterRepo) Update(_ context.Context, cluster *domain.Cluster) error {
	m.clusters[cluster.ID] = cluster
	return nil
}

func (m *mockClusterRepo) Delete(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.clusters, id)
	return nil
}

func (m *mockClusterRepo) SaveKubeconfig(_ context.Context, id string, kubeconfig []byte) error {
	cp := make([]byte, len(kubeconfig))
	copy(cp, kubeconfig)
	m.kubeconf[id] = cp
	return nil
}

func (m *mockClusterRepo) GetKubeconfig(_ context.Context, id string) ([]byte, error) {
	if cfg, ok := m.kubeconf[id]; ok {
		cp := make([]byte, len(cfg))
		copy(cp, cfg)
		return cp, nil
	}
	return nil, nil
}

func TestClusterUseCase_RegisterCluster_Success(t *testing.T) {
	repo := newMockClusterRepo()
	uc := NewClusterUseCase(repo)

	cluster, err := uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name:     "production-gke",
		Type:     domain.ClusterTypePipeline,
		Endpoint: "https://35.1.2.3:6443",
		OrgID:    "org_001",
	})

	require.NoError(t, err)
	assert.Equal(t, "production-gke", cluster.Name)
	assert.Equal(t, domain.ConnectionStatusPending, cluster.ConnectionStatus)
	assert.Equal(t, "org_001", cluster.OrgID)
	assert.NotEmpty(t, cluster.ID)
}

func TestClusterUseCase_GetCluster_NotFound(t *testing.T) {
	repo := newMockClusterRepo()
	uc := NewClusterUseCase(repo)

	_, err := uc.GetCluster(context.Background(), "nonexistent")

	require.Error(t, err)
	var appErr *shareddomain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "CLUSTER_NOT_FOUND", appErr.Code)
}

func TestClusterUseCase_ListClusters(t *testing.T) {
	repo := newMockClusterRepo()
	uc := NewClusterUseCase(repo)

	_, err := uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name: "cluster-a", Type: domain.ClusterTypePipeline, OrgID: "org_001",
	})
	require.NoError(t, err)

	_, err = uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name: "cluster-b", Type: domain.ClusterTypeTarget, OrgID: "org_001",
	})
	require.NoError(t, err)

	_, err = uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name: "cluster-c", Type: domain.ClusterTypePipeline, OrgID: "org_002",
	})
	require.NoError(t, err)

	clusters, err := uc.ListClusters(context.Background(), "org_001")
	require.NoError(t, err)
	assert.Len(t, clusters, 2)
}

func TestClusterUseCase_UpdateCluster_Success(t *testing.T) {
	repo := newMockClusterRepo()
	uc := NewClusterUseCase(repo)

	created, err := uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name: "old-name", Type: domain.ClusterTypePipeline, Endpoint: "https://old:6443", OrgID: "org_001",
	})
	require.NoError(t, err)

	updated, err := uc.UpdateCluster(context.Background(), created.ID, UpdateClusterInput{
		Name:     "new-name",
		Endpoint: "https://new:6443",
	})
	require.NoError(t, err)
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, "https://new:6443", updated.Endpoint)
}

func TestClusterUseCase_DeleteCluster_Success(t *testing.T) {
	repo := newMockClusterRepo()
	uc := NewClusterUseCase(repo)

	created, err := uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name: "to-delete", Type: domain.ClusterTypePipeline, OrgID: "org_001",
	})
	require.NoError(t, err)

	err = uc.DeleteCluster(context.Background(), created.ID)
	require.NoError(t, err)

	_, err = uc.GetCluster(context.Background(), created.ID)
	require.Error(t, err)
}

func TestClusterUseCase_DeleteCluster_InUseConflict(t *testing.T) {
	repo := newMockClusterRepo()
	repo.deleteErr = &pgconn.PgError{Code: "23503"}
	uc := NewClusterUseCase(repo)

	created, err := uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name: "in-use", Type: domain.ClusterTypePipeline, OrgID: "org_001",
	})
	require.NoError(t, err)

	err = uc.DeleteCluster(context.Background(), created.ID)
	require.Error(t, err)

	var appErr *shareddomain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "CLUSTER_IN_USE", appErr.Code)
	assert.Equal(t, 409, appErr.HTTPStatus)
}

func TestClusterUseCase_VerifyCluster_Success(t *testing.T) {
	repo := newMockClusterRepo()
	uc := NewClusterUseCase(repo)

	created, err := uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name: "pending-cluster", Type: domain.ClusterTypePipeline, OrgID: "org_001",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.ConnectionStatusPending, created.ConnectionStatus)

	verified, err := uc.VerifyCluster(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ConnectionStatusConnected, verified.ConnectionStatus)
}

func TestClusterUseCase_SaveAndGetKubeconfig(t *testing.T) {
	repo := newMockClusterRepo()
	uc := NewClusterUseCase(repo)

	created, err := uc.RegisterCluster(context.Background(), RegisterClusterInput{
		Name: "kube-cluster", Type: domain.ClusterTypePipeline, OrgID: "org_001",
	})
	require.NoError(t, err)

	plain := []byte("apiVersion: v1\nkind: Config\n")
	require.NoError(t, uc.SaveKubeconfig(context.Background(), created.ID, plain))

	stored, err := uc.GetKubeconfig(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, plain, stored)
}
