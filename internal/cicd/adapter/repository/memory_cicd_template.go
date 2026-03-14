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
