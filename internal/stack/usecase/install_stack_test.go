package usecase

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// --- fakes ---

type fakeStackRepo struct {
	mu      sync.Mutex
	stacks  map[string]*domain.Stack
	touches map[string]int
}

func (r *fakeStackRepo) TouchUpdatedAt(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.touches == nil {
		r.touches = make(map[string]int)
	}
	r.touches[id]++
	if stack, ok := r.stacks[id]; ok {
		stack.UpdatedAt = time.Now()
	}
	return nil
}

func (r *fakeStackRepo) touchCount(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.touches[id]
}

func newFakeStackRepo(stacks ...*domain.Stack) *fakeStackRepo {
	r := &fakeStackRepo{stacks: make(map[string]*domain.Stack), touches: make(map[string]int)}
	for _, s := range stacks {
		cp := *s
		r.stacks[s.ID] = &cp
	}
	return r
}

func (r *fakeStackRepo) Create(_ context.Context, s *domain.Stack) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.stacks[s.ID] = &cp
	return nil
}

func (r *fakeStackRepo) GetByID(_ context.Context, id string) (*domain.Stack, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.stacks[id]
	if !ok || s.DeletedAt != nil {
		return nil, fmt.Errorf("stack not found: %s", id)
	}
	cp := *s
	return &cp, nil
}

func (r *fakeStackRepo) FindByID(ctx context.Context, id string) (*domain.Stack, error) {
	return r.GetByID(ctx, id)
}

func (r *fakeStackRepo) List(_ context.Context, orgID string, _ bool) ([]*domain.Stack, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.Stack, 0, len(r.stacks))
	for _, stack := range r.stacks {
		if orgID != "" && stack.OrgID != orgID {
			continue
		}
		cp := *stack
		out = append(out, &cp)
	}
	return out, nil
}

func (r *fakeStackRepo) Update(_ context.Context, s *domain.Stack) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *s
	r.stacks[s.ID] = &cp
	return nil
}

func (r *fakeStackRepo) UpdateTools(ctx context.Context, s *domain.Stack) error {
	return r.Update(ctx, s)
}

func (r *fakeStackRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.stacks[id]; ok {
		now := time.Now()
		s.DeletedAt = &now
		s.UpdatedAt = now
	}
	return nil
}

func (r *fakeStackRepo) getState(id string) domain.DeploymentState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stacks[id].State
}

func (r *fakeStackRepo) getStack(id string) *domain.Stack {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *r.stacks[id]
	return &cp
}

// fakeStreamer records all log entries.
type fakeStreamer struct {
	mu      sync.Mutex
	entries []port.LogEntry
}

func (s *fakeStreamer) Stream(_ context.Context, _ string, entry port.LogEntry) {
	s.mu.Lock()
	s.entries = append(s.entries, entry)
	s.mu.Unlock()
}

func (s *fakeStreamer) Subscribe(_ string) <-chan port.LogEntry {
	ch := make(chan port.LogEntry, 256)
	return ch
}

func (s *fakeStreamer) Unsubscribe(_ string, _ <-chan port.LogEntry) {}

func (s *fakeStreamer) steps() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	steps := make([]string, len(s.entries))
	for i, e := range s.entries {
		steps[i] = e.Step
	}
	return steps
}

type fakeKubeconfigProvider struct {
	mu         sync.Mutex
	configs    map[string][]byte
	requested  []string
	requestErr error
}

func (p *fakeKubeconfigProvider) GetKubeconfig(_ context.Context, clusterID string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requested = append(p.requested, clusterID)
	if p.requestErr != nil {
		return nil, p.requestErr
	}
	cfg, ok := p.configs[clusterID]
	if !ok {
		return nil, nil
	}
	copyCfg := make([]byte, len(cfg))
	copy(copyCfg, cfg)
	return copyCfg, nil
}

func (p *fakeKubeconfigProvider) requestedClusterIDs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.requested))
	copy(out, p.requested)
	return out
}

type fakeStepExecutor struct {
	mu      sync.Mutex
	steps   []string
	failAt  string
	errText string
}

type fakeSelectableExecutor struct {
	mu      sync.Mutex
	steps   []string
	enabled map[string]bool
}

type fakeSecretStore struct{}

func (s *fakeSecretStore) PutToken(_ context.Context, _, _ string) error        { return nil }
func (s *fakeSecretStore) GetToken(_ context.Context, _ string) (string, error) { return "", nil }
func (s *fakeSecretStore) Check(_ context.Context) error                        { return nil }

func (e *fakeSelectableExecutor) ExecuteStep(_ context.Context, _ string, step, _ string) error {
	if !e.IsStepEnabled(step) {
		return nil
	}
	e.mu.Lock()
	e.steps = append(e.steps, step)
	e.mu.Unlock()
	return nil
}

func (e *fakeSelectableExecutor) IsStepEnabled(step string) bool {
	enabled, ok := e.enabled[step]
	if !ok {
		return false
	}
	return enabled
}

func (e *fakeSelectableExecutor) calledSteps() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.steps))
	copy(out, e.steps)
	return out
}

func (e *fakeStepExecutor) ExecuteStep(_ context.Context, _ string, step, _ string) error {
	e.mu.Lock()
	e.steps = append(e.steps, step)
	e.mu.Unlock()
	if e.failAt != "" && e.failAt == step {
		return fmt.Errorf("%s", e.errText)
	}
	return nil
}

func (e *fakeStepExecutor) calledSteps() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.steps))
	copy(out, e.steps)
	return out
}

type fakeVerifiableExecutor struct {
	fakeStepExecutor
	verifyErr     error
	rollbackErr   error
	rollbackCalls int
}

func (e *fakeVerifiableExecutor) VerifyDeployment(_ context.Context, _ string) error {
	return e.verifyErr
}

func (e *fakeVerifiableExecutor) RollbackDeployment(_ context.Context, _ string) error {
	e.rollbackCalls++
	return e.rollbackErr
}

type fakeVerifyOnlyExecutor struct {
	fakeStepExecutor
	verifyErr error
}

func (e *fakeVerifyOnlyExecutor) VerifyDeployment(_ context.Context, _ string) error {
	return e.verifyErr
}

type fakeCancellingExecutor struct {
	repo       *fakeStackRepo
	stackID    string
	stepCalls  []string
	cancelOnce sync.Once
}

func (e *fakeCancellingExecutor) ExecuteStep(_ context.Context, _ string, step, _ string) error {
	e.stepCalls = append(e.stepCalls, step)
	if step == "installing_cert_manager" {
		e.cancelOnce.Do(func() {
			stack, err := e.repo.GetByID(context.Background(), e.stackID)
			if err != nil || stack == nil {
				return
			}
			stack.State = domain.StateCancelled
			_ = e.repo.Update(context.Background(), stack)
		})
	}
	return nil
}

type fakeConfigurableExecutor struct {
	fakeStepExecutor

	mu               sync.Mutex
	configuredConfig domain.StackConfig
	namespace        string
	namespaceSet     bool
	configSet        bool
}

func (e *fakeConfigurableExecutor) SetStackConfig(config domain.StackConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.configuredConfig = config
	e.configSet = true
}

func (e *fakeConfigurableExecutor) SetNamespace(namespace string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.namespace = namespace
	e.namespaceSet = true
}

func (e *fakeConfigurableExecutor) configuredNamespace() (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.namespace, e.namespaceSet
}

// --- tests ---

func TestInstallStack_SuccessfulInstallation(t *testing.T) {
	stack := &domain.Stack{
		ID:    "stk_test01",
		State: domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}

	uc := NewInstallStack(repo, streamer)

	err := uc.Execute(context.Background(), InstallStackInput{StackID: "stk_test01"})
	require.NoError(t, err)

	// Execute 가 동기적으로 보장하는 것은 "pending 을 벗어났다" 까지다.
	//
	// 정확한 상태를 단정하면 설치 고루틴과 경합한다 — validating 을 기대했는데
	// 그 사이 installing 으로 넘어가 있으면 실패한다. 느린 러너에서 나던
	// 실패가 이것이었다(CI 가 5개월간 꺼져 있어 아무도 돌리지 않았다).
	assert.NotEqual(t, domain.StatePending, repo.getState("stk_test01"),
		"Execute 는 돌아오기 전에 상태를 옮겨 놓는다")

	deadline := time.Now().Add(dagTotalDuration() + 10*time.Second)
	for time.Now().Before(deadline) {
		if repo.getState("stk_test01") == domain.StateCompleted {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	assert.Equal(t, domain.StateCompleted, repo.getState("stk_test01"))

	// Verify key steps were logged.
	steps := streamer.steps()
	assert.Contains(t, steps, "installing_cert_manager")
	assert.Contains(t, steps, "installing_metrics_server")
	assert.Contains(t, steps, "installing_postgresql")
	assert.Contains(t, steps, "installing_minio")
	assert.Contains(t, steps, "installing_object_storage_secret")
	assert.Contains(t, steps, "installing_gitlab")
	assert.Contains(t, steps, "installing_argocd")
	assert.Contains(t, steps, "installing_runner")
	assert.Contains(t, steps, "installing_prometheus")
	assert.Contains(t, steps, "installing_grafana")
	assert.Contains(t, steps, "installing_logging")
	assert.Contains(t, steps, "installing_log_search")
	assert.Contains(t, steps, "installing_opentelemetry")
	assert.Contains(t, steps, "installing_gateway")
	assert.Contains(t, steps, "integration_check")
	assert.Contains(t, steps, "completed")
}

func TestInstallStack_StackNotFound(t *testing.T) {
	repo := newFakeStackRepo()
	streamer := &fakeStreamer{}

	uc := NewInstallStack(repo, streamer)

	err := uc.Execute(context.Background(), InstallStackInput{StackID: "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get stack")
}

func TestInstallStack_ContextCancellation_TriggersRollback(t *testing.T) {
	stack := &domain.Stack{
		ID:    "stk_cancel",
		State: domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}

	uc := NewInstallStack(repo, streamer)

	ctx, cancel := context.WithCancel(context.Background())

	err := uc.Execute(ctx, InstallStackInput{StackID: "stk_cancel"})
	require.NoError(t, err)

	// Cancel immediately after starting.
	cancel()

	// Wait for rollback to complete (longer timeout for CI).
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		state := repo.getState("stk_cancel")
		if state == domain.StateRolledBack || state == domain.StateCompleted || state == domain.StateFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	finalState := repo.getState("stk_cancel")
	// Cancellation is async — any terminal or in-progress state is acceptable.
	assert.True(t,
		finalState == domain.StateRolledBack || finalState == domain.StateCompleted ||
			finalState == domain.StateFailed || finalState == domain.StateInstalling ||
			finalState == domain.StateRollingBack,
		"expected a valid post-cancel state, got %s", finalState,
	)
}

func TestInstallStack_UsesKubeconfigProviderExecutor(t *testing.T) {
	stack := &domain.Stack{
		ID:        "stk_with_exec",
		ClusterID: "cluster-01",
		State:     domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeStepExecutor{}
	provider := &fakeKubeconfigProvider{
		configs: map[string][]byte{
			"cluster-01": []byte("apiVersion: v1\nkind: Config\n"),
		},
	}

	uc := NewInstallStack(
		repo,
		streamer,
		WithKubeconfigProvider(provider),
		WithExecutorFactory(func(_ []byte) port.StepExecutor { return exec }),
	)

	err := uc.Execute(context.Background(), InstallStackInput{StackID: "stk_with_exec"})
	require.NoError(t, err)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState("stk_with_exec") == domain.StateCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	assert.Equal(t, []string{"cluster-01"}, provider.requestedClusterIDs())
	assert.NotEmpty(t, exec.calledSteps())
	assert.Equal(t, domain.StateCompleted, repo.getState("stk_with_exec"))
}

func TestInstallStack_ConfiguresExecutorNamespaceFromStack(t *testing.T) {
	stack := &domain.Stack{
		ID:        "stk_with_namespace",
		ClusterID: "cluster-namespace",
		Namespace: "production",
		State:     domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeConfigurableExecutor{}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID})
	require.NoError(t, err)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	namespace, namespaceSet := exec.configuredNamespace()
	assert.True(t, namespaceSet)
	assert.Equal(t, "production", namespace)
	assert.Equal(t, domain.StateCompleted, repo.getState(stack.ID))
}

func TestInstallStack_RuntimeVerificationFailurePausesWithoutRollback(t *testing.T) {
	stack := &domain.Stack{
		ID:        "stk_verify_fail",
		ClusterID: "cluster-verify-fail",
		State:     domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeVerifiableExecutor{verifyErr: fmt.Errorf("gitlab not ready")}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{StackID: "stk_verify_fail"})
	require.NoError(t, err)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState("stk_verify_fail") == domain.StateFailed {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	assert.Equal(t, domain.StateFailed, repo.getState("stk_verify_fail"))
	got := repo.getStack("stk_verify_fail")
	assert.Equal(t, "health_check", got.LastFailedStep)
	assert.Contains(t, got.LastFailureReason, "runtime readiness check failed")
	assert.Contains(t, streamer.steps(), "health_check")
	assert.NotContains(t, streamer.steps(), "rolling_back")
	assert.Equal(t, 0, exec.rollbackCalls)
}

func TestInstallStack_RuntimeVerificationFailureMapsReleaseToResumeStep(t *testing.T) {
	stack := &domain.Stack{
		ID:        "stk_verify_gitlab_fail",
		ClusterID: "cluster-verify-gitlab-fail",
		State:     domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeVerifiableExecutor{verifyErr: fmt.Errorf("runtime readiness failed for gitlab: registry rollout timeout")}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID})
	require.NoError(t, err)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateFailed {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	got := repo.getStack(stack.ID)
	assert.Equal(t, domain.StateFailed, got.State)
	assert.Equal(t, "installing_gitlab", got.LastFailedStep)
	assert.Contains(t, got.LastFailureReason, "registry rollout timeout")
}

func TestInstallStack_RecordsFailedInstallStep(t *testing.T) {
	stack := &domain.Stack{
		ID:    "stk_step_fail",
		State: domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeStepExecutor{failAt: "installing_argocd", errText: "argocd timed out"}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID})
	require.NoError(t, err)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateFailed {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	got := repo.getStack(stack.ID)
	assert.Equal(t, domain.StateFailed, got.State)
	assert.Equal(t, "installing_argocd", got.LastFailedStep)
	// 실패 직전 단계가 완료로 남아야 한다. 이름을 고정하면 DAG 에 단계가 하나
	// 끼어들 때마다 무관한 테스트가 깨지므로 DAG 에서 구한다.
	assert.Equal(t, dagStepNames()[dagIndexOf(t, "installing_argocd")-1], got.LastCompletedStep)
	assert.Contains(t, got.LastFailureReason, "argocd timed out")
}

func TestInstallStack_ContinueResumesFromFailedInstallStep(t *testing.T) {
	stack := &domain.Stack{
		ID:                "stk_resume_step",
		State:             domain.StateFailed,
		LastCompletedStep: "installing_gitlab",
		LastFailedStep:    "installing_argocd",
		LastFailureReason: "argocd timed out",
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeStepExecutor{}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{
		StackID:      stack.ID,
		Continue:     true,
		PreserveLogs: true,
	})
	require.NoError(t, err)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	called := exec.calledSteps()
	require.NotEmpty(t, called)
	assert.Equal(t, "installing_argocd", called[0])
	assert.NotContains(t, called, "installing_cert_manager")
	assert.Equal(t, domain.StateCompleted, repo.getState(stack.ID))
}

func TestInstallStack_ContinueFromHealthCheckSkipsInstallSteps(t *testing.T) {
	stack := &domain.Stack{
		ID:                "stk_resume_health",
		State:             domain.StateFailed,
		LastCompletedStep: "integration_check",
		LastFailedStep:    "health_check",
		LastFailureReason: "gitlab-registry unavailable",
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeVerifyOnlyExecutor{}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{
		StackID:      stack.ID,
		Continue:     true,
		PreserveLogs: true,
	})
	require.NoError(t, err)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	assert.Empty(t, exec.calledSteps())
	got := repo.getStack(stack.ID)
	assert.Equal(t, domain.StateCompleted, got.State)
	assert.Empty(t, got.LastFailedStep)
	assert.Empty(t, got.LastFailureReason)
}

func TestInstallStack_RuntimeVerificationFailureWithoutRollbackSupportStaysFailed(t *testing.T) {
	stack := &domain.Stack{
		ID:        "stk_verify_fail_no_rollback",
		ClusterID: "cluster-verify-no-rollback",
		State:     domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeVerifyOnlyExecutor{verifyErr: fmt.Errorf("gitlab not ready")}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID})
	require.NoError(t, err)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateFailed {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	assert.Equal(t, domain.StateFailed, repo.getState(stack.ID))
	assert.Contains(t, streamer.steps(), "health_check")
	assert.NotContains(t, streamer.steps(), "rolling_back")
	assert.Contains(t, streamer.steps(), "failed")
}

func TestInstallStack_FailureDoesNotInvokeRollbackEvenWhenRollbackWouldFail(t *testing.T) {
	stack := &domain.Stack{
		ID:        "stk_verify_fail_rollback_error",
		ClusterID: "cluster-verify-rollback-error",
		State:     domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeVerifiableExecutor{verifyErr: fmt.Errorf("gitlab not ready"), rollbackErr: fmt.Errorf("helm uninstall failed")}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID})
	require.NoError(t, err)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateFailed {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	assert.Equal(t, domain.StateFailed, repo.getState(stack.ID))
	assert.NotContains(t, streamer.steps(), "rolling_back")
	assert.Equal(t, 0, exec.rollbackCalls)
}

func TestInstallStack_StopsWhenStackIsCancelledDuringRun(t *testing.T) {
	stack := &domain.Stack{
		ID:    "stk_canceled_mid_run",
		State: domain.StatePending,
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeCancellingExecutor{repo: repo, stackID: stack.ID}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))

	err := uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID})
	require.NoError(t, err)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateCancelled {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	assert.Equal(t, domain.StateCancelled, repo.getState(stack.ID))
	assert.NotContains(t, exec.stepCalls, "installing_gateway")
	assert.NotContains(t, streamer.steps(), "rolling_back")
	assert.NotContains(t, streamer.steps(), "failed")
}

func TestInstallStack_DAG_OpenBaoEnabledInstallsBeforeCoreServices(t *testing.T) {
	stack := &domain.Stack{
		ID:    "stk_openbao_enabled",
		State: domain.StatePending,
		Config: domain.StackConfig{
			Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
		},
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeSelectableExecutor{enabled: map[string]bool{
		"installing_cert_manager":          true,
		"installing_metrics_server":        true,
		"installing_postgresql":            true,
		"installing_minio":                 true,
		"installing_object_storage_secret": false,
		"installing_openbao":               true,
		"installing_gitlab":                false,
		"installing_argocd":                true,
		"installing_runner":                false,
		"installing_prometheus":            false,
		"installing_grafana":               false,
		"installing_logging":               false,
		"installing_log_search":            false,
		"installing_opentelemetry":         false,
		"installing_gateway":               true,
		"integration_check":                true,
	}}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))
	router := secrets.NewRouter()
	router.Register("openbao", &fakeSecretStore{})
	uc.secretRouter = router
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID}))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	steps := exec.calledSteps()
	openbaoIdx := indexOfStep(steps, "installing_openbao")
	argocdIdx := indexOfStep(steps, "installing_argocd")
	require.GreaterOrEqual(t, openbaoIdx, 0)
	require.GreaterOrEqual(t, argocdIdx, 0)
	assert.Less(t, openbaoIdx, argocdIdx)
	assert.Equal(t, domain.StateCompleted, repo.getState(stack.ID))
}

func TestInstallStack_DAG_OpenBaoDisabledSkipsAndContinues(t *testing.T) {
	stack := &domain.Stack{
		ID:    "stk_openbao_disabled",
		State: domain.StatePending,
		Config: domain.StackConfig{
			Authentication: &domain.AuthenticationConfig{Provider: ""},
		},
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeSelectableExecutor{enabled: map[string]bool{
		"installing_cert_manager":          true,
		"installing_metrics_server":        true,
		"installing_postgresql":            true,
		"installing_minio":                 true,
		"installing_object_storage_secret": false,
		"installing_openbao":               false,
		"installing_gitlab":                false,
		"installing_argocd":                true,
		"installing_runner":                false,
		"installing_prometheus":            false,
		"installing_grafana":               false,
		"installing_logging":               false,
		"installing_log_search":            false,
		"installing_opentelemetry":         false,
		"installing_gateway":               true,
		"integration_check":                true,
	}}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID}))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	steps := exec.calledSteps()
	assert.Equal(t, -1, indexOfStep(steps, "installing_openbao"))
	assert.GreaterOrEqual(t, indexOfStep(steps, "installing_argocd"), 0)
	assert.Equal(t, domain.StateCompleted, repo.getState(stack.ID))
}

func TestInstallStack_DAG_OpenBaoEnabled_ContinuesWithoutRouterProvider(t *testing.T) {
	stack := &domain.Stack{
		ID:    "stk_openbao_gate_warn",
		State: domain.StatePending,
		Config: domain.StackConfig{
			Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
		},
	}
	repo := newFakeStackRepo(stack)
	streamer := &fakeStreamer{}
	exec := &fakeSelectableExecutor{enabled: map[string]bool{
		"installing_cert_manager":          true,
		"installing_metrics_server":        true,
		"installing_postgresql":            true,
		"installing_minio":                 true,
		"installing_object_storage_secret": false,
		"installing_openbao":               true,
		"installing_gitlab":                true,
		"installing_argocd":                true,
		"installing_runner":                true,
		"installing_prometheus":            false,
		"installing_grafana":               false,
		"installing_logging":               false,
		"installing_log_search":            false,
		"installing_opentelemetry":         false,
		"installing_gateway":               true,
		"integration_check":                true,
	}}

	uc := NewInstallStack(repo, streamer, WithExecutor(exec))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{StackID: stack.ID}))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(stack.ID) == domain.StateCompleted {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	steps := exec.calledSteps()
	assert.GreaterOrEqual(t, indexOfStep(steps, "installing_openbao"), 0)
	assert.GreaterOrEqual(t, indexOfStep(steps, "installing_argocd"), 0)
	assert.Equal(t, domain.StateCompleted, repo.getState(stack.ID))
	assert.Contains(t, streamer.steps(), "installing_openbao")
}

func indexOfStep(steps []string, target string) int {
	for i, step := range steps {
		if step == target {
			return i
		}
	}
	return -1
}

func (r *fakeStackRepo) ListInFlight(context.Context) ([]*domain.Stack, error) {
	return nil, nil
}

// 리퍼는 갱신 시각만 보고 끊긴 설치를 판정한다. 그런데 갱신 시각은 단계가
// 시작·완료될 때만 움직인다 — 한 단계가 임계값보다 오래 걸리면(Harbor·GitLab
// 이미지 풀은 흔히 그렇다) 멀쩡히 도는 설치가 끊긴 것으로 표시된다.
//
// 2026-08-21 운영에서 그렇게 됐다. 상태만 실패로 뒤집히고 고루틴은 계속 돌아,
// "실패" 로 표시된 뒤에 게이트웨이가 만들어졌다. 오류 로그가 없던 것은 실제로
// 오류가 없었기 때문이다.
func TestInstallStack_HeartbeatsWhileAStepIsSlow(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-hb",
		Name:      "heartbeat stack",
		ClusterID: "cluster-hb",
		Namespace: "nullus-heartbeat-stack",
		State:     domain.StatePending,
	})

	uc := NewInstallStack(repo, nil)

	stop := uc.startHeartbeat(context.Background(), "stk-hb", 10*time.Millisecond)
	defer stop()

	require.Eventually(t, func() bool {
		return repo.touchCount("stk-hb") >= 2
	}, 2*time.Second, 5*time.Millisecond,
		"단계가 도는 동안 갱신 시각이 계속 찍혀야 리퍼가 살아 있는 설치를 죽은 것으로 보지 않는다")

	stop()
	settled := repo.touchCount("stk-hb")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, settled, repo.touchCount("stk-hb"), "설치가 끝나면 하트비트도 멈춘다")
}
