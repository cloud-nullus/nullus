package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stacklog "github.com/cloud-nullus/draft/internal/stack/adapter/log"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubClusterReader mirrors the one in usecase tests but lives here so the
// handler test doesn't have to cross package boundaries.
type stubClusterReader struct {
	summary *port.ClusterSummary
}

func (s *stubClusterReader) GetClusterSummary(_ context.Context, _ string) (*port.ClusterSummary, error) {
	return s.summary, nil
}

// newDeployEchoWithGate boots an Echo instance wired with the F8-F3 Pre-Deploy
// Gate. The cluster reader's node architectures drive the verdict branch.
func newDeployEchoWithGate(t *testing.T, clusterArchs []string) (*echo.Echo, *stackrepo.MemoryStackRepository) {
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

	h := stackhandler.NewDeployHandler(install, stackRepo, streamer).
		WithOptions(stackhandler.WithValidateCompatibility(validate))

	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1, e)
	return e, stackRepo
}

func seedStackForGate(
	t *testing.T,
	repo *stackrepo.MemoryStackRepository,
	id string,
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
		State:      domain.StatePending,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	require.NoError(t, repo.Create(context.Background(), stack))
	return stack.ID
}

// Verified matrix (amd64-only tools) + amd64-only cluster → pass → 202.
func TestDeployHandler_Gate_Pass(t *testing.T) {
	e, repo := newDeployEchoWithGate(t, []string{"amd64"})
	id := seedStackForGate(t, repo, "stk-pass", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitLab CE"},
		{Category: "ci_platform", Name: "GitLab CI"},
		{Category: "container_registry", Name: "GitLab Registry"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
}

// Verified matrix + mixed arch cluster → fail → 400 DEPLOY_COMPAT_FAIL.
// Verdict body must carry TOOL_ARCH_UNSUPPORTED so the client can render it.
func TestDeployHandler_Gate_FailsOnArchMiss(t *testing.T) {
	e, repo := newDeployEchoWithGate(t, []string{"amd64", "arm64"})
	id := seedStackForGate(t, repo, "stk-fail", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitLab CE"},
		{Category: "ci_platform", Name: "GitLab CI"},
		{Category: "container_registry", Name: "GitLab Registry"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Verdict struct {
				Overall struct {
					State string `json:"state"`
				} `json:"overall"`
				Issues []struct {
					Code string `json:"code"`
				} `json:"issues"`
			} `json:"verdict"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "DEPLOY_COMPAT_FAIL", resp.Error.Code)
	assert.Equal(t, "fail", resp.Error.Verdict.Overall.State)
	var hasArchIssue bool
	for _, i := range resp.Error.Verdict.Issues {
		if i.Code == "TOOL_ARCH_UNSUPPORTED" {
			hasArchIssue = true
		}
	}
	assert.True(t, hasArchIssue, "expected TOOL_ARCH_UNSUPPORTED issue in verdict body")
}

// Untested matrix + mixed cluster + ack omitted → 400 DEPLOY_COMPAT_WARN_UNACK.
func TestDeployHandler_Gate_WarnWithoutAck(t *testing.T) {
	e, repo := newDeployEchoWithGate(t, []string{"amd64", "arm64"})
	id := seedStackForGate(t, repo, "stk-warn-unack", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitHub"},
		{Category: "ci_platform", Name: "GitHub Actions"},
		{Category: "container_registry", Name: "Harbor"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Verdict struct {
				Overall struct {
					State string `json:"state"`
				} `json:"overall"`
			} `json:"verdict"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "DEPLOY_COMPAT_WARN_UNACK", resp.Error.Code)
	assert.Equal(t, "warn", resp.Error.Verdict.Overall.State)
}

// Same combination + acknowledge_warnings=true → 202.
func TestDeployHandler_Gate_WarnWithAck(t *testing.T) {
	e, repo := newDeployEchoWithGate(t, []string{"amd64", "arm64"})
	id := seedStackForGate(t, repo, "stk-warn-ack", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitHub"},
		{Category: "ci_platform", Name: "GitHub Actions"},
		{Category: "container_registry", Name: "Harbor"},
	})

	body, _ := json.Marshal(map[string]any{"acknowledge_warnings": true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
}

// Cluster NodeArchitectures empty → CLUSTER_ARCH_UNKNOWN warn → blocks without
// ack, passes with ack. Confirms the "unknown" branch is gated too.
func TestDeployHandler_Gate_ClusterArchUnknown(t *testing.T) {
	// Empty cluster archs → "unknown" verdict → downgrade verified → warn.
	e, repo := newDeployEchoWithGate(t, nil)
	id := seedStackForGate(t, repo, "stk-arch-unknown", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitLab CE"},
		{Category: "ci_platform", Name: "GitLab CI"},
		{Category: "container_registry", Name: "GitLab Registry"},
	})

	// Without ack.
	reqNoAck := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", nil)
	recNoAck := httptest.NewRecorder()
	e.ServeHTTP(recNoAck, reqNoAck)
	require.Equal(t, http.StatusBadRequest, recNoAck.Code)

	var blocked struct {
		Error struct {
			Code    string `json:"code"`
			Verdict struct {
				Issues []struct {
					Code string `json:"code"`
				} `json:"issues"`
			} `json:"verdict"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recNoAck.Body.Bytes(), &blocked))
	assert.Equal(t, "DEPLOY_COMPAT_WARN_UNACK", blocked.Error.Code)
	var hasUnknown bool
	for _, i := range blocked.Error.Verdict.Issues {
		if i.Code == "CLUSTER_ARCH_UNKNOWN" {
			hasUnknown = true
		}
	}
	assert.True(t, hasUnknown, "expected CLUSTER_ARCH_UNKNOWN issue")

	// With ack — same stack, re-issue (still pending since install attempts on
	// a different stack instance would mutate state; seed a fresh row).
	id2 := seedStackForGate(t, repo, "stk-arch-unknown-ack", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitLab CE"},
		{Category: "ci_platform", Name: "GitLab CI"},
		{Category: "container_registry", Name: "GitLab Registry"},
	})
	body, _ := json.Marshal(map[string]any{"acknowledge_warnings": true})
	reqAck := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id2+"/deploy", bytes.NewReader(body))
	reqAck.Header.Set("Content-Type", "application/json")
	recAck := httptest.NewRecorder()
	e.ServeHTTP(recAck, reqAck)
	require.Equal(t, http.StatusAccepted, recAck.Code)
}

// Phase 2 audit.Sink verification: when a deploy is accepted with
// acknowledge_warnings=true, the audit entry must record that flag along
// with `compatibility_verdict` and `issue_codes` so downstream observers
// can trace why the gate was overridden.
func TestDeployHandler_Gate_AuditRecordsAckAndVerdict(t *testing.T) {
	e := echo.New()
	stackRepo := stackrepo.NewMemoryStackRepository()
	streamer := stacklog.NewMemoryStreamer()
	install := usecase.NewInstallStack(stackRepo, streamer)

	reader := &stubClusterReader{summary: &port.ClusterSummary{
		ID:                "cluster-1",
		NodeArchitectures: []string{"amd64", "arm64"},
	}}
	validate := usecase.NewValidateCompatibility(
		stackrepo.NewMemoryCompatibilityRepository(),
		usecase.WithClusterReader(reader),
		usecase.WithStackRepository(stackRepo),
	)
	sink := audit.NewMemorySink()

	h := stackhandler.NewDeployHandler(install, stackRepo, streamer, sink).
		WithOptions(stackhandler.WithValidateCompatibility(validate))
	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1, e)

	// Untested matrix + mixed-arch cluster → warn. Ack=true lets it through.
	id := seedStackForGate(t, stackRepo, "stk-audit-ack", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitHub"},
		{Category: "ci_platform", Name: "GitHub Actions"},
		{Category: "container_registry", Name: "Harbor"},
	})

	body, _ := json.Marshal(map[string]any{"acknowledge_warnings": true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	// Exactly one audit entry, and it captures the warn override fields.
	entries := sink.Snapshot()
	require.Len(t, entries, 1, "expected a single deploy audit entry")
	entry := entries[0]
	assert.Equal(t, "deploy", entry.Action)
	assert.Equal(t, "stack", entry.ResourceType)
	assert.Equal(t, id, entry.ResourceID)
	require.NotNil(t, entry.Details, "audit details must be populated")
	assert.Equal(t, true, entry.Details["acknowledge_warnings"])
	assert.Equal(t, "warn", entry.Details["compatibility_verdict"])

	codes, ok := entry.Details["issue_codes"].([]string)
	require.True(t, ok, "issue_codes must be []string")
	var hasUntested bool
	for _, c := range codes {
		if c == "MATRIX_UNTESTED" || c == "TOOL_ARCH_UNSUPPORTED" {
			hasUntested = true
		}
	}
	assert.True(t, hasUntested, "issue_codes should include warn issue: %v", codes)
}

// A blocked warn-unack deploy must NOT emit an audit entry (the action
// didn't happen). Guards against the regression of "audit everything
// regardless of outcome".
func TestDeployHandler_Gate_NoAuditOnBlockedWarn(t *testing.T) {
	e := echo.New()
	stackRepo := stackrepo.NewMemoryStackRepository()
	streamer := stacklog.NewMemoryStreamer()
	install := usecase.NewInstallStack(stackRepo, streamer)

	reader := &stubClusterReader{summary: &port.ClusterSummary{
		ID: "cluster-1", NodeArchitectures: []string{"amd64", "arm64"},
	}}
	validate := usecase.NewValidateCompatibility(
		stackrepo.NewMemoryCompatibilityRepository(),
		usecase.WithClusterReader(reader),
		usecase.WithStackRepository(stackRepo),
	)
	sink := audit.NewMemorySink()

	h := stackhandler.NewDeployHandler(install, stackRepo, streamer, sink).
		WithOptions(stackhandler.WithValidateCompatibility(validate))
	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1, e)

	id := seedStackForGate(t, stackRepo, "stk-audit-blocked", []domain.ToolConfig{
		{Category: "source_repository", Name: "GitHub"},
		{Category: "ci_platform", Name: "GitHub Actions"},
		{Category: "container_registry", Name: "Harbor"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, sink.Snapshot(), "blocked deploy must not emit audit entry")
}

