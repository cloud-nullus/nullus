package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// Sentinel errors for cross-context validation.
var (
	ErrStackNotFound   = errors.New("referenced stack does not exist")
	ErrStackOrgMismatch = errors.New("stack belongs to a different organization")
)

// CreatePipelineInput holds the parameters for creating a new pipeline.
type CreatePipelineInput struct {
	Name       string
	TemplateID string
	OrgID      string
	ClusterID  string
	StackID    string // optional — links pipeline to a stack
	Namespace  string
	AppType    domain.AppType
	GitRepoURL string
}

// CreatePipelineOutput holds the result of creating a pipeline.
type CreatePipelineOutput struct {
	Pipeline     *domain.Pipeline
	StackWarning string `json:"stack_warning,omitempty"` // non-empty when stack exists but is not completed
}

// CreatePipeline creates a new pipeline configuration.
type CreatePipeline struct {
	pipelineRepo port.PipelineRepository
	templateRepo port.PipelineTemplateRepository
	stackReader  port.StackReader // optional — nil disables stack validation
}

// NewCreatePipeline constructs a CreatePipeline use case.
func NewCreatePipeline(
	pipelineRepo port.PipelineRepository,
	templateRepo port.PipelineTemplateRepository,
	stackReader ...port.StackReader,
) *CreatePipeline {
	uc := &CreatePipeline{
		pipelineRepo: pipelineRepo,
		templateRepo: templateRepo,
	}
	if len(stackReader) > 0 {
		uc.stackReader = stackReader[0]
	}
	return uc
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

	// --- Cross-context validation: Stack reference ---
	var stackWarning string
	if input.StackID != "" && uc.stackReader != nil {
		summary, err := uc.stackReader.GetStackSummary(ctx, input.StackID)
		if err != nil {
			return nil, fmt.Errorf("validate stack: %w", err)
		}
		if summary == nil {
			return nil, ErrStackNotFound
		}
		if summary.OrgID != input.OrgID {
			return nil, ErrStackOrgMismatch
		}
		// Warn (but allow) when stack is not yet deployed.
		if summary.State != "completed" {
			stackWarning = fmt.Sprintf(
				"stack %q is in state %q — CI/CD tools may not be available yet",
				input.StackID, summary.State,
			)
		}
	}

	pipeline := &domain.Pipeline{
		ID:         generateID("pip"),
		Name:       input.Name,
		TemplateID: input.TemplateID,
		OrgID:      input.OrgID,
		ClusterID:  input.ClusterID,
		StackID:    input.StackID,
		Namespace:  input.Namespace,
		AppType:    input.AppType,
		GitRepoURL: input.GitRepoURL,
		Status:     domain.PipelineStatusActive,
		CreatedAt:  time.Now(),
	}

	if err := uc.pipelineRepo.Create(ctx, pipeline); err != nil {
		return nil, fmt.Errorf("create pipeline: %w", err)
	}

	return &CreatePipelineOutput{
		Pipeline:     pipeline,
		StackWarning: stackWarning,
	}, nil
}
