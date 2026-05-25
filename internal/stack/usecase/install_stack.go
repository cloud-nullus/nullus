package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

var ErrDeploymentCancelled = errors.New("deployment canceled")

// installStep describes a single simulated installation step.
type installStep struct {
	name     string
	phase    string
	duration time.Duration
	deps     []string
}

// installDAG defines step dependencies with phase labels.
var installDAG = []installStep{
	{name: "installing_cert_manager", phase: "A", duration: time.Second},
	{name: "installing_metrics_server", phase: "A", duration: time.Second},
	{name: "installing_postgresql", phase: "A", duration: time.Second, deps: []string{"installing_cert_manager", "installing_metrics_server"}},
	{name: "installing_minio", phase: "A", duration: time.Second, deps: []string{"installing_postgresql"}},
	{name: "installing_object_storage_secret", phase: "A", duration: time.Second, deps: []string{"installing_minio"}},

	{name: "installing_openbao", phase: "B", duration: time.Second, deps: []string{"installing_postgresql", "installing_minio"}},
	{name: "installing_gitlab", phase: "B", duration: 2 * time.Second, deps: []string{"installing_openbao", "installing_object_storage_secret"}},
	{name: "installing_argocd", phase: "B", duration: time.Second, deps: []string{"installing_openbao"}},
	{name: "installing_runner", phase: "B", duration: time.Second, deps: []string{"installing_openbao", "installing_gitlab"}},

	{name: "installing_prometheus", phase: "C", duration: time.Second, deps: []string{"installing_argocd"}},
	{name: "installing_grafana", phase: "C", duration: time.Second, deps: []string{"installing_prometheus"}},
	{name: "installing_logging", phase: "C", duration: time.Second, deps: []string{"installing_argocd"}},
	{name: "installing_log_search", phase: "C", duration: time.Second, deps: []string{"installing_logging"}},
	{name: "installing_opentelemetry", phase: "C", duration: time.Second, deps: []string{"installing_logging"}},
	{name: "installing_gateway", phase: "C", duration: time.Second, deps: []string{"installing_argocd"}},
	{name: "integration_check", phase: "C", duration: time.Second, deps: []string{"installing_gateway"}},
}

type InstallStack struct {
	stackRepo           port.StackRepository
	streamer            port.LogStreamer
	executor            port.StepExecutor
	kubeconfigProvider  port.KubeconfigProvider
	dynamicExecutorFunc func(kubeconfig []byte) port.StepExecutor
	tokenRegistry       port.TokenSourceRegistry
	tokenRegistryEnv    string
	secretRouter        *secrets.Router
}

type stackConfigAwareExecutor interface {
	SetStackConfig(config domain.StackConfig)
}

type namespaceAwareExecutor interface {
	SetNamespace(namespace string)
}

type resumeAwareExecutor interface {
	ResumeFromStep(stackID, step string)
}

type deploymentVerifiableExecutor interface {
	VerifyDeployment(ctx context.Context, stackID string) error
}

type deploymentRollbackExecutor interface {
	RollbackDeployment(ctx context.Context, stackID string) error
}

type stepRuntimeReporter interface {
	StepRuntimeLogs(ctx context.Context, stackID, step string) (infos []string, warns []string)
}

type stepRuntimeTailer interface {
	StartStepRuntimeTail(ctx context.Context, stackID, step string, emit func(level, message string)) (stop func())
}

type stepEnabledChecker interface {
	IsStepEnabled(step string) bool
}

type deploymentLogResetter interface {
	ClearHistory(deploymentID string)
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

func WithTokenSourceRegistry(registry port.TokenSourceRegistry, env string) InstallStackOption {
	return func(uc *InstallStack) {
		uc.tokenRegistry = registry
		uc.tokenRegistryEnv = strings.TrimSpace(env)
	}
}

func WithSecretRouter(router *secrets.Router) InstallStackOption {
	return func(uc *InstallStack) {
		uc.secretRouter = router
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
	StackID        string
	Continue       bool
	PreserveLogs   bool
	ResumeFromStep string
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

	if input.Continue && input.ResumeFromStep == "" {
		input.ResumeFromStep = firstNonEmpty(stack.LastFailedStep, stack.CurrentStep)
	}
	if input.Continue && !isKnownResumeStep(input.ResumeFromStep) {
		input.ResumeFromStep = ""
	}
	if input.Continue && input.ResumeFromStep != "" {
		if resumable, ok := executor.(resumeAwareExecutor); ok {
			resumable.ResumeFromStep(stack.ID, input.ResumeFromStep)
		}
	}

	if input.Continue && stack.State == domain.StateFailed {
		if err := stack.TransitionTo(domain.StatePending); err != nil {
			return fmt.Errorf("transition failed stack to pending: %w", err)
		}
		if err := uc.stackRepo.Update(ctx, stack); err != nil {
			return fmt.Errorf("update stack state: %w", err)
		}
	}
	if !input.Continue {
		stack.CurrentStep = ""
		stack.LastCompletedStep = ""
		stack.LastFailedStep = ""
		stack.LastFailureReason = ""
	}

	if err := stack.TransitionTo(domain.StateValidating); err != nil {
		return fmt.Errorf("transition to validating: %w", err)
	}
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		return fmt.Errorf("update stack state: %w", err)
	}

	// Run the full installation pipeline asynchronously.
	go uc.run(context.WithoutCancel(ctx), stack, executor, input)

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
func (uc *InstallStack) run(ctx context.Context, stack *domain.Stack, executor port.StepExecutor, input InstallStackInput) {
	deploymentID := stack.ID

	if !input.PreserveLogs {
		if resetter, ok := uc.streamer.(deploymentLogResetter); ok {
			resetter.ClearHistory(deploymentID)
		}
	}
	if input.Continue {
		message := "continuing deployment after failure"
		if input.ResumeFromStep != "" {
			message = fmt.Sprintf("%s from %s", message, input.ResumeFromStep)
		}
		uc.emit(ctx, deploymentID, "info", "continue", "", message)
	}

	uc.markStepStarted(ctx, stack, "validate")
	uc.emit(ctx, deploymentID, "info", "validate", "A", "validation complete")
	uc.markStepCompleted(ctx, stack, "validate")

	// Transition: Validating → Installing
	if err := uc.transition(ctx, stack, domain.StateInstalling); err != nil {
		uc.handleFailure(ctx, stack, executor, err)
		return
	}

	// Execute installation phases A, B, C.
	if err := uc.runPhases(ctx, stack, executor, input.ResumeFromStep); err != nil {
		if errors.Is(err, ErrDeploymentCancelled) {
			slog.Info("installation stopped due to cancellation", "stack_id", stack.ID, "reason", err)
			return
		}
		uc.handleFailure(ctx, stack, executor, err)
		return
	}

	// Transition: Installing → Configuring
	if err := uc.transition(ctx, stack, domain.StateConfiguring); err != nil {
		uc.handleFailure(ctx, stack, executor, err)
		return
	}
	uc.markStepCompleted(ctx, stack, "configuring")
	uc.emit(ctx, deploymentID, "info", "configuring", "C", "post-install configuration applied")

	// Transition: Configuring → HealthCheck
	if err := uc.transition(ctx, stack, domain.StateHealthCheck); err != nil {
		uc.handleFailure(ctx, stack, executor, err)
		return
	}
	uc.markStepStarted(ctx, stack, "health_check")
	if err := uc.verifyDeployment(ctx, stack, executor); err != nil {
		uc.markStepFailed(ctx, stack, resumeStepForReadinessError(err), err)
		uc.handleFailure(ctx, stack, executor, err)
		return
	}
	uc.markStepCompleted(ctx, stack, "health_check")
	uc.emit(ctx, deploymentID, "info", "health_check", "C", "all health checks passed")

	// Transition: HealthCheck → Completed
	if err := uc.transition(ctx, stack, domain.StateCompleted); err != nil {
		uc.handleFailure(ctx, stack, executor, err)
		return
	}
	stack.CurrentStep = ""
	stack.LastFailedStep = ""
	stack.LastFailureReason = ""
	_ = uc.stackRepo.Update(ctx, stack)
	uc.emit(ctx, deploymentID, "info", "completed", "C", "installation completed successfully")
	if err := uc.registerStackTokenSources(ctx, stack); err != nil {
		slog.Warn("token source registration failed", "stack_id", stack.ID, "error", err)
	}
}

func (uc *InstallStack) runOpenBaoHealthGate(ctx context.Context, stack *domain.Stack, phase string) error {
	if stack == nil {
		return nil
	}
	cfg, ok := stackConfigFromInterface(stack.Config)
	if !ok || cfg.Authentication == nil {
		return nil
	}
	provider := strings.TrimSpace(strings.ToLower(cfg.Authentication.Provider))
	if provider != "openbao" {
		return nil
	}
	if uc.secretRouter == nil || !uc.secretRouter.Has(provider) {
		uc.emit(ctx, stack.ID, "warn", "installing_openbao", phase, "openbao provider is not configured in API router; proceeding with in-cluster health gate")
		return nil
	}
	if err := uc.secretRouter.Check(ctx, provider); err != nil {
		uc.emit(ctx, stack.ID, "warn", "installing_openbao", phase, fmt.Sprintf("openbao router health check failed (non-blocking): %v", err))
		return nil
	}
	uc.emit(ctx, stack.ID, "info", "installing_openbao", phase, "openbao health gate check passed")
	return nil
}

func (uc *InstallStack) registerStackTokenSources(ctx context.Context, stack *domain.Stack) error {
	if uc.tokenRegistry == nil || stack == nil {
		return nil
	}
	cfg, ok := stackConfigFromInterface(stack.Config)
	if !ok || cfg.Authentication == nil || strings.TrimSpace(strings.ToLower(cfg.Authentication.Provider)) != "openbao" {
		return nil
	}
	env := uc.tokenRegistryEnv
	if env == "" {
		env = "dev"
	}
	inputs := []port.TokenSourceInput{}
	appendTool := func(module, provider string) {
		provider = strings.TrimSpace(strings.ToLower(provider))
		if provider == "" {
			return
		}
		provider = strings.ReplaceAll(provider, " ", "-")
		inputs = append(inputs, port.TokenSourceInput{
			OrgID:         stack.OrgID,
			Module:        module,
			Provider:      provider,
			Path:          fmt.Sprintf("kv/nullus/%s/%s/%s/%s/token", env, stack.OrgID, module, provider),
			TokenType:     "reissue",
			Status:        "healthy",
			SecretManager: strings.TrimSpace(strings.ToLower(cfg.Authentication.Provider)),
			TokenValue:    "managed-by-nullus",
		})
	}

	appendTool("artifacts", cfg.Artifacts.SourceRepository.Name)
	appendTool("artifacts", cfg.Artifacts.ContainerRegistry.Name)
	appendTool("pipeline", cfg.Pipeline.CIPlatform.Name)
	appendTool("pipeline", cfg.Pipeline.CDTool.Name)

	namespace := strings.TrimSpace(stack.Namespace)
	if namespace == "" {
		namespace = "nullus"
	}

	appendBootstrap := func(module, provider, pathSuffix, value string) {
		provider = strings.TrimSpace(strings.ToLower(provider))
		if provider == "" || strings.TrimSpace(value) == "" {
			return
		}
		provider = strings.ReplaceAll(provider, " ", "-")
		inputs = append(inputs, port.TokenSourceInput{
			OrgID:         stack.OrgID,
			Module:        module,
			Provider:      provider,
			Path:          fmt.Sprintf("kv/nullus/%s/%s/%s/%s/%s", env, stack.OrgID, module, provider, pathSuffix),
			TokenType:     "bootstrap",
			Status:        "healthy",
			SecretManager: strings.TrimSpace(strings.ToLower(cfg.Authentication.Provider)),
			TokenValue:    value,
		})
	}

	if cfg.Storage != nil && strings.TrimSpace(strings.ToLower(cfg.Storage.Database.Mode)) == "create" {
		appendBootstrap("storage", "postgresql", "access", fmt.Sprintf("host=nullus-postgresql.%s.svc.cluster.local port=5432 db=gitlabhq_production username=gitlab password=nullus-gitlab-password", namespace)) // #nosec G101 -- default bootstrap credential, matches Helm default value
	}
	if cfg.Artifacts.StorageBackend.Enabled && strings.EqualFold(strings.TrimSpace(cfg.Artifacts.StorageBackend.Name), "minio") {
		appendBootstrap("artifacts", "minio", "access", fmt.Sprintf("endpoint=http://nullus-minio.%s.svc.cluster.local:9000 access_key=nullus-admin secret_key=nullus-minio-secret", namespace)) // #nosec G101 -- default bootstrap credential, matches Helm default value
	}
	cdTool := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(cfg.Pipeline.CDTool.Name)), " ", "-")
	if cfg.Pipeline.CDTool.Enabled && (cdTool == "argocd" || cdTool == "argo-cd") {
		appendBootstrap("pipeline", "argocd", "access", fmt.Sprintf("url=http://argo-cd-argocd-server.%s.svc.cluster.local username=admin password_secret=argocd-initial-admin-secret", namespace))
	}
	if cfg.Artifacts.SourceRepository.Enabled && (strings.EqualFold(strings.TrimSpace(cfg.Artifacts.SourceRepository.Name), "gitlab") || strings.EqualFold(strings.TrimSpace(cfg.Artifacts.SourceRepository.Name), "gitlab-ce")) {
		appendBootstrap("artifacts", "gitlab", "access", fmt.Sprintf("url=http://gitlab-webservice-default.%s.svc:8181 username=root password=nullus-gitlab-password", namespace)) // #nosec G101 -- default bootstrap credential, matches Helm default value
	}

	seen := map[string]struct{}{}
	for _, input := range inputs {
		key := input.OrgID + ":" + input.Module + ":" + input.Provider + ":" + input.Path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := uc.tokenRegistry.Upsert(ctx, input); err != nil {
			return err
		}
	}
	return nil
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

func (uc *InstallStack) runPhases(ctx context.Context, stack *domain.Stack, executor port.StepExecutor, resumeFromStep string) error {
	completed := map[string]bool{}
	processed := map[string]bool{}
	resumeStarted := resumeFromStep == "" || resumeFromStep == "validate"
	if resumeFromStep == "health_check" || resumeFromStep == "configuring" {
		resumeStarted = true
	}

	for len(processed) < len(installDAG) {
		progressed := false

		for _, step := range installDAG {
			if processed[step.name] {
				continue
			}

			depsDone := true
			for _, dep := range step.deps {
				if !completed[dep] {
					depsDone = false
					break
				}
			}
			if !depsDone {
				continue
			}

			if !resumeStarted {
				if step.name != resumeFromStep {
					uc.emit(ctx, stack.ID, "info", "resume_skip", step.phase,
						fmt.Sprintf("skipping previously completed %s", step.name))
					processed[step.name] = true
					completed[step.name] = true
					progressed = true
					continue
				}
				resumeStarted = true
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := uc.ensureDeploymentActive(ctx, stack.ID); err != nil {
				return err
			}

			if checker, ok := executor.(stepEnabledChecker); ok && !checker.IsStepEnabled(step.name) {
				if err := uc.executeStep(ctx, stack.ID, step, executor); err != nil {
					return fmt.Errorf("step %s: %w", step.name, err)
				}
				uc.emit(ctx, stack.ID, "info", "skipped", step.phase,
					fmt.Sprintf("skipped %s because it is not selected", step.name))
				processed[step.name] = true
				completed[step.name] = true
				progressed = true
				continue
			}

			uc.emit(ctx, stack.ID, "info", step.name, step.phase,
				fmt.Sprintf("starting %s", step.name))
			uc.markStepStarted(ctx, stack, step.name)

			var stopTail func()
			if tailer, ok := executor.(stepRuntimeTailer); ok {
				stopTail = tailer.StartStepRuntimeTail(ctx, stack.ID, step.name, func(level, message string) {
					normalized := strings.TrimSpace(strings.ToLower(level))
					if normalized != "warn" && normalized != "error" {
						normalized = "info"
					}
					uc.emit(ctx, stack.ID, normalized, step.name, step.phase, message)
				})
			}

			if err := uc.executeStep(ctx, stack.ID, step, executor); err != nil {
				if stopTail != nil {
					stopTail()
				}
				uc.markStepFailed(ctx, stack, step.name, err)
				return fmt.Errorf("step %s: %w", step.name, err)
			}
			if step.name == "installing_openbao" {
				if err := uc.runOpenBaoHealthGate(ctx, stack, step.phase); err != nil {
					if stopTail != nil {
						stopTail()
					}
					return err
				}
			}
			if stopTail != nil {
				stopTail()
			}
			if err := uc.ensureDeploymentActive(ctx, stack.ID); err != nil {
				return err
			}

			if reporter, ok := executor.(stepRuntimeReporter); ok {
				infos, warns := reporter.StepRuntimeLogs(ctx, stack.ID, step.name)
				for _, message := range infos {
					uc.emit(ctx, stack.ID, "info", step.name, step.phase, message)
				}
				for _, message := range warns {
					uc.emit(ctx, stack.ID, "warn", step.name, step.phase, message)
				}
			}

			uc.emit(ctx, stack.ID, "info", step.name, step.phase,
				fmt.Sprintf("%s completed", step.name))
			uc.markStepCompleted(ctx, stack, step.name)
			processed[step.name] = true
			completed[step.name] = true
			progressed = true
		}

		if !progressed {
			return fmt.Errorf("install DAG is blocked: unresolved dependencies or disabled prerequisite steps")
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
		return fmt.Errorf("%w: stack marked canceled", ErrDeploymentCancelled)
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
	slog.Warn("step executor is nil; running simulated install step",
		"stack_id", stackID,
		"step", step.name,
		"phase", step.phase,
		"duration", step.duration,
	)
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
func (uc *InstallStack) handleFailure(ctx context.Context, stack *domain.Stack, executor port.StepExecutor, cause error) {
	_ = executor
	slog.Error("installation failed", "stack_id", stack.ID, "error", cause)
	if stack.LastFailedStep == "" {
		uc.markStepFailed(ctx, stack, firstNonEmpty(stack.CurrentStep, "deployment"), cause)
	}
	uc.emit(ctx, stack.ID, "error", "failed", "", fmt.Sprintf("installation failed: %s", cause))

	if err := uc.transition(ctx, stack, domain.StateFailed); err != nil {
		slog.Error("failed to transition to failed state", "stack_id", stack.ID, "error", err)
		return
	}

	uc.emit(ctx, stack.ID, "warn", "failed", "", "deployment paused; fix the cause and press Continue to resume")
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

func (uc *InstallStack) markStepStarted(ctx context.Context, stack *domain.Stack, step string) {
	if stack == nil || uc.stackRepo == nil || strings.TrimSpace(step) == "" {
		return
	}
	stack.CurrentStep = step
	if stack.LastFailedStep == step {
		stack.LastFailedStep = ""
		stack.LastFailureReason = ""
	}
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		slog.Warn("failed to persist deployment step start", "stack_id", stack.ID, "step", step, "error", err)
	}
}

func (uc *InstallStack) markStepCompleted(ctx context.Context, stack *domain.Stack, step string) {
	if stack == nil || uc.stackRepo == nil || strings.TrimSpace(step) == "" {
		return
	}
	stack.CurrentStep = step
	stack.LastCompletedStep = step
	if stack.LastFailedStep == step {
		stack.LastFailedStep = ""
		stack.LastFailureReason = ""
	}
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		slog.Warn("failed to persist deployment step completion", "stack_id", stack.ID, "step", step, "error", err)
	}
}

func (uc *InstallStack) markStepFailed(ctx context.Context, stack *domain.Stack, step string, cause error) {
	if stack == nil || uc.stackRepo == nil || strings.TrimSpace(step) == "" {
		return
	}
	stack.CurrentStep = step
	stack.LastFailedStep = step
	if cause != nil {
		stack.LastFailureReason = cause.Error()
	}
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		slog.Warn("failed to persist deployment step failure", "stack_id", stack.ID, "step", step, "error", err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isKnownResumeStep(step string) bool {
	step = strings.TrimSpace(step)
	if step == "" || step == "validate" || step == "configuring" || step == "health_check" {
		return true
	}
	for _, phase := range installPhases {
		for _, item := range phase {
			if item.name == step {
				return true
			}
		}
	}
	return false
}

func resumeStepForReadinessError(err error) string {
	if err == nil {
		return "health_check"
	}
	message := strings.ToLower(err.Error())
	releaseSteps := []struct {
		hint string
		step string
	}{
		{hint: "gitlab-runner", step: "installing_runner"},
		{hint: "argo-cd", step: "installing_argocd"},
		{hint: "argocd", step: "installing_argocd"},
		{hint: "gitlab", step: "installing_gitlab"},
		{hint: "metrics-server", step: "installing_metrics_server"},
		{hint: "nullus-postgresql", step: "installing_postgresql"},
		{hint: "postgresql", step: "installing_postgresql"},
		{hint: "nullus-minio", step: "installing_minio"},
		{hint: "minio", step: "installing_minio"},
		{hint: "kube-prometheus-stack", step: "installing_prometheus"},
		{hint: "grafana", step: "installing_grafana"},
		{hint: "envoy", step: "installing_gateway"},
		{hint: "gateway", step: "installing_gateway"},
	}
	for _, item := range releaseSteps {
		if strings.Contains(message, " for "+item.hint+":") ||
			strings.Contains(message, " for "+item.hint+" ") ||
			strings.Contains(message, "release "+item.hint+" ") ||
			strings.Contains(message, "status check failed for "+item.hint) {
			return item.step
		}
	}
	return "health_check"
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
