package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stacklog "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// newRetryEcho boots an Echo server with the Retry route + gate wired up.
// Phase 3 of F8 follow-up. Reuses stubClusterReader from deploy_handler_compat_test.go.
func newRetryEcho(
	t *testing.T,
	clusterArchs []string,
	sink audit.Sink,
) (*echo.Echo, *stackrepo.MemoryStackRepository) {
	t.Helper()
	e := echo.New()
	stackRepo := stackrepo.NewMemoryStackRepository()
	streamer := stacklog.NewMemoryStreamer()
	install := usecase.NewInstallStack(stackRepo, streamer)

	reader := &stubClusterReader{summary: &port.ClusterSummary{
		ID:                "cluster-1",
		NodeArchitectures: clusterArchs,
	}}
	validate := usecase.NewValidateCompatibility(
		stackrepo.NewMemoryCompatibilityRepository(),
		usecase.WithClusterReader(reader),
		usecase.WithStackRepository(stackRepo),
	)

	h := stackhandler.NewDeployHandler(install, stackRepo, streamer, sink).
		WithOptions(stackhandler.WithValidateCompatibility(validate))

	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1, e)
	return e, stackRepo
}

func seedStackInState(
	t *testing.T,
	repo *stackrepo.MemoryStackRepository,
	id string,
	state domain.DeploymentState,
	tools []domain.ToolConfig,
) string {
	t.Helper()
	stack := &domain.Stack{
		ID:         id,
		Name:       id,
		TemplateID: "gitlab-allinone-v1",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
		Namespace:  "nullus",
		Tools:      tools,
		State:      state,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	require.NoError(t, repo.Create(context.Background(), stack))
	return stack.ID
}

// verified matrix + amd64-only cluster → gate passes → failed stack
// can retry to 202.
func TestRetry_FromFailed_VerifiedPasses(t *testing.T) {
	sink := audit.NewMemorySink()
	e, repo := newRetryEcho(t, []string{"amd64"}, sink)
	id := seedStackInState(t, repo, "stk-retry-failed", domain.StateFailed,
		[]domain.ToolConfig{
			{Category: "source_repository", Name: "GitLab CE"},
			{Category: "ci_platform", Name: "GitLab CI"},
			{Category: "container_registry", Name: "GitLab Registry"},
		})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/retry", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	entries := sink.Snapshot()
	require.Len(t, entries, 1)
	assert.Equal(t, "retry", entries[0].Action)
	assert.Equal(t, "failed", entries[0].Details["previous_state"])
}

// rolled_back → retry should also succeed.
func TestRetry_FromRolledBack_VerifiedPasses(t *testing.T) {
	sink := audit.NewMemorySink()
	e, repo := newRetryEcho(t, []string{"amd64"}, sink)
	id := seedStackInState(t, repo, "stk-retry-rb", domain.StateRolledBack,
		[]domain.ToolConfig{
			{Category: "source_repository", Name: "GitLab CE"},
			{Category: "ci_platform", Name: "GitLab CI"},
			{Category: "container_registry", Name: "GitLab Registry"},
		})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/retry", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	entries := sink.Snapshot()
	require.Len(t, entries, 1)
	assert.Equal(t, "rolled_back", entries[0].Details["previous_state"])
}

// Non-retryable state → 409.
func TestRetry_FromCompleted_Returns409(t *testing.T) {
	sink := audit.NewMemorySink()
	e, repo := newRetryEcho(t, []string{"amd64"}, sink)
	id := seedStackInState(t, repo, "stk-retry-completed", domain.StateCompleted,
		[]domain.ToolConfig{
			{Category: "source_repository", Name: "GitLab CE"},
			{Category: "ci_platform", Name: "GitLab CI"},
			{Category: "container_registry", Name: "GitLab Registry"},
		})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/retry", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "STACK_RETRY_INVALID_STATE", resp["error"]["code"])
	assert.Empty(t, sink.Snapshot(), "rejected retry must not emit audit entry")
}

// failed + untested+mixed combo + ack missing → 400 DEPLOY_COMPAT_WARN_UNACK.
func TestRetry_FailedWithWarn_NoAckBlocks(t *testing.T) {
	sink := audit.NewMemorySink()
	e, repo := newRetryEcho(t, []string{"amd64", "arm64"}, sink)
	id := seedStackInState(t, repo, "stk-retry-warn-unack", domain.StateFailed,
		[]domain.ToolConfig{
			{Category: "source_repository", Name: "GitHub"},
			{Category: "ci_platform", Name: "GitHub Actions"},
			{Category: "container_registry", Name: "Harbor"},
		})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/retry", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "DEPLOY_COMPAT_WARN_UNACK", resp["error"]["code"])
	assert.Empty(t, sink.Snapshot())
}

// Same combo with ack=true → 202 + audit records retry + ack.
func TestRetry_FailedWithWarn_AckAccepted(t *testing.T) {
	sink := audit.NewMemorySink()
	e, repo := newRetryEcho(t, []string{"amd64", "arm64"}, sink)
	id := seedStackInState(t, repo, "stk-retry-warn-ack", domain.StateFailed,
		[]domain.ToolConfig{
			{Category: "source_repository", Name: "GitHub"},
			{Category: "ci_platform", Name: "GitHub Actions"},
			{Category: "container_registry", Name: "Harbor"},
		})

	body, _ := json.Marshal(map[string]any{"acknowledge_warnings": true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/retry", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	entries := sink.Snapshot()
	require.Len(t, entries, 1)
	assert.Equal(t, "retry", entries[0].Action)
	assert.Equal(t, true, entries[0].Details["acknowledge_warnings"])
	assert.Equal(t, "warn", entries[0].Details["compatibility_verdict"])
}
