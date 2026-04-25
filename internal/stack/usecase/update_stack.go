package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// UpdateStackInput holds the parameters for mutating an existing stack that
// hasn't yet completed installation. F8 follow-up Phase 4: closes the
// "orphan stack" gap where F8-F3's createStack → validate → fail flow left
// a persisted stack with no way to amend tools and resubmit.
type UpdateStackInput struct {
	StackID   string
	Name      *string
	ClusterID *string
	Namespace *string
	Config    *domain.StackConfig
	Tools     []domain.ToolConfig
}

// UpdateStackOutput returns the updated stack for easy chaining.
type UpdateStackOutput struct {
	Stack *domain.Stack
}

// UpdateStack permits in-place edits of stacks in state pending or failed.
// The prior config is snapshotted to HistoryRepository before the mutation
// lands so operators can rollback via the existing /rollback endpoint.
type UpdateStack struct {
	stackRepo     port.StackRepository
	manageHistory *ManageHistory
}

// NewUpdateStack wires the usecase. manageHistory may be nil — when absent,
// the usecase still updates but does not record history.
func NewUpdateStack(stackRepo port.StackRepository, manageHistory *ManageHistory) *UpdateStack {
	return &UpdateStack{stackRepo: stackRepo, manageHistory: manageHistory}
}

var updatableStates = map[domain.DeploymentState]struct{}{
	domain.StatePending: {},
	domain.StateFailed:  {},
}

// Execute applies the requested fields to the stack. Returns
// STACK_UPDATE_INVALID_STATE-shaped errors so the handler layer can map to
// HTTP 409 without string matching.
func (uc *UpdateStack) Execute(ctx context.Context, input UpdateStackInput) (*UpdateStackOutput, error) {
	if input.StackID == "" {
		return nil, fmt.Errorf("stack_id is required")
	}
	stack, err := uc.stackRepo.GetByID(ctx, input.StackID)
	if err != nil {
		return nil, fmt.Errorf("get stack: %w", err)
	}
	if stack == nil {
		return nil, fmt.Errorf("stack %q not found", input.StackID)
	}
	if _, ok := updatableStates[stack.State]; !ok {
		return nil, fmt.Errorf("stack state %q is not updatable", stack.State)
	}

	// Snapshot the current config into history before we mutate. Best-effort;
	// a history failure shouldn't block the update itself.
	if uc.manageHistory != nil {
		if cfg, ok := stackConfigFromInterface(stack.Config); ok {
			_, _ = uc.manageHistory.SaveVersion(ctx, SaveVersionInput{
				StackID:      stack.ID,
				Config:       cfg,
				ChangedBy:    "system",
				ChangeReason: "pre-update snapshot",
			})
		}
	}

	if input.Name != nil {
		stack.Name = *input.Name
	}
	if input.ClusterID != nil {
		stack.ClusterID = *input.ClusterID
	}
	if input.Namespace != nil {
		stack.Namespace = *input.Namespace
	}
	if input.Config != nil {
		stack.Config = *input.Config
	}
	if input.Tools != nil {
		stack.Tools = input.Tools
	}
	stack.UpdatedAt = time.Now()

	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		return nil, fmt.Errorf("update stack: %w", err)
	}
	return &UpdateStackOutput{Stack: stack}, nil
}
