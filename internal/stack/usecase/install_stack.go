package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

var ErrDeploymentCancelled = errors.New("deployment cancelled")

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
		{name: "installing_postgresql", phase: "A", duration: time.Second},
		{name: "installing_minio", phase: "A", duration: time.Second},
		{name: "installing_object_storage_secret", phase: "A", duration: time.Second},
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
		{name: "installing_logging", phase: "C", duration: time.Second},
		{name: "installing_log_search", phase: "C", duration: time.Second},
		{name: "installing_opentelemetry", phase: "C", duration: time.Second},
		{name: "installing_gateway", phase: "C", duration: time.Second},
		{name: "integration_check", phase: "C", duration: time.Second},
	},
}

type InstallStack struct {
	stackRepo           port.StackRepository
	streamer            port.LogStreamer
	executor            port.StepExecutor
	kubeconfigProvider  port.KubeconfigProvider
	dynamicExecutorFunc func(kubeconfig []byte) port.StepExecutor
}

type stackConfigAwareExecutor interface {
	SetStackConfig(config domain.StackConfig)
}

type namespaceAwareExecutor interface {
	SetNamespace(namespace string)
}

type deploymentVerifiableExecutor interface {
	VerifyDeployment(ctx context.Context, stackID string) error
}

type InstallStackOption func(*InstallStack)

func WithExecutor(executor port.StepExecutor) InstallStackOption {
	return func(uc *InstallStack) {
		uc.executor = executor
	}
}

func WithKubeconfigProvider(provider port.KubeconfigProvider) InstallStackOption {
	return func(uc *InstallStack) {
		uc.kubeconfigProvider = provider
	}
}

func WithExecutorFactory(factory func(kubeconfig []byte) port.StepExecutor) InstallStackOption {
	return func(uc *InstallStack) {
		uc.dynamicExecutorFunc = factory
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

	executor := uc.resolveExecutor(ctx, stack)
	uc.configureExecutorForStack(stack, executor)

	if err := stack.TransitionTo(domain.StateValidating); err != nil {
		return fmt.Errorf("transition to validating: %w", err)
	}
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		return fmt.Errorf("update stack state: %w", err)
	}

	// Run the full installation pipeline asynchronously.
	go uc.run(context.WithoutCancel(ctx), stack, executor)

	return nil
}

func (uc *InstallStack) configureExecutorForStack(stack *domain.Stack, executor port.StepExecutor) {
	if stack == nil || executor == nil {
		return
	}

	if awareExecutor, ok := executor.(namespaceAwareExecutor); ok {
		namespace := stack.Namespace
		if namespace == "" {
			namespace = "nullus"
		}
		awareExecutor.SetNamespace(namespace)
	}

	awareExecutor, ok := executor.(stackConfigAwareExecutor)
	if !ok {
		return
	}

	cfg, ok := stackConfigFromInterface(stack.Config)
	if !ok {
		return
	}

	awareExecutor.SetStackConfig(cfg)
}

func stackConfigFromInterface(rawConfig any) (domain.StackConfig, bool) {
	if rawConfig == nil {
		return domain.StackConfig{}, false
	}

	switch cfg := rawConfig.(type) {
	case domain.StackConfig:
		return cfg, true
	case *domain.StackConfig:
		if cfg == nil {
			return domain.StackConfig{}, false
		}
		return *cfg, true
	default:
		payload, err := json.Marshal(rawConfig)
		if err != nil {
			return domain.StackConfig{}, false
		}

		var decoded domain.StackConfig
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return domain.StackConfig{}, false
		}
		return decoded, true
	}
}

// run executes the full installation pipeline, performing state transitions and
// emitting log entries. On any failure it initiates rollback.
func (uc *InstallStack) run(ctx context.Context, stack *domain.Stack, executor port.StepExecutor) {
	deploymentID := stack.ID

	uc.emit(ctx, deploymentID, "info", "validate", "A", "validation complete")

	// Transition: Validating → Installing
	if err := uc.transition(ctx, stack, domain.StateInstalling); err != nil {
		uc.handleFailure(ctx, stack, err)
		return
	}

	// Execute installation phases A, B, C.
	if err := uc.runPhases(ctx, stack, executor); err != nil {
		if errors.Is(err, ErrDeploymentCancelled) {
			slog.Info("installation stopped due to cancellation", "stack_id", stack.ID, "reason", err)
			return
		}
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
	if err := uc.verifyDeployment(ctx, stack, executor); err != nil {
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

func (uc *InstallStack) verifyDeployment(ctx context.Context, stack *domain.Stack, executor port.StepExecutor) error {
	verifier, ok := executor.(deploymentVerifiableExecutor)
	if !ok {
		uc.emit(ctx, stack.ID, "warn", "health_check", "C", "executor does not support deep verification, skipping runtime readiness checks")
		return nil
	}

	uc.emit(ctx, stack.ID, "info", "health_check", "C", "running runtime readiness checks")
	if err := verifier.VerifyDeployment(ctx, stack.ID); err != nil {
		return fmt.Errorf("runtime readiness check failed: %w", err)
	}

	return nil
}

func (uc *InstallStack) runPhases(ctx context.Context, stack *domain.Stack, executor port.StepExecutor) error {
	for _, phase := range installPhases {
		for _, step := range phase {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := uc.ensureDeploymentActive(ctx, stack.ID); err != nil {
				return err
			}

			uc.emit(ctx, stack.ID, "info", step.name, step.phase,
				fmt.Sprintf("starting %s", step.name))

			if err := uc.executeStep(ctx, stack.ID, step, executor); err != nil {
				return fmt.Errorf("step %s: %w", step.name, err)
			}
			if err := uc.ensureDeploymentActive(ctx, stack.ID); err != nil {
				return err
			}

			uc.emit(ctx, stack.ID, "info", step.name, step.phase,
				fmt.Sprintf("%s completed", step.name))
		}
	}
	return nil
}

func (uc *InstallStack) ensureDeploymentActive(ctx context.Context, stackID string) error {
	if uc.stackRepo == nil || strings.TrimSpace(stackID) == "" {
		return nil
	}

	current, err := uc.stackRepo.FindByID(ctx, stackID)
	if err != nil {
		if isStackNotFoundError(err) {
			return fmt.Errorf("%w: stack deleted during deployment", ErrDeploymentCancelled)
		}
		return fmt.Errorf("check stack deployment state: %w", err)
	}
	if current == nil {
		return fmt.Errorf("%w: stack deleted during deployment", ErrDeploymentCancelled)
	}

	if current.State == domain.StateCancelled {
		return fmt.Errorf("%w: stack marked cancelled", ErrDeploymentCancelled)
	}
	if current.State == domain.StateRollingBack || current.State == domain.StateRolledBack || current.State == domain.StateFailed {
		return fmt.Errorf("%w: stack state is %s", ErrDeploymentCancelled, current.State)
	}

	return nil
}

func (uc *InstallStack) executeStep(ctx context.Context, stackID string, step installStep, executor port.StepExecutor) error {
	if executor != nil {
		return executor.ExecuteStep(ctx, stackID, step.name, step.phase)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(step.duration):
		return nil
	}
}

func (uc *InstallStack) resolveExecutor(ctx context.Context, stack *domain.Stack) port.StepExecutor {
	if uc.kubeconfigProvider == nil || uc.dynamicExecutorFunc == nil {
		return uc.executor
	}

	kubeconfig, err := uc.kubeconfigProvider.GetKubeconfig(ctx, stack.ClusterID)
	if err != nil {
		slog.Warn("failed to load kubeconfig for stack deployment", "stack_id", stack.ID, "cluster_id", stack.ClusterID, "error", err)
		return uc.executor
	}
	if len(kubeconfig) == 0 {
		return uc.executor
	}

	dynamic := uc.dynamicExecutorFunc(kubeconfig)
	if dynamic != nil {
		return dynamic
	}
	return uc.executor
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
