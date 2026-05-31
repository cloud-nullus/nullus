package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func TestMemoryCICDTemplateRepository_ListReturnsCanonicalTemplates(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	templates, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, templates, 2, "should have exactly 2 CI/CD pipeline templates")
}

func TestMemoryCICDTemplateRepository_GetByID_WebBackend(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "web-backend-v1")
	require.NoError(t, err)

	assert.Equal(t, "web-backend-v1", tmpl.ID)
	assert.Equal(t, "User Custom Pipeline", tmpl.Name)
	assert.Equal(t, []string{"Build", "Test", "ImageBuild", "Deploy"}, tmpl.Stages)
}

func TestMemoryCICDTemplateRepository_DoesNotReturnRemovedWebFrontend(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	_, err := repo.GetByID(context.Background(), "web-frontend-v1")
	require.Error(t, err)
}

func TestMemoryCICDTemplateRepository_GetByID_BatchJob(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "batch-job-v1")
	require.NoError(t, err)

	assert.Equal(t, "batch-job-v1", tmpl.ID)
	assert.Equal(t, "Batch Job Pipeline", tmpl.Name)
	assert.Equal(t, []string{"Build", "ImageBuild", "CronJobDeploy"}, tmpl.Stages)
}

func TestMemoryCICDTemplateRepository_GetByID_NotFound(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	_, err := repo.GetByID(context.Background(), "nonexistent-template")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMemoryCICDTemplateRepository_ListContainsCanonicalTemplateIDs(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	templates, err := repo.List(context.Background())
	require.NoError(t, err)

	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		ids = append(ids, tmpl.ID)
	}

	assert.ElementsMatch(t, []string{
		"web-backend-v1",
		"batch-job-v1",
	}, ids)
}

func TestMemoryCICDTemplateRepository_GetByID_ReturnsExpectedAppType(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "web-backend-v1")
	require.NoError(t, err)

	assert.Equal(t, domain.AppTypeBackend, tmpl.AppType)
}
