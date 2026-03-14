package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryTemplateRepository_ListReturnsThreeTemplates(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	templates, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, templates, 3, "should have exactly 3 Golden Path templates")
}

func TestMemoryTemplateRepository_GetByID_GitLabAllInOne(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "gitlab-allinone-v1")
	require.NoError(t, err)

	assert.Equal(t, "gitlab-allinone-v1", tmpl.ID)
	assert.Equal(t, "GitLab All-in-One", tmpl.Name)
	assert.NotEmpty(t, tmpl.Tools)
	assert.Greater(t, tmpl.EstimatedInstallTime.Minutes(), 0.0)

	// Verify all required tool categories are present
	categories := make(map[string]bool)
	for _, tool := range tmpl.Tools {
		categories[tool.Category] = true
	}
	assert.True(t, categories["source_repository"], "should have source_repository")
	assert.True(t, categories["ci_platform"], "should have ci_platform")
	assert.True(t, categories["cd_tool"], "should have cd_tool")
}

func TestMemoryTemplateRepository_GetByID_GitLabArgoCD(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "gitlab-argocd-v1")
	require.NoError(t, err)

	assert.Equal(t, "gitlab-argocd-v1", tmpl.ID)
	assert.Equal(t, "GitLab + Argo CD", tmpl.Name)
	assert.NotEmpty(t, tmpl.Tools)

	// Harbor should be the container registry for this template
	var hasHarbor bool
	for _, tool := range tmpl.Tools {
		if tool.Name == "Harbor" {
			hasHarbor = true
		}
	}
	assert.True(t, hasHarbor, "GitLab + Argo CD template should use Harbor")
}

func TestMemoryTemplateRepository_GetByID_GitHubArgoCD(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "github-argocd-v1")
	require.NoError(t, err)

	assert.Equal(t, "github-argocd-v1", tmpl.ID)
	assert.Equal(t, "GitHub + Argo CD", tmpl.Name)
	assert.NotEmpty(t, tmpl.Tools)

	// GitHub and GitHub Actions should be marked external
	var githubTool, githubActions bool
	for _, tool := range tmpl.Tools {
		if tool.Name == "GitHub" && tool.AppVersion == "external" {
			githubTool = true
		}
		if tool.Name == "GitHub Actions" && tool.AppVersion == "external" {
			githubActions = true
		}
	}
	assert.True(t, githubTool, "should have GitHub (external)")
	assert.True(t, githubActions, "should have GitHub Actions (external)")
}

func TestMemoryTemplateRepository_GetByID_NotFound(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	_, err := repo.GetByID(context.Background(), "nonexistent-template")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMemoryTemplateRepository_AllTemplatesHaveRequiredFields(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	templates, err := repo.List(context.Background())
	require.NoError(t, err)

	for _, tmpl := range templates {
		t.Run(tmpl.ID, func(t *testing.T) {
			assert.NotEmpty(t, tmpl.ID, "ID must not be empty")
			assert.NotEmpty(t, tmpl.Name, "Name must not be empty")
			assert.NotEmpty(t, tmpl.Description, "Description must not be empty")
			assert.NotEmpty(t, tmpl.Tools, "Tools must not be empty")
			assert.Greater(t, tmpl.EstimatedInstallTime.Minutes(), 0.0, "EstimatedInstallTime must be positive")
			assert.NotEmpty(t, tmpl.MinResources, "MinResources must not be empty")
		})
	}
}
