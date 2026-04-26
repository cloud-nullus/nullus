package repository

import (
	"context"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryTemplateRepository_ListReturnsSeededTemplates(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	templates, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, templates, 4, "should have exactly 4 Golden Path templates")
}

func TestMemoryTemplateRepository_GetByID_EmptyTemplate(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "empty-template-v1")
	require.NoError(t, err)

	assert.Equal(t, "empty-template-v1", tmpl.ID)
	assert.Equal(t, "Empty Template", tmpl.Name)
	assert.Empty(t, tmpl.Tools)
	assert.Greater(t, tmpl.EstimatedInstallTime.Minutes(), 0.0)
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

	var hasGitLabRegistry bool
	for _, tool := range tmpl.Tools {
		if tool.Name == "GitLab Registry" {
			hasGitLabRegistry = true
		}
	}
	assert.True(t, hasGitLabRegistry, "GitLab + Argo CD template should use GitLab Registry")
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
			assert.Greater(t, tmpl.EstimatedInstallTime.Minutes(), 0.0, "EstimatedInstallTime must be positive")
			assert.NotEmpty(t, tmpl.MinResources, "MinResources must not be empty")
		})
	}
}

func TestMemoryTemplateRepository_Create(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	tmpl := &domain.Template{
		ID:                   "custom-template-v1",
		Name:                 "Custom Template",
		Description:          "Custom description",
		Tools:                []domain.ToolConfig{{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"}},
		EstimatedInstallTime: 30 * time.Minute,
		RecommendedUseCase:   "테스트",
		MinResources:         "2 vCPU / 4Gi RAM / 20Gi Storage",
	}

	require.NoError(t, repo.Create(context.Background(), tmpl))

	got, err := repo.GetByID(context.Background(), tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, tmpl.Name, got.Name)
}

func TestMemoryTemplateRepository_Update(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	original, err := repo.GetByID(context.Background(), "gitlab-allinone-v1")
	require.NoError(t, err)

	updated := &domain.Template{
		ID:                   original.ID,
		Name:                 "GitLab All-in-One Updated",
		Description:          original.Description,
		Tools:                original.Tools,
		EstimatedInstallTime: original.EstimatedInstallTime,
		RecommendedUseCase:   original.RecommendedUseCase,
		MinResources:         original.MinResources,
	}

	require.NoError(t, repo.Update(context.Background(), updated))

	got, err := repo.GetByID(context.Background(), original.ID)
	require.NoError(t, err)
	assert.Equal(t, "GitLab All-in-One Updated", got.Name)
}

func TestMemoryTemplateRepository_Delete(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	require.NoError(t, repo.Delete(context.Background(), "gitlab-allinone-v1"))

	_, err := repo.GetByID(context.Background(), "gitlab-allinone-v1")
	require.Error(t, err)
}
