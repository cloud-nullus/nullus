//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	logadapter "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// fakeStepExecutor implements port.StepExecutor. When the shared atomic.Bool
// is true, ExecuteStep fails with a deterministic error — the exact
// deployment failure injection Task 7 needs to drive handleFailure →
// rollback → StateRolledBack in the state machine.
type fakeStepExecutor struct {
	fail     *atomic.Bool
	failStep string // optional specific step to fail on; blank = all steps fail
}

func (e *fakeStepExecutor) ExecuteStep(_ context.Context, _, step, _ string) error {
	if e.fail.Load() && (e.failStep == "" || e.failStep == step) {
		return errors.New("helm install failed (simulated)")
	}
	return nil
}

// warnClusterReader is a test double for port.ClusterReader. Returns the
// pre-configured summary (typically a cluster whose NodeArchitectures
// trigger TOOL_ARCH_UNSUPPORTED against the untested matrix) for any id.
type warnClusterReader struct {
	summary *port.ClusterSummary
}

func (r *warnClusterReader) GetClusterSummary(_ context.Context, _ string) (*port.ClusterSummary, error) {
	return r.summary, nil
}

// warnServer bundles the per-subtest HTTP server and the handles the test
// uses to drive state: the atomic failure switch, repositories for direct
// state rewind (Task 7 §E accepts test-level state rewind in lieu of a
// production /retry endpoint), and history repo for rollback seeding.
type warnServer struct {
	baseURL     string
	fail        *atomic.Bool
	stackRepo   *stackrepo.MemoryStackRepository
	historyRepo *stackrepo.MemoryHistoryRepository
}

// newWarnForcedTestServer boots an isolated httptest server wired with
// in-memory repositories so each subtest starts from a clean slate. The
// bundled compat repo uses the Narwhal seed `github-argocd-v1` (status
// untested, Harbor amd64-only) — pairing it with an arm64-only cluster
// produces the warn + TOOL_ARCH_UNSUPPORTED verdict the task scenario
// centers on.
func newWarnForcedTestServer(t *testing.T) *warnServer {
	t.Helper()

	memStackRepo := stackrepo.NewMemoryStackRepository()
	memCompatRepo := stackrepo.NewMemoryCompatibilityRepository()
	memHistoryRepo := stackrepo.NewMemoryHistoryRepository()
	memStreamer := logadapter.NewMemoryStreamer()

	fail := &atomic.Bool{}
	executor := &fakeStepExecutor{fail: fail}

	installUC := stackuc.NewInstallStack(
		memStackRepo,
		memStreamer,
		stackuc.WithExecutor(executor),
	)

	// Cluster reader returns arm64-only → untested matrix (Harbor
	// amd64-only) triggers TOOL_ARCH_UNSUPPORTED warning.
	reader := &warnClusterReader{summary: &port.ClusterSummary{
		ID:                "warn-cluster",
		OrgID:             "org-warn",
		NodeArchitectures: []string{"arm64"},
	}}

	validateUC := stackuc.NewValidateCompatibility(
		memCompatRepo,
		stackuc.WithClusterReader(reader),
		stackuc.WithStackRepository(memStackRepo),
	)

	manageHistoryUC := stackuc.NewManageHistory(memHistoryRepo)

	deployHandler := stackhandler.NewDeployHandler(installUC, memStackRepo, memStreamer).
		WithOptions(stackhandler.WithValidateCompatibility(validateUC))
	compatHandler := stackhandler.NewCompatibilityHandler(memCompatRepo, validateUC)
	historyHandler := stackhandler.NewHistoryHandler(memHistoryRepo, memStackRepo, manageHistoryUC)

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler
	e.Use(echomw.Recover())

	v1 := e.Group("/api/v1")
	stacks := v1.Group("/stacks")
	deployHandler.RegisterRoutes(v1, e)
	compatHandler.RegisterRoutes(stacks)
	historyHandler.RegisterRoutes(stacks)

	ts := httptest.NewServer(e)
	t.Cleanup(func() { ts.Close() })

	return &warnServer{
		baseURL:     ts.URL,
		fail:        fail,
		stackRepo:   memStackRepo,
		historyRepo: memHistoryRepo,
	}
}

// seedWarnStack inserts a stack pointing at the github-argocd-v1 (untested)
// matrix with Harbor as container_registry. The combination of an arm64-only
// cluster + Harbor (amd64-only) is what produces the warn verdict under
// test.
func (s *warnServer) seedWarnStack(t *testing.T, id string) {
	t.Helper()
	now := time.Now().UTC()
	stack := &domain.Stack{
		ID:         id,
		Name:       id,
		TemplateID: "github-argocd-v1",
		OrgID:      "org-warn",
		ClusterID:  "warn-cluster",
		Namespace:  "nullus",
		Tools: []domain.ToolConfig{
			{Category: "source_repository", Name: "GitHub", AppVersion: "external"},
			{Category: "ci_platform", Name: "GitHub Actions", AppVersion: "external"},
			{Category: "container_registry", Name: "Harbor", HelmVersion: "1.15.0", AppVersion: "2.11.0"},
		},
		State: domain.StatePending,
		Config: domain.StackConfig{
			Artifacts: domain.ArtifactsConfig{
				ContainerRegistry: domain.ToolSelection{Name: "Harbor", Version: "2.11.0", Enabled: true},
			},
			Pipeline: domain.PipelineConfig{
				CDTool: domain.ToolSelection{Name: "Argo CD", Version: "v2.8.3", Enabled: true},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.stackRepo.Create(context.Background(), stack); err != nil {
		t.Fatalf("seed warn stack: %v", err)
	}
}

// do sends an HTTP request and decodes the JSON body. Returns HTTP status
// and the decoded map so assertions can drill into nested fields.
func (s *warnServer) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, s.baseURL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	parsed := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Logf("non-JSON body: %s", string(raw))
		}
	}
	return resp.StatusCode, parsed
}

// waitForState polls stack state until it matches one of `want` or the
// deadline is hit. Returns the last observed state.
func (s *warnServer) waitForState(t *testing.T, id string, want ...domain.DeploymentState) domain.DeploymentState {
	t.Helper()
	wantSet := make(map[domain.DeploymentState]struct{}, len(want))
	for _, w := range want {
		wantSet[w] = struct{}{}
	}
	deadline := time.Now().Add(5 * time.Second)
	var last domain.DeploymentState
	for time.Now().Before(deadline) {
		st, err := s.stackRepo.GetByID(context.Background(), id)
		if err == nil && st != nil {
			last = st.State
			if _, ok := wantSet[last]; ok {
				return last
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return last
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestF8Task7_WarnForcedRetryRollback(t *testing.T) {
	t.Run("A_validate_returns_warn_with_tool_arch_unsupported", func(t *testing.T) {
		s := newWarnForcedTestServer(t)
		s.seedWarnStack(t, "stk-warn-A")

		status, body := s.do(t, http.MethodPost, "/api/v1/stacks/stk-warn-A/validate", map[string]any{})
		if status != http.StatusOK {
			t.Fatalf("validate: HTTP %d, body=%v", status, body)
		}
		overall, _ := body["overall"].(map[string]any)
		if overall["state"] != "warn" {
			t.Errorf("verdict.overall.state = %v, want warn", overall["state"])
		}
		if !hasIssueCode(body, "TOOL_ARCH_UNSUPPORTED") {
			t.Errorf("verdict missing TOOL_ARCH_UNSUPPORTED: %v", body["issues"])
		}
	})

	t.Run("B_deploy_without_ack_blocks", func(t *testing.T) {
		s := newWarnForcedTestServer(t)
		s.seedWarnStack(t, "stk-warn-B")

		status, body := s.do(t, http.MethodPost, "/api/v1/stacks/stk-warn-B/deploy", map[string]any{})
		if status != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%v", status, body)
		}
		errObj, _ := body["error"].(map[string]any)
		if errObj["code"] != "DEPLOY_COMPAT_WARN_UNACK" {
			t.Errorf("error.code = %v, want DEPLOY_COMPAT_WARN_UNACK", errObj["code"])
		}
		verdict, _ := errObj["verdict"].(map[string]any)
		overall, _ := verdict["overall"].(map[string]any)
		if overall["state"] != "warn" {
			t.Errorf("verdict.overall.state = %v, want warn", overall["state"])
		}
		// Stack state is untouched by a blocked deploy.
		st, _ := s.stackRepo.GetByID(context.Background(), "stk-warn-B")
		if st != nil && st.State != domain.StatePending {
			t.Errorf("stack.state after block = %q, want %q", st.State, domain.StatePending)
		}
	})

	t.Run("C_deploy_with_ack_succeeds", func(t *testing.T) {
		s := newWarnForcedTestServer(t)
		s.seedWarnStack(t, "stk-warn-C")

		status, body := s.do(t, http.MethodPost, "/api/v1/stacks/stk-warn-C/deploy",
			map[string]any{"acknowledge_warnings": true})
		if status != http.StatusAccepted {
			t.Fatalf("expected 202, got %d body=%v", status, body)
		}

		got := s.waitForState(t, "stk-warn-C", domain.StateCompleted, domain.StateFailed, domain.StateRolledBack)
		if got != domain.StateCompleted {
			t.Errorf("final state = %q, want completed", got)
		}
	})

	t.Run("D_deploy_with_ack_but_executor_fails_rolls_back", func(t *testing.T) {
		s := newWarnForcedTestServer(t)
		s.seedWarnStack(t, "stk-warn-D")
		s.fail.Store(true)

		status, _ := s.do(t, http.MethodPost, "/api/v1/stacks/stk-warn-D/deploy",
			map[string]any{"acknowledge_warnings": true})
		if status != http.StatusAccepted {
			t.Fatalf("expected 202 (gate passed with ack), got %d", status)
		}

		got := s.waitForState(t, "stk-warn-D", domain.StateRolledBack, domain.StateFailed)
		if got != domain.StateRolledBack {
			t.Errorf("final state = %q, want rolled_back (handleFailure path)", got)
		}
	})

	t.Run("E_retry_after_failure_via_state_rewind", func(t *testing.T) {
		// Retry contract: the state machine permits rolled_back → pending.
		// Production lacks a `/stacks/:id/retry` endpoint today, so this
		// subtest verifies the transition contract itself — rewind via
		// repo Update, then call /deploy with ack=true + executor healthy.
		// Follow-up ticket: plan §6 "POST /stacks/:id/retry".
		s := newWarnForcedTestServer(t)
		s.seedWarnStack(t, "stk-warn-E")
		s.fail.Store(true)

		status, _ := s.do(t, http.MethodPost, "/api/v1/stacks/stk-warn-E/deploy",
			map[string]any{"acknowledge_warnings": true})
		if status != http.StatusAccepted {
			t.Fatalf("first deploy: expected 202, got %d", status)
		}
		if got := s.waitForState(t, "stk-warn-E", domain.StateRolledBack); got != domain.StateRolledBack {
			t.Fatalf("after first deploy state = %q, want rolled_back", got)
		}

		// Test-level rewind to pending: rolled_back → pending is allowed
		// by validTransitions. Simulates an operator clicking a future
		// Retry button that would also flip the health switch.
		st, err := s.stackRepo.GetByID(context.Background(), "stk-warn-E")
		if err != nil || st == nil {
			t.Fatalf("load stk-warn-E for rewind: %v", err)
		}
		if err := st.TransitionTo(domain.StatePending); err != nil {
			t.Fatalf("rewind to pending: %v", err)
		}
		if err := s.stackRepo.Update(context.Background(), st); err != nil {
			t.Fatalf("persist rewind: %v", err)
		}

		// Heal the executor and retry.
		s.fail.Store(false)
		status, _ = s.do(t, http.MethodPost, "/api/v1/stacks/stk-warn-E/deploy",
			map[string]any{"acknowledge_warnings": true})
		if status != http.StatusAccepted {
			t.Fatalf("retry deploy: expected 202, got %d", status)
		}
		if got := s.waitForState(t, "stk-warn-E", domain.StateCompleted); got != domain.StateCompleted {
			t.Errorf("retry final state = %q, want completed", got)
		}
	})

	t.Run("F_rollback_to_prior_version", func(t *testing.T) {
		// Scope: verify the rollback endpoint contract — given a stored
		// prior version, POST /stacks/:id/rollback must restore the
		// stack config and append a new history row. We DON'T go through
		// an initial deploy first because `completed` is a terminal state
		// in validTransitions and would block any further config changes;
		// the prompt's §1.1.3-F explicitly permits direct history seeding.
		s := newWarnForcedTestServer(t)
		s.seedWarnStack(t, "stk-warn-F")

		// Simulate "prior good config" as v1 in history.
		v1Config := domain.StackConfig{
			Artifacts: domain.ArtifactsConfig{
				ContainerRegistry: domain.ToolSelection{Name: "Harbor", Version: "2.10.0-stable", Enabled: true},
			},
		}
		v1 := &domain.StackVersion{
			ID:           "ver-F-v1",
			StackID:      "stk-warn-F",
			Version:      1,
			ChangedBy:    "tester",
			ChangeReason: "initial config",
			Config:       v1Config,
			CreatedAt:    time.Now().UTC().Add(-time.Hour),
		}
		if err := s.historyRepo.SaveVersion(context.Background(), v1); err != nil {
			t.Fatalf("seed v1: %v", err)
		}

		// Mutate the stack to a "bad" config. No deploy — we're only
		// testing the rollback endpoint.
		st, _ := s.stackRepo.GetByID(context.Background(), "stk-warn-F")
		if st == nil {
			t.Fatalf("stack missing")
		}
		st.Config = domain.StackConfig{
			Artifacts: domain.ArtifactsConfig{
				ContainerRegistry: domain.ToolSelection{Name: "Harbor", Version: "broken-tag", Enabled: true},
			},
		}
		if err := s.stackRepo.Update(context.Background(), st); err != nil {
			t.Fatalf("persist bad config: %v", err)
		}

		// Rollback endpoint must restore v1 config + write a new history entry.
		status, body := s.do(t, http.MethodPost, "/api/v1/stacks/stk-warn-F/rollback",
			map[string]any{"versionId": "ver-F-v1", "reason": "retry cancelled"})
		if status != http.StatusOK {
			t.Fatalf("rollback: expected 200, got %d body=%v", status, body)
		}

		st, _ = s.stackRepo.GetByID(context.Background(), "stk-warn-F")
		if st == nil {
			t.Fatalf("stack missing after rollback")
		}
		cfg, ok := st.Config.(domain.StackConfig)
		if !ok {
			t.Fatalf("stack.Config is not StackConfig: %T", st.Config)
		}
		if cfg.Artifacts.ContainerRegistry.Version != "2.10.0-stable" {
			t.Errorf("rollback did not restore Harbor version: got %q",
				cfg.Artifacts.ContainerRegistry.Version)
		}
		versions, _ := s.historyRepo.ListVersions(context.Background(), "stk-warn-F")
		if len(versions) < 2 {
			t.Errorf("rollback should append a new history version, got %d total", len(versions))
		}
	})
}

// hasIssueCode returns true when the validate response body contains an
// issue whose `code` equals want.
func hasIssueCode(body map[string]any, want string) bool {
	issues, _ := body["issues"].([]any)
	for _, i := range issues {
		m, ok := i.(map[string]any)
		if !ok {
			continue
		}
		if m["code"] == want {
			return true
		}
	}
	return false
}

// Ensure the compile-time references to fmt/bytes keep in place when the
// body-printing debug hooks are removed in future cleanups.
var _ = fmt.Sprintf
