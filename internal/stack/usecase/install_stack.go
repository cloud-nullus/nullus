package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// installStep describes a single simulated installation step.
type installStep struct {
	name     string
	phase    string
	duration time.Duration
}

// installPhases defines the ordered phases A→B→C and the steps within each.
var installPhases = [][]installStep{
	// Phase A
	{
		{name: "installing_cert_manager", phase: "A", duration: time.Second},
		{name: "installing_minio", phase: "A", duration: time.Second},
	},
	// Phase B
	{
		{name: "installing_gitlab", phase: "B", duration: 2 * time.Second},
		{name: "installing_argocd", phase: "B", duration: time.Second},
		{name: "installing_runner", phase: "B", duration: time.Second},
	},
	// Phase C
	{
		{name: "installing_prometheus", phase: "C", duration: time.Second},
		{name: "installing_grafana", phase: "C", duration: time.Second},
		{name: "integration_check", phase: "C", duration: time.Second},
	},
}

type InstallStack struct {
	stackRepo port.StackRepository
	streamer  port.LogStreamer
	executor  port.StepExecutor
}

type InstallStackOption func(*InstallStack)

func WithExecutor(executor port.StepExecutor) InstallStackOption {
	return func(uc *InstallStack) {
		uc.executor = executor
	}
}

func NewInstallStack(stackRepo port.StackRepository, streamer port.LogStreamer, opts ...InstallStackOption) *InstallStack {
	uc := &InstallStack{
		stackRepo: stackRepo,
		streamer:  streamer,
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// InstallStackInput holds the parameters for starting an installation.
type InstallStackInput struct {
	StackID string
}

// Execute starts the installation in a goroutine and returns immediately.
// The caller can track progress by subscribing to the LogStreamer.
func (uc *InstallStack) Execute(ctx context.Context, input InstallStackInput) error {
	stack, err := uc.stackRepo.GetByID(ctx, input.StackID)
	if err != nil {
		return fmt.Errorf("get stack: %w", err)
	}

	if err := stack.TransitionTo(domain.StateValidating); err != nil {
		return fmt.Errorf("transition to validating: %w", err)
	}
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		return fmt.Errorf("update stack state: %w", err)
	}

	// Run the full installation pipeline asynchronously.
	go uc.run(context.WithoutCancel(ctx), stack)

	return nil
}

// run executes the full installation pipeline, performing state transitions and
// emitting log entries. On any failure it initiates rollback.
func (uc *InstallStack) run(ctx context.Context, stack *domain.Stack) {
	deploymentID := stack.ID

	uc.emit(ctx, deploymentID, "info", "validate", "A", "validation complete")

	// Transition: Validating → Installing
	if err := uc.transition(ctx, stack, domain.StateInstalling); err != nil {
		uc.handleFailure(ctx, stack, err)
		return
	}

	// Execute installation phases A, B, C.
	if err := uc.runPhases(ctx, stack); err != nil {
		uc.handleFailure(ctx, stack, err)
		return
	}

	// Transition: Installing → Configuring
	if err := uc.transition(ctx, stack, domain.StateConfiguring); err != nil {
		uc.handleFailure(ctx, stack, err)
		return
	}
	uc.emit(ctx, deploymentID, "info", "configuring", "C", "post-install configuration applied")

	// Transition: Configuring → HealthCheck
	if err := uc.transition(ctx, stack, domain.StateHealthCheck); err != nil {
		uc.handleFailure(ctx, stack, err)
		return
	}
	uc.emit(ctx, deploymentID, "info", "health_check", "C", "all health checks passed")

	// Transition: HealthCheck → Completed
	if err := uc.transition(ctx, stack, domain.StateCompleted); err != nil {
		uc.handleFailure(ctx, stack, err)
		return
	}
	uc.emit(ctx, deploymentID, "info", "completed", "C", "installation completed successfully")
}

func (uc *InstallStack) runPhases(ctx context.Context, stack *domain.Stack) error {
	for _, phase := range installPhases {
		for _, step := range phase {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			uc.emit(ctx, stack.ID, "info", step.name, step.phase,
				fmt.Sprintf("starting %s", step.name))

			if err := uc.executeStep(ctx, stack.ID, step); err != nil {
				return fmt.Errorf("step %s: %w", step.name, err)
			}

			uc.emit(ctx, stack.ID, "info", step.name, step.phase,
				fmt.Sprintf("%s completed", step.name))
		}
	}
	return nil
}

func (uc *InstallStack) executeStep(ctx context.Context, stackID string, step installStep) error {
	if uc.executor != nil {
		return uc.executor.ExecuteStep(ctx, stackID, step.name, step.phase)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(step.duration):
		return nil
	}
}

// handleFailure transitions to Failed and attempts rollback.
func (uc *InstallStack) handleFailure(ctx context.Context, stack *domain.Stack, cause error) {
	slog.Error("installation failed", "stack_id", stack.ID, "error", cause)
	uc.emit(ctx, stack.ID, "error", "failed", "", fmt.Sprintf("installation failed: %s", cause))

	if err := uc.transition(ctx, stack, domain.StateFailed); err != nil {
		slog.Error("failed to transition to failed state", "stack_id", stack.ID, "error", err)
		return
	}

	uc.emit(ctx, stack.ID, "warn", "rolling_back", "", "initiating rollback")

	if err := uc.transition(ctx, stack, domain.StateRollingBack); err != nil {
		slog.Error("failed to transition to rolling_back", "stack_id", stack.ID, "error", err)
		return
	}

	// Simulate rollback work.
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
	}

	if err := uc.transition(ctx, stack, domain.StateRolledBack); err != nil {
		slog.Error("failed to transition to rolled_back", "stack_id", stack.ID, "error", err)
		return
	}
	uc.emit(ctx, stack.ID, "info", "rolled_back", "", "rollback completed")
}

// transition updates the stack state machine and persists the new state.
func (uc *InstallStack) transition(ctx context.Context, stack *domain.Stack, next domain.DeploymentState) error {
	if err := stack.TransitionTo(next); err != nil {
		return err
	}
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		return fmt.Errorf("persist state %s: %w", next, err)
	}
	return nil
}

// emit sends a log entry to the streamer.
func (uc *InstallStack) emit(ctx context.Context, deploymentID, level, step, phase, message string) {
	uc.streamer.Stream(ctx, deploymentID, port.LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Step:      step,
		Message:   message,
		Phase:     phase,
	})
}
