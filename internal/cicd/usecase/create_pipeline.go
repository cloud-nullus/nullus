package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// CreatePipelineInput holds the parameters for creating a new pipeline.
type CreatePipelineInput struct {
	Name       string
	TemplateID string
	OrgID      string
	ClusterID  string
	Namespace  string
	AppType    domain.AppType
	GitRepoURL string
}

// CreatePipelineOutput holds the result of creating a pipeline.
type CreatePipelineOutput struct {
	Pipeline *domain.Pipeline
}

// CreatePipeline creates a new pipeline configuration.
type CreatePipeline struct {
	pipelineRepo port.PipelineRepository
	templateRepo port.PipelineTemplateRepository
}

// NewCreatePipeline constructs a CreatePipeline use case.
func NewCreatePipeline(pipelineRepo port.PipelineRepository, templateRepo port.PipelineTemplateRepository) *CreatePipeline {
	return &CreatePipeline{
		pipelineRepo: pipelineRepo,
		templateRepo: templateRepo,
	}
}

// Execute creates a new pipeline.
func (uc *CreatePipeline) Execute(ctx context.Context, input CreatePipelineInput) (*CreatePipelineOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("pipeline name is required")
	}
	if input.OrgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}
	if input.ClusterID == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	if input.TemplateID != "" {
		if _, err := uc.templateRepo.GetByID(ctx, input.TemplateID); err != nil {
			return nil, fmt.Errorf("template not found: %w", err)
		}
	}

	pipeline := &domain.Pipeline{
		ID:         generateID("pip"),
		Name:       input.Name,
		TemplateID: input.TemplateID,
		OrgID:      input.OrgID,
		ClusterID:  input.ClusterID,
		Namespace:  input.Namespace,
		AppType:    input.AppType,
		GitRepoURL: input.GitRepoURL,
		Status:     domain.PipelineStatusActive,
		CreatedAt:  time.Now(),
	}

	if err := uc.pipelineRepo.Create(ctx, pipeline); err != nil {
		return nil, fmt.Errorf("create pipeline: %w", err)
	}

	return &CreatePipelineOutput{Pipeline: pipeline}, nil
}
