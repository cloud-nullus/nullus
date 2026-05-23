package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ListPipelinesInput holds the parameters for listing pipelines.
type ListPipelinesInput struct {
	OrgID   string
	StackID string // optional — filters to pipelines linked to this stack
}

// ListPipelinesOutput holds the result of listing pipelines.
type ListPipelinesOutput struct {
	Pipelines []*domain.Pipeline
}

// ListPipelines lists all pipelines for an organization.
type ListPipelines struct {
	pipelineRepo port.PipelineRepository
}

// NewListPipelines constructs a ListPipelines use case.
func NewListPipelines(pipelineRepo port.PipelineRepository) *ListPipelines {
	return &ListPipelines{pipelineRepo: pipelineRepo}
}

// Execute returns all pipelines for the given organization.
// When StackID is set, results are filtered to that stack via the dedicated repo method.
func (uc *ListPipelines) Execute(ctx context.Context, input ListPipelinesInput) (*ListPipelinesOutput, error) {
	var (
		pipelines []*domain.Pipeline
		err       error
	)
	if input.StackID != "" {
		pipelines, err = uc.pipelineRepo.ListByStackID(ctx, input.StackID)
	} else {
		pipelines, err = uc.pipelineRepo.List(ctx, input.OrgID)
	}
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	return &ListPipelinesOutput{Pipelines: pipelines}, nil
}
