package repository

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// MemoryCICDTemplateRepository is an in-memory implementation of port.PipelineTemplateRepository
// with three hard-coded CI/CD pipeline templates.
type MemoryCICDTemplateRepository struct {
	templates map[string]*domain.PipelineTemplate
}

// NewMemoryCICDTemplateRepository constructs a MemoryCICDTemplateRepository with three
// canonical CI/CD pipeline templates pre-loaded.
func NewMemoryCICDTemplateRepository() *MemoryCICDTemplateRepository {
	repo := &MemoryCICDTemplateRepository{
		templates: make(map[string]*domain.PipelineTemplate),
	}
	for _, t := range cicdTemplates() {
		repo.templates[t.ID] = t
	}
	return repo
}

// GetByID returns the template with the given ID.
func (r *MemoryCICDTemplateRepository) GetByID(_ context.Context, id string) (*domain.PipelineTemplate, error) {
	t, ok := r.templates[id]
	if !ok {
		return nil, fmt.Errorf("pipeline template %q not found", id)
	}
	return t, nil
}

// List returns all available CI/CD pipeline templates.
func (r *MemoryCICDTemplateRepository) List(_ context.Context) ([]*domain.PipelineTemplate, error) {
	result := make([]*domain.PipelineTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		result = append(result, t)
	}
	return result, nil
}

// Create stores a new pipeline template.
func (r *MemoryCICDTemplateRepository) Create(_ context.Context, tmpl *domain.PipelineTemplate) error {
	r.templates[tmpl.ID] = tmpl
	return nil
}

// Update replaces an existing pipeline template.
func (r *MemoryCICDTemplateRepository) Update(_ context.Context, tmpl *domain.PipelineTemplate) error {
	if _, ok := r.templates[tmpl.ID]; !ok {
		return fmt.Errorf("pipeline template %q not found", tmpl.ID)
	}
	r.templates[tmpl.ID] = tmpl
	return nil
}

// Delete removes a pipeline template by ID.
func (r *MemoryCICDTemplateRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.templates[id]; !ok {
		return fmt.Errorf("pipeline template %q not found", id)
	}
	delete(r.templates, id)
	return nil
}

// cicdTemplates returns the three canonical CI/CD pipeline templates.
func cicdTemplates() []*domain.PipelineTemplate {
	return []*domain.PipelineTemplate{
		{
			ID:          "web-backend-v1",
			Name:        "Web Backend Pipeline",
			Description: "백엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 이미지 빌드, 배포 단계를 포함합니다.",
			AppType:     domain.AppTypeBackend,
			Stages:      []string{"Build", "Test", "ImageBuild", "Deploy"},
		},
		{
			ID:          "web-frontend-v1",
			Name:        "Web Frontend Pipeline",
			Description: "프론트엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 정적 빌드, 배포 단계를 포함합니다.",
			AppType:     domain.AppTypeWeb,
			Stages:      []string{"Build", "Test", "StaticBuild", "Deploy"},
		},
		{
			ID:          "batch-job-v1",
			Name:        "Batch Job Pipeline",
			Description: "배치 작업을 위한 CI/CD 파이프라인. 빌드, 이미지 빌드, CronJob 배포 단계를 포함합니다.",
			AppType:     domain.AppTypeBatch,
			Stages:      []string{"Build", "ImageBuild", "CronJobDeploy"},
		},
	}
}

// MemoryCICDGoldenPathRepository is an in-memory implementation of port.CICDGoldenPathRepository
// with three hard-coded CI/CD Golden Path templates.
type MemoryCICDGoldenPathRepository struct {
	goldenPaths map[string]*domain.CICDGoldenPath
}

// NewMemoryCICDGoldenPathRepository constructs a MemoryCICDGoldenPathRepository with three
// canonical CI/CD Golden Path templates pre-loaded.
func NewMemoryCICDGoldenPathRepository() *MemoryCICDGoldenPathRepository {
	repo := &MemoryCICDGoldenPathRepository{
		goldenPaths: make(map[string]*domain.CICDGoldenPath),
	}
	for _, gp := range cicdGoldenPaths() {
		repo.goldenPaths[gp.ID] = gp
	}
	return repo
}

// GetByID returns the golden path with the given ID.
func (r *MemoryCICDGoldenPathRepository) GetByID(_ context.Context, id string) (*domain.CICDGoldenPath, error) {
	gp, ok := r.goldenPaths[id]
	if !ok {
		return nil, fmt.Errorf("CI/CD golden path %q not found", id)
	}
	return gp, nil
}

// List returns all available CI/CD Golden Path templates.
func (r *MemoryCICDGoldenPathRepository) List(_ context.Context) ([]*domain.CICDGoldenPath, error) {
	result := make([]*domain.CICDGoldenPath, 0, len(r.goldenPaths))
	for _, gp := range r.goldenPaths {
		result = append(result, gp)
	}
	return result, nil
}

// Create stores a new golden path.
func (r *MemoryCICDGoldenPathRepository) Create(_ context.Context, goldenPath *domain.CICDGoldenPath) error {
	r.goldenPaths[goldenPath.ID] = goldenPath
	return nil
}

// Update replaces an existing golden path.
func (r *MemoryCICDGoldenPathRepository) Update(_ context.Context, goldenPath *domain.CICDGoldenPath) error {
	if _, ok := r.goldenPaths[goldenPath.ID]; !ok {
		return fmt.Errorf("CI/CD golden path %q not found", goldenPath.ID)
	}
	r.goldenPaths[goldenPath.ID] = goldenPath
	return nil
}

// Delete removes a golden path by ID.
func (r *MemoryCICDGoldenPathRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.goldenPaths[id]; !ok {
		return fmt.Errorf("CI/CD golden path %q not found", id)
	}
	delete(r.goldenPaths, id)
	return nil
}

// cicdGoldenPaths returns the three canonical CI/CD Golden Path templates.
func cicdGoldenPaths() []*domain.CICDGoldenPath {
	return []*domain.CICDGoldenPath{
		{
			ID:                   "gitlab-allinone-v1",
			Name:                 "GitLab All-in-One",
			Description:          "GitLab CE 기반 단일 플랫폼. 소스코드 관리, CI/CD, 컨테이너 레지스트리를 GitLab에서 통합 제공합니다.",
			EstimatedInstallTime: 90,
			RecommendedUseCase:   "중견기업, 단일 플랫폼 선호",
			MinResources:         "8 vCPU / 16Gi RAM / 100Gi Storage",
			Tools: []domain.CICDTool{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "container_registry", Name: "GitLab Registry", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
				{Category: "log_aggregation", Name: "Loki", HelmVersion: "6.6.2", AppVersion: "3.0.0"},
			},
		},
		{
			ID:                   "gitlab-argocd-v1",
			Name:                 "GitLab + Argo CD",
			Description:          "GitLab CI와 Harbor 레지스트리를 분리하여 GitOps 패턴을 강화한 구성입니다.",
			EstimatedInstallTime: 120,
			RecommendedUseCase:   "GitOps 중심 조직",
			MinResources:         "10 vCPU / 20Gi RAM / 130Gi Storage",
			Tools: []domain.CICDTool{
				{Category: "source_repository", Name: "GitLab CE", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "ci_platform", Name: "GitLab CI", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
				{Category: "container_registry", Name: "Harbor", HelmVersion: "1.14.0", AppVersion: "2.11.0"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
				{Category: "log_aggregation", Name: "Loki", HelmVersion: "6.6.2", AppVersion: "3.0.0"},
			},
		},
		{
			ID:                   "github-argocd-v1",
			Name:                 "GitHub + Argo CD",
			Description:          "GitHub와 GitHub Actions를 외부 서비스로 사용하고, 클러스터 내에는 Harbor + Argo CD + 모니터링만 설치합니다.",
			EstimatedInstallTime: 60,
			RecommendedUseCase:   "GitHub 사용 조직",
			MinResources:         "6 vCPU / 12Gi RAM / 80Gi Storage",
			Tools: []domain.CICDTool{
				{Category: "source_repository", Name: "GitHub", HelmVersion: "external", AppVersion: "external"},
				{Category: "ci_platform", Name: "GitHub Actions", HelmVersion: "external", AppVersion: "external"},
				{Category: "container_registry", Name: "Harbor", HelmVersion: "1.14.0", AppVersion: "2.11.0"},
				{Category: "storage_backend", Name: "MinIO", HelmVersion: "5.3.0", AppVersion: "2024.11.7"},
				{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
				{Category: "monitoring_collection", Name: "Prometheus", HelmVersion: "67.0.0", AppVersion: "3.1.0"},
				{Category: "monitoring_visualization", Name: "Grafana", HelmVersion: "8.5.0", AppVersion: "11.4.0"},
				{Category: "log_aggregation", Name: "Loki", HelmVersion: "6.6.2", AppVersion: "3.0.0"},
			},
		},
	}
}
