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
	compatRepo := stackrepo.NewMemoryCompatibilityRepository()
	seedUntestedMatrix(t, compatRepo)
	validate := usecase.NewValidateCompatibility(
		compatRepo,
		usecase.WithClusterReader(reader),
		usecase.WithStackRepository(stackRepo),
	)

	h := stackhandler.NewDeployHandler(install, stackRepo, streamer).
		WithOptions(stackhandler.WithValidateCompatibility(validate))

	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1.Group("/stacks"), e)
	return e, stackRepo
}

// untestedComboTools 는 게이트의 "검증되지 않은 조합" 분기를 시험하기 위한
// 조합이다. seedUntestedMatrix 가 세우는 행렬과 짝을 이룬다.
// Gitea + Jenkins 는 이제 출하되는 Golden Path(gitea-jenkins-argocd-v1)라
// 여기서 쓸 수 없다 — verified 매트릭스가 먼저 잡혀 warn 분기에 닿지 못한다.
// 아래 주석이 경고한 그 상황이므로, 어느 Golden Path 와도 겹치지 않는 조합을 쓴다.
var untestedComboTools = []domain.ToolConfig{
	{Category: "source_repository", Name: "Bitbucket"},
	{Category: "ci_platform", Name: "Drone"},
	{Category: "container_registry", Name: "Harbor"},
}

// seedUntestedMatrix 는 warn 분기 전용 행렬을 하나 얹는다.
//
// 예전에는 이 분기를 github-argocd-v1 로 시험했는데, 그 조합이 검증을 마치고
// verified 로 올라가면서 warn 을 낼 대상이 사라졌다 — 게이트 로직은 그대로인데
// 테스트 다섯 개가 함께 죽었다. 출하되는 Golden Path 를 빌려 쓰면 그 조합의
// 검증 상태가 바뀔 때마다 관계없는 테스트가 깨진다. 분기 전용 행렬을 따로 세워
// 시드 데이터의 모양에서 떼어 놓는다.
//
// Harbor 를 amd64 전용으로 두는 것은 "untested + 아키텍처 불일치" 가 fail 이
// 아니라 warn 에 머무는지까지 이 조합 하나로 보기 위해서다.
func seedUntestedMatrix(t *testing.T, repo *stackrepo.MemoryCompatibilityRepository) {
	t.Helper()
	require.NoError(t, repo.Create(context.Background(), &domain.CompatibilityMatrix{
		ID:     "untested-combo-v1",
		Name:   "Untested combination",
		Status: "untested",
		Kubernetes: domain.KubernetesCompat{
			Min: "1.26", Max: "1.35", Recommended: "1.31",
		},
		Tools: map[string]domain.ToolVersion{
			"source_repository": {
				Name: "Bitbucket", HelmVersion: "1.0.0", AppVersion: "8.19.0",
				MinK8sVersion: "1.26", ArchSupport: []string{"amd64", "arm64"}, Tier: "beta",
			},
			"ci_platform": {
				Name: "Drone", HelmVersion: "0.6.5", AppVersion: "2.25.0",
				MinK8sVersion: "1.26", ArchSupport: []string{"amd64", "arm64"}, Tier: "beta",
			},
			"container_registry": {
				Name: "Harbor", HelmVersion: "1.15.0", AppVersion: "2.11.0",
				MinK8sVersion: "1.27", ArchSupport: []string{"amd64"}, Tier: "beta",
			},
		},
	}))
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
	id := seedStackForGate(t, repo, "stk-warn-unack", untestedComboTools)

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
	id := seedStackForGate(t, repo, "stk-warn-ack", untestedComboTools)

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
	compatRepo := stackrepo.NewMemoryCompatibilityRepository()
	seedUntestedMatrix(t, compatRepo)
	validate := usecase.NewValidateCompatibility(
		compatRepo,
		usecase.WithClusterReader(reader),
		usecase.WithStackRepository(stackRepo),
	)
	sink := audit.NewMemorySink()

	h := stackhandler.NewDeployHandler(install, stackRepo, streamer, sink).
		WithOptions(stackhandler.WithValidateCompatibility(validate))
	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1.Group("/stacks"), e)

	// Untested matrix + mixed-arch cluster → warn. Ack=true lets it through.
	id := seedStackForGate(t, stackRepo, "stk-audit-ack", untestedComboTools)

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
	compatRepo := stackrepo.NewMemoryCompatibilityRepository()
	seedUntestedMatrix(t, compatRepo)
	validate := usecase.NewValidateCompatibility(
		compatRepo,
		usecase.WithClusterReader(reader),
		usecase.WithStackRepository(stackRepo),
	)
	sink := audit.NewMemorySink()

	h := stackhandler.NewDeployHandler(install, stackRepo, streamer, sink).
		WithOptions(stackhandler.WithValidateCompatibility(validate))
	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1.Group("/stacks"), e)

	id := seedStackForGate(t, stackRepo, "stk-audit-blocked", untestedComboTools)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, sink.Snapshot(), "blocked deploy must not emit audit entry")
}
