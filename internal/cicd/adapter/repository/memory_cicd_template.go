package repository

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/scaffold"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// MemoryCICDTemplateRepository is an in-memory implementation of port.PipelineTemplateRepository
// with hard-coded CI/CD pipeline templates.
type MemoryCICDTemplateRepository struct {
	templates map[string]*domain.PipelineTemplate
}

// NewMemoryCICDTemplateRepository constructs a MemoryCICDTemplateRepository with
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
		return nil, fmt.Errorf("pipeline template %q: %w", id, domain.ErrTemplateNotFound)
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

// Create stores a new pipeline template. Postgres 저장소와 같이 같은 ID 는 거부한다.
func (r *MemoryCICDTemplateRepository) Create(_ context.Context, tmpl *domain.PipelineTemplate) error {
	if _, ok := r.templates[tmpl.ID]; ok {
		return fmt.Errorf("pipeline template %q: %w", tmpl.ID, domain.ErrTemplateAlreadyExists)
	}
	r.templates[tmpl.ID] = tmpl
	return nil
}

// Update replaces an existing pipeline template.
func (r *MemoryCICDTemplateRepository) Update(_ context.Context, tmpl *domain.PipelineTemplate) error {
	if _, ok := r.templates[tmpl.ID]; !ok {
		return fmt.Errorf("pipeline template %q: %w", tmpl.ID, domain.ErrTemplateNotFound)
	}
	r.templates[tmpl.ID] = tmpl
	return nil
}

// Delete removes a pipeline template by ID.
func (r *MemoryCICDTemplateRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.templates[id]; !ok {
		return fmt.Errorf("pipeline template %q: %w", id, domain.ErrTemplateNotFound)
	}
	delete(r.templates, id)
	return nil
}

// cicdTemplates returns the canonical CI/CD pipeline templates.
func cicdTemplates() []*domain.PipelineTemplate {
	return []*domain.PipelineTemplate{
		{
			ID:          "web-backend-v1",
			Name:        "User Custom Pipeline",
			Description: "백엔드 서비스를 위한 CI/CD 파이프라인. 빌드, 테스트, 이미지 빌드, 배포 단계를 포함합니다.",
			AppType:     domain.AppTypeBackend,
			Stages:      scaffold.PipelineStageNames(scaffold.StageOptions{}),
		},
		{
			ID:          "batch-job-v1",
			Name:        "Batch Job Pipeline",
			Description: "배치 작업을 위한 CI/CD 파이프라인. 빌드, 이미지 빌드, CronJob 배포 단계를 포함합니다.",
			AppType:     domain.AppTypeBatch,
			Stages:      scaffold.PipelineStageNames(scaffold.StageOptions{}),
		},
	}
}
