package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// ListStacksInput holds parameters for listing stacks.
type ListStacksInput struct {
	OrgID          string
	IncludeDeleted bool
}

// ListStacksOutput holds the result of listing stacks.
type ListStacksOutput struct {
	Stacks []*domain.Stack
}

// ListStacks retrieves all stacks belonging to an organization.
type ListStacks struct {
	stackRepo port.StackRepository
}

// NewListStacks constructs a ListStacks use case.
func NewListStacks(stackRepo port.StackRepository) *ListStacks {
	return &ListStacks{stackRepo: stackRepo}
}

// Execute lists stacks for the given organization.
func (uc *ListStacks) Execute(ctx context.Context, input ListStacksInput) (*ListStacksOutput, error) {
	if input.OrgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}

	stacks, err := uc.stackRepo.List(ctx, input.OrgID, input.IncludeDeleted)
	if err != nil {
		return nil, fmt.Errorf("list stacks: %w", err)
	}

	return &ListStacksOutput{Stacks: stacks}, nil
}
