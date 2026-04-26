package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

func seedCluster(id string, archs []string) *domain.Cluster {
	now := time.Now().UTC()
	return &domain.Cluster{
		ID:                id,
		Name:              "test-" + id,
		Type:              domain.ClusterTypeTarget,
		OrgID:             "org-1",
		CloudProvider:     domain.CloudProviderOnPremise,
		Endpoint:          "https://" + id + ".test",
		ConnectionStatus:  domain.ConnectionStatusConnected,
		NodeArchitectures: archs,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func TestMemoryClusterRepository_NodeArchitectures_RoundTrip(t *testing.T) {
	repo := NewMemoryClusterRepository()
	ctx := context.Background()

	// Create with unsorted + duplicate input; store should normalize.
	cluster := seedCluster("c1", []string{"arm64", "amd64", "arm64"})
	require.NoError(t, repo.Create(ctx, cluster))

	got, err := repo.GetByID(ctx, "c1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, []string{"amd64", "arm64"}, got.NodeArchitectures)
}

func TestMemoryClusterRepository_NodeArchitectures_DeepCopy(t *testing.T) {
	repo := NewMemoryClusterRepository()
	ctx := context.Background()

	cluster := seedCluster("c2", []string{"amd64"})
	require.NoError(t, repo.Create(ctx, cluster))

	// Mutating the caller's slice must not leak into the store.
	cluster.NodeArchitectures[0] = "arm64"

	got, err := repo.GetByID(ctx, "c2")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, []string{"amd64"}, got.NodeArchitectures)

	// Mutating the returned slice must not leak either.
	got.NodeArchitectures[0] = "ppc64le"
	got2, err := repo.GetByID(ctx, "c2")
	require.NoError(t, err)
	assert.Equal(t, []string{"amd64"}, got2.NodeArchitectures)
}

func TestMemoryClusterRepository_UpdateReplacesNodeArchitectures(t *testing.T) {
	repo := NewMemoryClusterRepository()
	ctx := context.Background()

	cluster := seedCluster("c3", []string{"amd64"})
	require.NoError(t, repo.Create(ctx, cluster))

	// Discovery sees an additional arm64 node — caller normalizes and persists.
	cluster.NodeArchitectures = []string{"arm64", "amd64"}
	require.NoError(t, repo.Update(ctx, cluster))

	got, err := repo.GetByID(ctx, "c3")
	require.NoError(t, err)
	assert.Equal(t, []string{"amd64", "arm64"}, got.NodeArchitectures)
}

func TestMemoryClusterRepository_EmptyArchSliceIsNilOnRead(t *testing.T) {
	repo := NewMemoryClusterRepository()
	ctx := context.Background()

	cluster := seedCluster("c4", nil)
	require.NoError(t, repo.Create(ctx, cluster))

	got, err := repo.GetByID(ctx, "c4")
	require.NoError(t, err)
	assert.Nil(t, got.NodeArchitectures)
}
