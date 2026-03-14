package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// MemoryTemplateRepository is an in-memory implementation of port.TemplateRepository
// with three hard-coded Golden Path templates.
type MemoryTemplateRepository struct {
	templates map[string]*domain.Template
}

// NewMemoryTemplateRepository constructs a MemoryTemplateRepository with the three
// canonical Golden Path templates pre-loaded.
func NewMemoryTemplateRepository() *MemoryTemplateRepository {
	repo := &MemoryTemplateRepository{
		templates: make(map[string]*domain.Template),
	}
	for _, t := range goldenPathTemplates() {
		repo.templates[t.ID] = t
	}
	return repo
}

// GetByID returns the template with the given ID.
func (r *MemoryTemplateRepository) GetByID(_ context.Context, id string) (*domain.Template, error) {
	t, ok := r.templates[id]
	if !ok {
		return nil, fmt.Errorf("template %q not found", id)
	}
	return t, nil
}

// List returns all available templates.
func (r *MemoryTemplateRepository) List(_ context.Context) ([]*domain.Template, error) {
	result := make([]*domain.Template, 0, len(r.templates))
	for _, t := range r.templates {
		result = append(result, t)
	}
	return result, nil
}

// goldenPathTemplates returns the three canonical Golden Path templates.
func goldenPathTemplates() []*domain.Template {
	return []*domain.Template{
		{
			ID:          "gitlab-allinone-v1",
			Name:        "GitLab All-in-One",
			Description: "GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "container_registry", Name: "GitLab Registry", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
			},
			EstimatedInstallTime: 90 * time.Minute,
			RecommendedUseCase:   "중견기업, 단일 플랫폼 선호",
			MinResources:         "8 vCPU / 16Gi RAM / 100Gi Storage",
		},
		{
			ID:          "gitlab-argocd-v1",
			Name:        "GitLab + Argo CD",
			Description: "GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "container_registry", Name: "Harbor", HelmVersion: "1.14.0", AppVersion: "2.11.0"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
			},
			EstimatedInstallTime: 120 * time.Minute,
			RecommendedUseCase:   "GitOps 중심 조직",
			MinResources:         "10 vCPU / 20Gi RAM / 130Gi Storage",
		},
		{
			ID:          "github-argocd-v1",
			Name:        "GitHub + Argo CD",
			Description: "GitHub와 GitHub Actions를 외부 서비스로 사용하고, 클러스터 내에는 Harbor + Argo CD + 모니터링만 설치합니다.",
			Tools: []domain.ToolConfig{
				{Category: "source_repository", Name: "GitHub", HelmVersion: "external", AppVersion: "external"},
				{Category: "ci_platform", Name: "GitHub Actions", HelmVersion: "external", AppVersion: "external"},
				{Category: "container_registry", Name: "Harbor", HelmVersion: "1.14.0", AppVersion: "2.11.0"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
			},
			EstimatedInstallTime: 60 * time.Minute,
			RecommendedUseCase:   "GitHub 사용 조직",
			MinResources:         "6 vCPU / 12Gi RAM / 80Gi Storage",
		},
	}
}
