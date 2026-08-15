package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestMemoryTemplateRepository_ListReturnsSeededTemplates(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	templates, err := repo.List(context.Background())
	require.NoError(t, err)

	// 개수를 고정하면 템플릿을 하나 추가할 때마다 무관하게 깨진다.
	// 있어야 하는 것이 있는지를 본다.
	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		ids = append(ids, tmpl.ID)
	}
	assert.Subset(t, ids, []string{
		"empty-template-v1",
		"gitlab-allinone-v1",
		"gitlab-argocd-v1",
		"gitlab-harbor-v1",
		"gitlab-nexus-v1",
		"github-argocd-v1",
	})
}

// 도구를 고른 템플릿에는 대응하는 호환성 매트릭스가 있어야 한다.
// 없으면 Pre-Deploy Gate 가 판정할 근거가 없어 설치 직전에 막힌다 —
// 템플릿만 추가하고 매트릭스를 빠뜨리는 실수를 여기서 잡는다.
func TestSeededTemplates_HaveCompatibilityMatrix(t *testing.T) {
	templates, err := NewMemoryTemplateRepository().List(context.Background())
	require.NoError(t, err)
	matrices, err := NewMemoryCompatibilityRepository().GetAll(context.Background())
	require.NoError(t, err)

	known := make(map[string]struct{}, len(matrices))
	for _, m := range matrices {
		known[m.ID] = struct{}{}
	}

	for _, tmpl := range templates {
		if len(tmpl.Tools) == 0 {
			continue // 빈 템플릿은 고른 도구가 없어 판정할 대상이 없다.
		}
		_, ok := known[tmpl.ID]
		assert.Truef(t, ok, "template %s has no compatibility matrix", tmpl.ID)
	}
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

	toolByCategory := make(map[string]domain.ToolConfig, len(tmpl.Tools))
	for _, tool := range tmpl.Tools {
		toolByCategory[tool.Category] = tool
	}
	assert.Equal(t, "18.5.1", toolByCategory["source_repository"].AppVersion)
	assert.Equal(t, "v2.8.3", toolByCategory["cd_tool"].AppVersion)
	assert.Equal(t, "11.1.0", toolByCategory["monitoring_visualization"].AppVersion)

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

// 8Gi 노드에 들어가는 경량 템플릿.
//
// 고정비(PostgreSQL 2Gi + 게이트웨이 0.8Gi + OpenBao·ESO 0.6Gi + cert-manager
// 1.5Gi)만으로 5Gi 가 나간다. 남는 예산이 3Gi 남짓이라 무엇을 빼느냐가 이
// 템플릿의 전부다 — GitLab(4.5Gi)·Prometheus(5Gi, 벡터가 5개 컴포넌트에 그대로
// 실린다)·Nexus(1.5Gi 고정)는 이 예산에 들어가지 않는다.
func TestMemoryTemplateRepository_GetByID_LightweightTemplate(t *testing.T) {
	repo := NewMemoryTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "gitea-jenkins-argocd-lite-v1")
	require.NoError(t, err)

	assert.Equal(t, domain.PlanningProfileLocal, tmpl.PlanningProfile,
		"경량 템플릿은 Local 프로파일로 설치돼야 한다 — standard 로 깔리면 8Gi 에 들어가지 않는다")

	toolByCategory := make(map[string]domain.ToolConfig, len(tmpl.Tools))
	for _, tool := range tmpl.Tools {
		toolByCategory[tool.Category] = tool
	}
	assert.Equal(t, "Gitea", toolByCategory["source_repository"].Name)
	assert.Equal(t, "Jenkins", toolByCategory["ci_platform"].Name)
	assert.Equal(t, "Argo CD", toolByCategory["cd_tool"].Name)

	// 레지스트리는 뺄 수 없다. 없으면 파이프라인을 만드는 순간
	// registry.ResolverFor 가 "이미지 레지스트리를 결정할 수 없습니다" 로 막아
	// 스택은 서는데 아무것도 배포할 수 없는 템플릿이 된다(실측 확인).
	// 8Gi 안에서 세울 수 있는 레지스트리는 Harbor 뿐이다 — Nexus 는 JVM 고정으로
	// 1.5Gi 를 요청한다.
	assert.Equal(t, "Harbor", toolByCategory["container_registry"].Name)

	for _, category := range []string{
		"storage_backend",          // MinIO 는 GitLab 이 없으면 쓸 곳이 없다
		"monitoring_collection",    // Prometheus 하나로 예산을 다 쓴다
		"monitoring_visualization", //
	} {
		assert.NotContainsf(t, toolByCategory, category,
			"경량 템플릿에 %s 가 들어가면 8Gi 예산을 넘는다", category)
	}
}

func TestSeededTemplates_HaveValidPlanningProfile(t *testing.T) {
	templates, err := NewMemoryTemplateRepository().List(context.Background())
	require.NoError(t, err)

	for _, tmpl := range templates {
		t.Run(tmpl.ID, func(t *testing.T) {
			assert.NotEmpty(t, domain.NormalizePlanningProfile(tmpl.PlanningProfile),
				"planning profile %q is not a known profile", tmpl.PlanningProfile)
		})
	}
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
