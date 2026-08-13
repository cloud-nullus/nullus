package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type stubReleaseManager struct {
	upgrades     []port.HelmUpgradeRequest
	upgradeError error
}

func (s *stubReleaseManager) ListReleases(context.Context, string) ([]port.ReleaseInfo, error) {
	return []port.ReleaseInfo{{
		ReleaseName: "harbor",
		StepName:    "installing_harbor",
		ChartName:   "harbor",
		Namespace:   "nullus",
		Revision:    2,
		Status:      "deployed",
	}}, nil
}

func (s *stubReleaseManager) GetValues(context.Context, string, string) (map[string]any, error) {
	return map[string]any{
		"externalURL": "http://harbor.nullus.local",
		"trivy":       map[string]any{"enabled": true},
	}, nil
}

func (s *stubReleaseManager) Upgrade(_ context.Context, req port.HelmUpgradeRequest) (*port.HelmUpgradeResult, error) {
	s.upgrades = append(s.upgrades, req)
	if s.upgradeError != nil {
		return nil, s.upgradeError
	}
	return &port.HelmUpgradeResult{ReleaseName: req.ReleaseName, Namespace: req.Namespace, Revision: 3, Status: "deployed"}, nil
}

// recordingAuditSink 는 감사 기록을 메모리에 모아 테스트가 내용을 검사하게 한다.
type recordingAuditSink struct {
	entries []audit.AuditEntry
}

func (r *recordingAuditSink) Log(_ context.Context, entry audit.AuditEntry) error {
	r.entries = append(r.entries, entry)
	return nil
}

type stubReleaseKubeconfigProvider struct{}

func (stubReleaseKubeconfigProvider) GetKubeconfig(context.Context, string) ([]byte, error) {
	return []byte("apiVersion: v1\nkind: Config\n"), nil
}

func newReleaseValuesServer(t *testing.T) (*echo.Echo, *stubReleaseManager) {
	t.Helper()

	stackRepo := stackrepo.NewMemoryStackRepository()
	require.NoError(t, stackRepo.Create(context.Background(), &domain.Stack{
		ID:        "stk_1",
		Namespace: "nullus",
		ClusterID: "cls_1",
		State:     domain.StateCompleted,
		Config:    domain.StackConfig{AccessDomain: "nullus.local"},
	}))

	manager := &stubReleaseManager{}
	uc := usecase.NewManageReleaseValues(
		stackRepo,
		stubReleaseKubeconfigProvider{},
		func([]byte) port.HelmReleaseManager { return manager },
		usecase.WithReleaseValuesHistory(usecase.NewManageHistory(stackrepo.NewMemoryHistoryRepository())),
	)

	e := echo.New()
	stackhandler.NewReleaseValuesHandler(uc, nil).RegisterRoutes(e.Group("/stacks"))
	return e, manager
}

// stacks 그룹에는 :id 를 쓰는 라우트(배포·재시도)와 :stackId 를 쓰는 라우트가
// 섞여 등록된다. 같은 자리의 파라미터 이름이 다르면 어느 이름으로 바인딩될지
// 등록 순서에 달리므로, 두 종류가 함께 있을 때도 스택 ID 를 읽어야 한다.
func TestReleaseValuesHandler_CoexistsWithIdParamRoutes(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	require.NoError(t, stackRepo.Create(context.Background(), &domain.Stack{
		ID: "stk_1", Namespace: "nullus", ClusterID: "cls_1", State: domain.StateCompleted,
		Config: domain.StackConfig{},
	}))
	uc := usecase.NewManageReleaseValues(
		stackRepo,
		stubReleaseKubeconfigProvider{},
		func([]byte) port.HelmReleaseManager { return &stubReleaseManager{} },
	)

	e := echo.New()
	stacks := e.Group("/stacks")
	// 배포 핸들러가 쓰는 형태를 먼저 등록한다 — 실제 main.go 의 순서다.
	stacks.POST("/:id/deploy", func(c echo.Context) error { return c.NoContent(http.StatusAccepted) })
	stacks.GET("/:id/status", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	stackhandler.NewReleaseValuesHandler(uc, nil).RegisterRoutes(stacks)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stacks/stk_1/releases", nil))

	require.Equal(t, http.StatusOK, rec.Code, "스택 ID 를 못 읽으면 404/500 이 된다: %s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "harbor")
}

func TestReleaseValuesHandler_ListReleases(t *testing.T) {
	e, _ := newReleaseValuesServer(t)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stacks/stk_1/releases", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Releases []port.ReleaseInfo `json:"releases"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Releases, 1)
	require.Equal(t, "installing_harbor", body.Releases[0].StepName)
}

func TestReleaseValuesHandler_GetValues_DefaultsToLiveMode(t *testing.T) {
	e, _ := newReleaseValuesServer(t)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stacks/stk_1/releases/harbor/values", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var body usecase.ReleaseValuesOutput
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, usecase.ReleaseValuesModeLive, body.Mode)
	require.Contains(t, body.YAML, "externalURL")
	require.Contains(t, body.ProtectedPaths, "externalURL")
}

func TestReleaseValuesHandler_Preview_DoesNotTouchCluster(t *testing.T) {
	e, manager := newReleaseValuesServer(t)

	req := httptest.NewRequest(http.MethodPost, "/stacks/stk_1/releases/harbor/values/preview",
		strings.NewReader(`{"mode":"override","yaml":"trivy:\n  enabled: false\n"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body usecase.ApplyReleaseValuesOutput
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.DryRun)
	require.Len(t, manager.upgrades, 1)
	require.True(t, manager.upgrades[0].DryRun)
}

func TestReleaseValuesHandler_Apply(t *testing.T) {
	e, manager := newReleaseValuesServer(t)

	req := httptest.NewRequest(http.MethodPut, "/stacks/stk_1/releases/harbor/values",
		strings.NewReader(`{"mode":"override","yaml":"trivy:\n  enabled: false\n"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body usecase.ApplyReleaseValuesOutput
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.False(t, body.DryRun)
	require.Equal(t, 3, body.Revision)
	require.Len(t, manager.upgrades, 1)
	require.False(t, manager.upgrades[0].DryRun)
}

// 감사 기록은 "누가 무엇을 바꿨나" 에 답해야 한다. 값 자체는 남기지 않는다 —
// values 에는 사용자가 적어 넣은 자격증명이 들어갈 수 있다.
func TestTopLevelValuePaths_RecordsKeysNotValues(t *testing.T) {
	paths := usecase.TopLevelValuePaths("resources:\n  requests:\n    cpu: 300m\nadminPassword: s3cret\n")

	require.Equal(t, []string{"adminPassword", "resources"}, paths)
	for _, path := range paths {
		require.NotContains(t, path, "s3cret")
		require.NotContains(t, path, "300m")
	}
}

// 깨진 YAML 이 와도 감사 기록 자체가 사라지면 안 된다.
func TestTopLevelValuePaths_InvalidYAMLIsEmpty(t *testing.T) {
	require.Empty(t, usecase.TopLevelValuePaths("resources:\n\tcpu: 1\n"))
}

// 실패한 적용도 감사에 남아야 하지만, Kubernetes 의 패치 오류는 요청 본문을
// 통째로 담아 온다(실측 9.8KB). 앞머리만 남긴다.
func TestReleaseValuesHandler_FailedApplyIsAuditedWithTruncatedError(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	require.NoError(t, stackRepo.Create(context.Background(), &domain.Stack{
		ID: "stk_1", Namespace: "nullus", ClusterID: "cls_1", State: domain.StateCompleted,
		Config: domain.StackConfig{},
	}))

	manager := &stubReleaseManager{upgradeError: errors.New(strings.Repeat("x", 9000))}
	uc := usecase.NewManageReleaseValues(
		stackRepo,
		stubReleaseKubeconfigProvider{},
		func([]byte) port.HelmReleaseManager { return manager },
	)
	recorder := &recordingAuditSink{}

	e := echo.New()
	stackhandler.NewReleaseValuesHandler(uc, recorder).RegisterRoutes(e.Group("/stacks"))

	req := httptest.NewRequest(http.MethodPut, "/stacks/stk_1/releases/harbor/values",
		strings.NewReader(`{"mode":"override","yaml":"trivy:\n  enabled: false\n"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-User-ID", "usr_1")
	req.Header.Set("X-User-Email", "devops@nullus.dev")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Len(t, recorder.entries, 1, "실패한 적용이 감사에 안 남았다")

	entry := recorder.entries[0]
	require.Equal(t, "usr_1", entry.UserID)
	require.Equal(t, "failed", entry.Details["result"])
	require.Equal(t, "devops@nullus.dev", entry.Details["actor"])
	require.Equal(t, []string{"trivy"}, entry.Details["changed_paths"])

	message, _ := entry.Details["error"].(string)
	require.Less(t, len(message), 600, "감사 메시지가 잘리지 않았다: %d bytes", len(message))
	require.Contains(t, message, "truncated")
}

func TestReleaseValuesHandler_UnknownRelease_Returns404(t *testing.T) {
	e, _ := newReleaseValuesServer(t)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stacks/stk_1/releases/nope/values", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "RELEASE_NOT_FOUND")
}

func TestReleaseValuesHandler_InvalidYAML_Returns400(t *testing.T) {
	e, _ := newReleaseValuesServer(t)

	req := httptest.NewRequest(http.MethodPut, "/stacks/stk_1/releases/harbor/values",
		strings.NewReader(`{"mode":"live","yaml":"trivy:\n\tenabled: false\n"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "RELEASE_VALUES_INVALID_YAML")
}

func TestReleaseValuesHandler_InvalidMode_Returns400(t *testing.T) {
	e, _ := newReleaseValuesServer(t)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stacks/stk_1/releases/harbor/values?mode=whatever", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "RELEASE_VALUES_INVALID_MODE")
}
