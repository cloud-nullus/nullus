package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

// newDeployEchoWithAudit 은 감사 싱크가 붙은 배포 라우터를 만든다.
func newDeployEchoWithAudit(
	t *testing.T,
	clusterArchs []string,
) (*echo.Echo, *stackrepo.MemoryStackRepository, *audit.MemorySink) {
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
	sink := audit.NewMemorySink()

	h := stackhandler.NewDeployHandler(install, stackRepo, streamer, sink).
		WithOptions(stackhandler.WithValidateCompatibility(validate))
	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1.Group("/stacks"), e)
	return e, stackRepo, sink
}

// deployBodyWithToken 은 마법사가 보내는 배포 요청 본문이다.
func deployBodyWithToken(token string) *bytes.Reader {
	raw, _ := json.Marshal(map[string]any{
		"acknowledge_warnings": true,
		"source_control":       map[string]any{"personal_access_token": token},
	})
	return bytes.NewReader(raw)
}

func githubGateTools() []domain.ToolConfig {
	return []domain.ToolConfig{
		{Category: "source_repository", Name: "GitHub"},
		{Category: "ci_platform", Name: "GitHub Actions"},
		{Category: "container_registry", Name: "GHCR"},
	}
}

// 예전 클라이언트는 source_control 없이 호출한다. 새 필드 때문에 바인딩이
// 깨지면 GitHub 과 무관한 모든 배포가 400 으로 죽는다.
func TestDeploy_AcceptsSourceControlBody(t *testing.T) {
	e, repo := newDeployEchoWithGate(t, []string{"amd64", "arm64"})
	id := seedStackForGate(t, repo, "stk-sc-accept", githubGateTools())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", deployBodyWithToken("ghp_wizard"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code, "body=%s", rec.Body.String())
}

func TestDeploy_StillAcceptsBodyWithoutSourceControl(t *testing.T) {
	e, repo := newDeployEchoWithGate(t, []string{"amd64", "arm64"})
	id := seedStackForGate(t, repo, "stk-sc-legacy", githubGateTools())

	body, _ := json.Marshal(map[string]any{"acknowledge_warnings": true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
}

// PAT 가 응답에 되비치면 브라우저 개발자도구·프록시 로그에 그대로 남는다.
func TestDeploy_ResponseNeverEchoesToken(t *testing.T) {
	e, repo := newDeployEchoWithGate(t, []string{"amd64", "arm64"})
	id := seedStackForGate(t, repo, "stk-sc-echo", githubGateTools())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", deployBodyWithToken("ghp_super_secret"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.NotContains(t, rec.Body.String(), "ghp_super_secret")
}

// 감사 로그는 허용 목록으로 필드를 고른다. 요청 본문을 통째로 담게 되면
// 토큰이 감사 저장소에 영구히 남는다.
func TestDeploy_AuditNeverRecordsToken(t *testing.T) {
	e, repo, sink := newDeployEchoWithAudit(t, []string{"amd64", "arm64"})
	id := seedStackForGate(t, repo, "stk-sc-audit", githubGateTools())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+id+"/deploy", deployBodyWithToken("ghp_super_secret"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	entries := sink.Snapshot()
	require.NotEmpty(t, entries, "배포 감사 항목이 있어야 한다")
	raw, err := json.Marshal(entries)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "ghp_super_secret")
	assert.NotContains(t, string(raw), "personal_access_token")
}
