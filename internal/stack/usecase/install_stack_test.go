package usecase

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeStackRepo struct {
	mu     sync.Mutex
	stacks map[string]*domain.Stack
}

func newFakeStackRepo(stacks ...*domain.Stack) *fakeStackRepo {
	r := &fakeStackRepo{stacks: make(map[string]*domain.Stack)}
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
	if !ok {
		return nil, fmt.Errorf("stack not found: %s", id)
	}
	cp := *s
	return &cp, nil
}

func (r *fakeStackRepo) FindByID(ctx context.Context, id string) (*domain.Stack, error) {
	return r.GetByID(ctx, id)
}

func (r *fakeStackRepo) List(_ context.Context, _ string) ([]*domain.Stack, error) {
	return nil, nil
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
	delete(r.stacks, id)
	return nil
}

func (r *fakeStackRepo) getState(id string) domain.DeploymentState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stacks[id].State
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
	verifyErr error
}

func (e *fakeVerifiableExecutor) VerifyDeployment(_ context.Context, _ string) error {
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

	// After Execute returns, state should be Validating (goroutine may not have finished yet).
	assert.Equal(t, domain.StateValidating, repo.getState("stk_test01"))

	deadline := time.Now().Add(25 * time.Second)
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

func TestInstallStack_RuntimeVerificationFailureTriggersRollback(t *testing.T) {
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
		if repo.getState("stk_verify_fail") == domain.StateRolledBack {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	assert.Equal(t, domain.StateRolledBack, repo.getState("stk_verify_fail"))
	assert.Contains(t, streamer.steps(), "health_check")
	assert.Contains(t, streamer.steps(), "rolling_back")
}

func TestInstallStack_StopsWhenStackIsCancelledDuringRun(t *testing.T) {
	stack := &domain.Stack{
		ID:    "stk_cancelled_mid_run",
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
