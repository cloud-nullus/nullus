package usecase

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCacheClearer struct {
	cleared atomic.Int32
}

func (f *fakeCacheClearer) Clear() { f.cleared.Add(1) }

func validPayload(id string) *domain.CompatibilityMatrix {
	return &domain.CompatibilityMatrix{
		ID:     id,
		Name:   "Valid Matrix " + id,
		Status: "untested",
		Kubernetes: domain.KubernetesCompat{
			Min: "1.27", Max: "1.35", Recommended: "1.35",
		},
		Tools: map[string]domain.ToolVersion{
			"db": {
				Name: "Postgres", HelmVersion: "12.0.0", AppVersion: "16.0",
				Tier: "stable", ArchSupport: []string{"amd64", "arm64"},
			},
		},
	}
}

func TestManageCompatibility_Create_Valid_ClearsCache(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	cache := &fakeCacheClearer{}
	uc := NewManageCompatibility(repo, WithVerdictCacheClearer(cache))

	require.NoError(t, uc.Create(context.Background(), validPayload("mc-v1")))
	assert.EqualValues(t, 1, cache.cleared.Load())
}

func TestManageCompatibility_Create_InvalidStatus_Rejected(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewManageCompatibility(repo)

	m := validPayload("mc-bad-status")
	m.Status = "bogus"
	err := uc.Create(context.Background(), m)
	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestManageCompatibility_Create_InvalidArch_Rejected(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewManageCompatibility(repo)

	m := validPayload("mc-bad-arch")
	m.Tools["db"] = domain.ToolVersion{
		Name: "Postgres", HelmVersion: "12.0.0", AppVersion: "16.0",
		Tier: "stable", ArchSupport: []string{"s390x"},
	}
	err := uc.Create(context.Background(), m)
	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestManageCompatibility_Create_EmptyTools_Rejected(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewManageCompatibility(repo)

	m := validPayload("mc-empty-tools")
	m.Tools = map[string]domain.ToolVersion{}
	err := uc.Create(context.Background(), m)
	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestManageCompatibility_Update_NotFound_Forwards(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewManageCompatibility(repo)

	err := uc.Update(context.Background(), validPayload("not-there"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, port.ErrCompatibilityMatrixNotFound))
}

func TestManageCompatibility_Delete_CacheCleared(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	cache := &fakeCacheClearer{}
	uc := NewManageCompatibility(repo, WithVerdictCacheClearer(cache))

	require.NoError(t, uc.Create(context.Background(), validPayload("del-v1")))
	cache.cleared.Store(0) // reset after Create
	require.NoError(t, uc.Delete(context.Background(), "del-v1"))
	assert.EqualValues(t, 1, cache.cleared.Load())
}

func TestManageCompatibility_Delete_MissingID_Rejected(t *testing.T) {
	uc := NewManageCompatibility(repository.NewMemoryCompatibilityRepository())
	err := uc.Delete(context.Background(), "")
	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestManageCompatibility_NilCacheClearer_NoPanic(t *testing.T) {
	uc := NewManageCompatibility(repository.NewMemoryCompatibilityRepository())
	require.NoError(t, uc.Create(context.Background(), validPayload("no-cache")))
}
