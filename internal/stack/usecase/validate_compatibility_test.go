package usecase

import (
	"context"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCompatibility_MatchFound(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository": "GitLab CE",
			"ci_platform":       "GitLab CI",
			"cd_tool":           "Argo CD",
		},
	})

	require.NoError(t, err)
	assert.True(t, out.Compatible)
	assert.NotNil(t, out.Matrix)
	assert.NotEmpty(t, out.Message)
	assert.Equal(t, "pass", out.Overall.State)
	assert.Equal(t, 100, out.Overall.Score)
	assert.Empty(t, out.Issues)
	assert.False(t, out.CheckedAt.IsZero())
}

func TestValidateCompatibility_NoMatch(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"ci_platform": "Jenkins",
			"cd_tool":     "Spinnaker",
		},
	})

	require.NoError(t, err)
	assert.False(t, out.Compatible)
	assert.Nil(t, out.Matrix)
	assert.Equal(t, "fail", out.Overall.State)
	assert.NotEmpty(t, out.Issues)
	assert.Equal(t, "MATRIX_NOT_FOUND", out.Issues[0].Code)
	assert.False(t, out.CheckedAt.IsZero())
}

func TestValidateCompatibility_EmptyTools(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	_, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tools map must not be empty")
}

func TestValidateCompatibility_GitHubArgoCD(t *testing.T) {
	repo := repository.NewMemoryCompatibilityRepository()
	uc := NewValidateCompatibility(repo)

	out, err := uc.Execute(context.Background(), ValidateCompatibilityInput{
		Tools: map[string]string{
			"source_repository": "GitHub",
			"ci_platform":       "GitHub Actions",
			"cd_tool":           "Argo CD",
		},
	})

	require.NoError(t, err)
	assert.True(t, out.Compatible)
	assert.NotNil(t, out.Matrix)
	assert.Equal(t, "untested", out.Matrix.Status)
	assert.Equal(t, "warn", out.Overall.State)
	assert.Equal(t, 70, out.Overall.Score)
	assert.NotEmpty(t, out.Issues)
	assert.Equal(t, "MATRIX_UNTESTED", out.Issues[0].Code)
	assert.False(t, out.CheckedAt.IsZero())
}
