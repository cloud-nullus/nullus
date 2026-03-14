package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCompatibilityRepository_GetAll(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	matrices, err := repo.GetAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, matrices, 3)
}

func TestMemoryCompatibilityRepository_GetByID(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	m, err := repo.GetByID(context.Background(), "gitlab-allinone-v1")
	require.NoError(t, err)
	assert.Equal(t, "GitLab All-in-One", m.Name)
	assert.Equal(t, "verified", m.Status)
}

func TestMemoryCompatibilityRepository_GetByID_NotFound(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	_, err := repo.GetByID(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMemoryCompatibilityRepository_Validate_Match(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	m, err := repo.Validate(context.Background(), map[string]string{
		"source_repository": "GitLab CE",
		"cd_tool":           "Argo CD",
	})
	require.NoError(t, err)
	assert.NotNil(t, m)
}

func TestMemoryCompatibilityRepository_Validate_NoMatch(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	_, err := repo.Validate(context.Background(), map[string]string{
		"ci_platform": "Jenkins",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no compatible matrix found")
}

func TestMemoryCompatibilityRepository_KubernetesCompat(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	m, err := repo.GetByID(context.Background(), "github-argocd-v1")
	require.NoError(t, err)
	assert.Equal(t, "1.27", m.Kubernetes.Min)
	assert.Equal(t, "1.32", m.Kubernetes.Max)
	assert.Equal(t, "1.29", m.Kubernetes.Recommended)
}
