package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// CreateStackInput holds the parameters for creating a new stack.
type CreateStackInput struct {
	Name       string
	OrgID      string
	ClusterID  string
	TemplateID string
	Config     domain.StackConfig
}

// CreateStackOutput holds the result of creating a stack.
type CreateStackOutput struct {
	Stack *domain.Stack
}

// CreateStack creates a new stack configuration, optionally loading defaults from a template.
type CreateStack struct {
	stackRepo    port.StackRepository
	templateRepo port.TemplateRepository
}

// NewCreateStack constructs a CreateStack use case.
func NewCreateStack(stackRepo port.StackRepository, templateRepo port.TemplateRepository) *CreateStack {
	return &CreateStack{
		stackRepo:    stackRepo,
		templateRepo: templateRepo,
	}
}

// Execute creates a new stack, merging template defaults when a TemplateID is provided.
func (uc *CreateStack) Execute(ctx context.Context, input CreateStackInput) (*CreateStackOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("stack name is required")
	}
	if input.ClusterID == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	if input.OrgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}

	now := time.Now()
	stack := &domain.Stack{
		ID:         generateID("stk"),
		Name:       input.Name,
		TemplateID: input.TemplateID,
		OrgID:      input.OrgID,
		ClusterID:  input.ClusterID,
		State:      domain.StatePending,
		Config:     input.Config,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := uc.stackRepo.Create(ctx, stack); err != nil {
		return nil, fmt.Errorf("create stack: %w", err)
	}

	return &CreateStackOutput{Stack: stack}, nil
}
