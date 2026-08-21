package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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
	// ServiceMonitor / Probe 를 만드는 단계들이 kube-prometheus-stack 설치보다
	// 앞서므로 그 CRD 를 먼저 깐다. 없으면 MinIO 설치가
	// "no matches for kind Probe" 로 죽는다.
	{name: "installing_prometheus_crds", phase: "A", duration: time.Second, deps: []string{"installing_cert_manager"}},
	{name: "installing_metrics_server", phase: "A", duration: time.Second},

	// 시크릿 평면(OpenBao → ESO → provisioning)이 스토리지보다 먼저 선다.
	// PostgreSQL/MinIO 차트는 비밀번호를 values 로 받지 않고 여기서 만든
	// Secret 을 existingSecret 으로 참조하므로, 순서가 뒤바뀌면 파드가
	// FailedMount 로 영원히 기동하지 못한다.
	//
	// OpenBao 는 file 스토리지 백엔드를 쓰므로 PostgreSQL/MinIO 에 의존하지
	// 않는다 — 그래서 스토리지 앞으로 옮길 수 있다.
	// 평면 내부 순서는 강제된다: init 이 만든 root token 으로 부트스트랩이
	// 인증을 구성하고, 그 role 이 있어야 ESO 가 로그인한다.
	{name: "installing_openbao", phase: "A", duration: time.Second, deps: []string{"installing_cert_manager", "installing_metrics_server"}},
	{name: "installing_external_secrets", phase: "A", duration: time.Second, deps: []string{"installing_openbao"}},
	{name: "provisioning_secrets", phase: "A", duration: time.Second, deps: []string{"installing_external_secrets"}},

	{name: "installing_postgresql", phase: "A", duration: time.Second, deps: []string{"provisioning_secrets"}},
	{name: "installing_minio", phase: "A", duration: time.Second, deps: []string{"installing_postgresql"}},
	{name: "installing_object_storage_secret", phase: "A", duration: time.Second, deps: []string{"installing_minio"}},
	{name: "installing_object_storage_buckets", phase: "A", duration: time.Second, deps: []string{"installing_object_storage_secret"}},
	{name: "installing_database_connection_check", phase: "A", duration: time.Second, deps: []string{"installing_object_storage_secret"}},

	// SSO 프로비저닝은 OIDC 클라이언트를 만들어 두는 단계라 이를 소비하는
	// GitLab/Argo CD/Grafana 보다 앞서야 한다. orderedStep 에만 있고 여기에
	// 없으면 단계가 영원히 실행되지 않는다.
	{name: "provisioning_sso", phase: "A", duration: time.Second, deps: []string{"provisioning_secrets", "installing_object_storage_secret", "installing_object_storage_buckets", "installing_database_connection_check"}},

	{name: "installing_gitlab", phase: "B", duration: 2 * time.Second, deps: []string{"provisioning_sso"}},

	// Gitea 는 GitLab 과 같은 슬롯(소스 저장소)의 다른 선택지다. 둘은 술어가
	// 배타적이라 동시에 서지 않는다.
	{name: "installing_gitea", phase: "B", duration: time.Second, deps: []string{"provisioning_sso"}},
	// Gitea 는 OAuth 소스를 Helm values 로 받지 않는다. CLI 로만 등록할 수 있고
	// CLI 는 app.ini 로 DB 를 찾으므로 기동된 뒤에 파드에 exec 한다.
	{name: "provisioning_gitea", phase: "B", duration: time.Second, deps: []string{"installing_gitea"}},

	// 독립 레지스트리(Harbor / Nexus)는 Argo CD 앞에 선다. Argo CD 가 배포할
	// 이미지를 여기서 받으므로 먼저 서 있어야 한다.
	//
	// 설치만 한 Nexus 는 Docker 커넥터도 저장소도 없어 CI 가 이미지를 올릴 곳이
	// 없다. provisioning_nexus 가 커넥터·저장소·관리자 비밀번호를 맞춘다.
	{name: "installing_harbor", phase: "B", duration: 2 * time.Second, deps: []string{"provisioning_sso"}},
	// 설치만 한 Harbor 에는 프로젝트가 없어 CI 가 이미지를 올릴 곳이 없다.
	{name: "provisioning_harbor", phase: "B", duration: time.Second, deps: []string{"installing_harbor"}},
	{name: "installing_nexus", phase: "B", duration: 2 * time.Second, deps: []string{"provisioning_sso"}},
	{name: "provisioning_nexus", phase: "B", duration: time.Second, deps: []string{"installing_nexus"}},

	{name: "installing_argocd", phase: "B", duration: time.Second, deps: []string{"provisioning_sso"}},
	{name: "installing_runner", phase: "B", duration: time.Second, deps: []string{"provisioning_sso", "installing_gitlab"}},

	// Jenkins 는 CI 슬롯의 다른 선택지다. GitLab 러너와 달리 소스 저장소에
	// 의존하지 않는다 — job 등록은 설치가 아니라 파이프라인 생성 시점이다.
	{name: "installing_jenkins", phase: "B", duration: time.Second, deps: []string{"provisioning_sso"}},

	{name: "installing_prometheus", phase: "C", duration: time.Second, deps: []string{"installing_argocd"}},
	{name: "installing_grafana", phase: "C", duration: time.Second, deps: []string{"installing_prometheus"}},
	{name: "installing_logging", phase: "C", duration: time.Second, deps: []string{"installing_argocd"}},
	{name: "installing_log_search", phase: "C", duration: time.Second, deps: []string{"installing_logging"}},
	{name: "installing_opentelemetry", phase: "C", duration: time.Second, deps: []string{"installing_logging"}},
	// 수집기는 자기가 내보낼 백엔드가 모두 선 뒤에 온다. 추적 저장소·메트릭·로그
	// 어느 쪽이든 아직 없으면 수집기가 뜨자마자 내보내기 실패를 쌓는다.
	{name: "installing_otel_collector", phase: "C", duration: time.Second, deps: []string{"installing_opentelemetry", "installing_prometheus"}},
	// 에이전트는 자기가 로그를 넘길 게이트웨이가 선 뒤에 온다.
	{name: "installing_otel_agent", phase: "C", duration: time.Second, deps: []string{"installing_otel_collector"}},
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

// secretScopeAwareExecutor 는 OpenBao 경로 접두사에 필요한 스코프를 받는다.
type secretScopeAwareExecutor interface {
	SetSecretScope(env, orgID string)
}

type resumeAwareExecutor interface {
	ResumeFromStep(stackID, step string)
}

// preflightExecutor 는 설치를 시작하기 전에 대상 네임스페이스를 검사할 수 있는
// 실행기다. 구현하지 않은 실행기(단위 테스트의 가짜 등)는 검사를 건너뛴다.
type preflightExecutor interface {
	PreflightNamespace(ctx context.Context, namespace string) error
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
	// SourceControl 은 외부 SCM 자격증명이다. 요청에서만 흐르고
	// stacks.config 에는 저장되지 않는다 — 설치가 끝나면 OpenBao 로 옮겨진다.
	SourceControl SourceControlCredentials
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

	if scopeAware, ok := executor.(secretScopeAwareExecutor); ok {
		env := strings.TrimSpace(uc.tokenRegistryEnv)
		if env == "" {
			env = "dev"
		}
		scopeAware.SetSecretScope(env, stack.OrgID)
	}
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

	// 설치가 도는 동안 살아 있음을 알린다. 없으면 오래 걸리는 단계 하나가
	// 멀쩡한 설치를 끊긴 것으로 만든다.
	stopHeartbeat := uc.startHeartbeat(ctx, stack.ID, installHeartbeatInterval)
	defer stopHeartbeat()

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

	// 이전 설치의 볼륨이 남아 있으면 여기서 멈춘다.
	//
	// 그대로 두면 새 설치가 옛 데이터베이스를 물려받고, 그 안의 비밀번호는 이번에
	// 새로 만든 Secret 과 다르다. 그 사실은 스무 단계쯤 뒤에 엉뚱한 도구의 오류로
	// 드러난다 — 실제로 Gitea 의 28P01 과 Harbor 의 401 로 두 번 나왔고, 매번
	// 20분을 태운 뒤였다.
	//
	// 이어서 진행(continue)하는 경우는 검사하지 않는다. 그때 남아 있는 볼륨은
	// 지금 하고 있는 설치가 만든 것이라 지우면 안 된다.
	if !input.Continue {
		if preflight, ok := executor.(preflightExecutor); ok {
			if err := preflight.PreflightNamespace(ctx, stack.Namespace); err != nil {
				uc.handleFailure(ctx, stack, executor, err)
				return
			}
		}
	}

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
	if err := uc.registerStackTokenSources(ctx, stack, input.SourceControl); err != nil {
		slog.Warn("token source registration failed", "stack_id", stack.ID, "error", err)
		// 사용자가 마법사에 직접 넣은 값이라 조용히 흘리면 안 된다. 이 등록이
		// 실패하면 파이프라인 생성이 "등록된 PAT 가 없다" 로 죽는데, 그 시점에는
		// 설치 로그를 다시 보지 않아 원인을 찾기 어렵다.
		if strings.TrimSpace(input.SourceControl.PersonalAccessToken) != "" {
			uc.emit(ctx, deploymentID, "warn", "completed", "C",
				fmt.Sprintf("GitHub 자격증명 등록에 실패했습니다 — 파이프라인 생성 전에 다시 등록해야 합니다: %v", err))
		}
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

func (uc *InstallStack) registerStackTokenSources(
	ctx context.Context,
	stack *domain.Stack,
	creds SourceControlCredentials,
) error {
	if uc.tokenRegistry == nil || stack == nil {
		return nil
	}
	for _, input := range BuildStackTokenSourceInputs(stack, uc.tokenRegistryEnv, creds) {
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
	currentExecutor := executor
	resumeStarted := resumeFromStep == "" || resumeFromStep == "validate"
	if resumeFromStep == "health_check" || resumeFromStep == "configuring" {
		resumeStarted = false
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

			if checker, ok := currentExecutor.(stepEnabledChecker); ok && !checker.IsStepEnabled(step.name) {
				if err := uc.executeStep(ctx, stack.ID, step, currentExecutor); err != nil {
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
			if tailer, ok := currentExecutor.(stepRuntimeTailer); ok {
				stopTail = tailer.StartStepRuntimeTail(ctx, stack.ID, step.name, func(level, message string) {
					normalized := strings.TrimSpace(strings.ToLower(level))
					if normalized != "warn" && normalized != "error" {
						normalized = "info"
					}
					uc.emit(ctx, stack.ID, normalized, step.name, step.phase, message)
				})
			}

			if err := uc.executeStep(ctx, stack.ID, step, currentExecutor); err != nil {
				if step.name == "installing_cert_manager" {
					refreshedExecutor := uc.resolveExecutor(ctx, stack)
					if refreshedExecutor != nil && refreshedExecutor != currentExecutor {
						uc.configureExecutorForStack(stack, refreshedExecutor)
						uc.emit(ctx, stack.ID, "warn", step.name, step.phase, "cert-manager preflight failed; retrying once with refreshed cluster connection")
						currentExecutor = refreshedExecutor
						if retryErr := uc.executeStep(ctx, stack.ID, step, currentExecutor); retryErr == nil {
							if stopTail != nil {
								stopTail()
								stopTail = nil
							}
							goto stepSucceeded
						}
					}
				}
				if stopTail != nil {
					stopTail()
				}
				uc.markStepFailed(ctx, stack, step.name, err)
				return fmt.Errorf("step %s: %w", step.name, err)
			}
		stepSucceeded:
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

			if reporter, ok := currentExecutor.(stepRuntimeReporter); ok {
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
	if cause != nil {
		message := strings.ToLower(cause.Error())
		if strings.Contains(message, "installing_cert_manager") &&
			(strings.Contains(message, "connect: connection refused") || strings.Contains(message, "cluster unreachable") || strings.Contains(message, "no such host")) {
			cause = fmt.Errorf("%w (possible stale cluster endpoint/kubeconfig; try cluster verify or refresh-discovery)", cause)
		}
	}
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

// installHeartbeatInterval 은 설치가 살아 있음을 알리는 주기다.
//
// domain.StaleInstallThreshold 보다 충분히 짧아야 한다 — 몇 번 걸러도 끊긴
// 것으로 오인되지 않을 만큼.
const installHeartbeatInterval = 2 * time.Minute

// stackToucher 는 갱신 시각만 찍는 저장소다.
type stackToucher interface {
	TouchUpdatedAt(ctx context.Context, stackID string) error
}

// startHeartbeat 는 설치가 도는 동안 스택의 갱신 시각을 주기적으로 찍는다.
// 돌려주는 함수를 부르면 멈춘다(여러 번 불러도 안전하다).
//
// 끊긴 설치를 찾는 리퍼는 갱신 시각만 본다. 그런데 그 시각은 단계가 시작·완료될
// 때만 움직인다 — 한 단계가 임계값보다 오래 걸리면(Harbor·GitLab 이미지 풀은
// 흔히 그렇다) 멀쩡히 도는 설치가 끊긴 것으로 표시된다. 2026-08-21 운영에서
// 그렇게 됐다: 상태만 실패로 뒤집히고 고루틴은 계속 돌아, "실패" 로 표시된 뒤에
// 게이트웨이가 만들어졌다. 오류 로그가 없던 것은 실제로 오류가 없었기 때문이다.
//
// 하트비트가 있으면 "갱신 시각이 멈췄다" 가 "설치를 돌리던 것이 사라졌다" 를
// 뜻하게 된다 — 리퍼가 원래 판정하려던 그것이다.
func (uc *InstallStack) startHeartbeat(ctx context.Context, stackID string, interval time.Duration) func() {
	toucher, ok := uc.stackRepo.(stackToucher)
	if !ok || strings.TrimSpace(stackID) == "" || interval <= 0 {
		return func() {}
	}

	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := toucher.TouchUpdatedAt(ctx, stackID); err != nil {
					slog.Warn("install heartbeat failed", "stack_id", stackID, "error", err)
				}
			}
		}
	}()

	return func() { once.Do(func() { close(done) }) }
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
	for _, item := range installDAG {
		if item.name == step {
			return true
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
		{hint: domain.PostgresServiceName, step: "installing_postgresql"},
		{hint: "postgresql", step: "installing_postgresql"},
		{hint: domain.MinIOServiceName, step: "installing_minio"},
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
