package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type bundleStatusChange struct {
	id, status string
	confirmed  bool
}

type stubBundles struct {
	items      []domain.UpgradeBundle
	bundle     *domain.UpgradeBundle
	err        error
	registered []usecase.RegisterUpgradeBundleInput
	changes    []bundleStatusChange
}

func (s *stubBundles) List(context.Context) ([]domain.UpgradeBundle, error) { return s.items, s.err }

func (s *stubBundles) Register(_ context.Context, in usecase.RegisterUpgradeBundleInput) (*domain.UpgradeBundle, error) {
	s.registered = append(s.registered, in)
	return s.bundle, s.err
}

func (s *stubBundles) ChangeStatus(_ context.Context, id, status string, confirmed bool) (*domain.UpgradeBundle, error) {
	s.changes = append(s.changes, bundleStatusChange{id: id, status: status, confirmed: confirmed})
	return s.bundle, s.err
}

func serveBundles(t *testing.T, svc *stubBundles, sink AuditSink, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	NewUpgradeBundleHandler(svc, sink).RegisterRoutes(e.Group("/api/v1/admin"))
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-User-ID", "usr_admin")
	req.Header.Set("X-User-Email", "admin@nullus.local")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func sampleBundle() *domain.UpgradeBundle {
	return &domain.UpgradeBundle{ID: "argocd-7.7.16-7.7.22", Tool: "argocd", ReleaseName: "argo-cd", ChartName: "argo-cd",
		RepoURL: "https://argoproj.github.io/argo-helm", SourceChartVersion: "7.7.16", SourceAppVersion: "v2.13.3",
		TargetChartVersion: "7.7.22", TargetAppVersion: "v2.13.4", RiskLevel: "low", RequiresBackup: true,
		Status: domain.UpgradeBundleVerified}
}

func TestUpgradeBundleHandler_List(t *testing.T) {
	svc := &stubBundles{items: []domain.UpgradeBundle{*sampleBundle()}}
	rec := serveBundles(t, svc, nil, http.MethodGet, "/api/v1/admin/upgrade-bundles", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Items []domain.UpgradeBundle `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Items, 1)
	assert.Equal(t, "argocd-7.7.16-7.7.22", out.Items[0].ID)
}

func TestUpgradeBundleHandler_List_EmptyIsEmptyArray(t *testing.T) {
	rec := serveBundles(t, &stubBundles{items: []domain.UpgradeBundle{}}, nil, http.MethodGet, "/api/v1/admin/upgrade-bundles", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"items":[]}`, rec.Body.String())
}

func TestUpgradeBundleHandler_Register_CreatedAndAudited(t *testing.T) {
	svc := &stubBundles{bundle: sampleBundle()}
	sink := &recordingAudit{}
	body := `{"tool":"argocd","release_name":"argo-cd","chart_name":"argo-cd","repo_url":"https://argoproj.github.io/argo-helm",
 "source_chart_version":"7.7.16","source_app_version":"v2.13.3","target_chart_version":"7.7.22","target_app_version":"v2.13.4",
 "risk_level":"low","expected_disruption":"약 1~2분 재시작","release_notes":"https://github.com/argoproj/argo-cd/releases/tag/v2.13.4",
 "supported_architectures":["amd64","arm64"],"fixes_cves":["CVE-2025-12345"],"status":"verified","rehearsal_confirmed":true}`
	rec := serveBundles(t, svc, sink, http.MethodPost, "/api/v1/admin/upgrade-bundles", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var out domain.UpgradeBundle
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "argocd-7.7.16-7.7.22", out.ID)

	require.Len(t, svc.registered, 1)
	in := svc.registered[0]
	assert.Equal(t, "argocd", in.Tool)
	assert.Equal(t, "7.7.16", in.SourceChartVersion)
	assert.Equal(t, "7.7.22", in.TargetChartVersion)
	assert.True(t, in.RequiresBackup, "requires_backup 을 비우면 참 — 안전 조건을 기본으로 둔다")
	assert.True(t, in.RehearsalConfirmed)
	assert.Equal(t, domain.UpgradeBundleVerified, in.Status)
	assert.Equal(t, []string{"CVE-2025-12345"}, in.FixesCVEs)

	require.Len(t, sink.entries, 1)
	e := sink.entries[0]
	assert.Equal(t, "register_upgrade_bundle", e.Action)
	assert.Equal(t, "upgrade_bundle", e.ResourceType)
	assert.Equal(t, "argocd-7.7.16-7.7.22", e.ResourceID)
	assert.Equal(t, "usr_admin", e.UserID)
	assert.Equal(t, "admin@nullus.local", e.Details["actor"])
	assert.Equal(t, "7.7.16", e.Details["from"])
	assert.Equal(t, "7.7.22", e.Details["to"])
	assert.Equal(t, true, e.Details["rehearsal_confirmed"])
}

func TestUpgradeBundleHandler_Register_ExplicitRequiresBackupFalse(t *testing.T) {
	svc := &stubBundles{bundle: sampleBundle()}
	body := `{"tool":"argocd","release_name":"argo-cd","chart_name":"argo-cd","repo_url":"https://x","source_chart_version":"1","target_chart_version":"2","risk_level":"low","requires_backup":false}`
	rec := serveBundles(t, svc, nil, http.MethodPost, "/api/v1/admin/upgrade-bundles", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, svc.registered, 1)
	assert.False(t, svc.registered[0].RequiresBackup)
}

func TestUpgradeBundleHandler_Register_InvalidJSON(t *testing.T) {
	svc := &stubBundles{bundle: sampleBundle()}
	sink := &recordingAudit{}
	rec := serveBundles(t, svc, sink, http.MethodPost, "/api/v1/admin/upgrade-bundles", `{"tool":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, svc.registered)
	assert.Empty(t, sink.entries)
}

func TestUpgradeBundleHandler_Register_RejectedIsAudited(t *testing.T) {
	svc := &stubBundles{err: domain.ErrUpgradeBundleRehearsalRequired}
	sink := &recordingAudit{}
	rec := serveBundles(t, svc, sink, http.MethodPost, "/api/v1/admin/upgrade-bundles", `{"tool":"argocd","status":"verified"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	require.Len(t, sink.entries, 1)
	assert.Equal(t, "rejected", sink.entries[0].Details["status"])
}

func TestUpgradeBundleHandler_ChangeStatus(t *testing.T) {
	revoked := sampleBundle()
	revoked.Status = domain.UpgradeBundleRevoked
	svc := &stubBundles{bundle: revoked}
	sink := &recordingAudit{}
	rec := serveBundles(t, svc, sink, http.MethodPatch, "/api/v1/admin/upgrade-bundles/argocd-7.7.16-7.7.22/status", `{"status":"revoked"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out domain.UpgradeBundle
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, domain.UpgradeBundleRevoked, out.Status)
	require.Equal(t, []bundleStatusChange{{id: "argocd-7.7.16-7.7.22", status: "revoked"}}, svc.changes)

	require.Len(t, sink.entries, 1)
	assert.Equal(t, "change_upgrade_bundle_status", sink.entries[0].Action)
	assert.Equal(t, "revoked", sink.entries[0].Details["to_status"])
}

func TestUpgradeBundleHandler_ChangeStatus_PassesConfirmation(t *testing.T) {
	svc := &stubBundles{bundle: sampleBundle()}
	rec := serveBundles(t, svc, nil, http.MethodPatch, "/api/v1/admin/upgrade-bundles/b-1/status", `{"status":"verified","rehearsal_confirmed":true}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []bundleStatusChange{{id: "b-1", status: "verified", confirmed: true}}, svc.changes)
}

func TestUpgradeBundleHandler_ChangeStatus_RequiresStatus(t *testing.T) {
	svc := &stubBundles{bundle: sampleBundle()}
	rec := serveBundles(t, svc, nil, http.MethodPatch, "/api/v1/admin/upgrade-bundles/b-1/status", `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, svc.changes)
}

func TestUpgradeBundleHandler_ErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrUpgradeBundleInvalid, http.StatusBadRequest, "UPGRADE_BUNDLE_INVALID"},
		{domain.ErrUpgradeBundleRehearsalRequired, http.StatusBadRequest, "UPGRADE_BUNDLE_REHEARSAL_REQUIRED"},
		{domain.ErrUpgradeBundleExists, http.StatusConflict, "UPGRADE_BUNDLE_EXISTS"},
		{domain.ErrUpgradeBundleStatusTransition, http.StatusConflict, "UPGRADE_BUNDLE_STATUS_CONFLICT"},
		{domain.ErrUpgradeBundleUnknown, http.StatusNotFound, "UPGRADE_BUNDLE_NOT_FOUND"},
		{errors.New("db down"), http.StatusInternalServerError, "UPGRADE_BUNDLE_FAILED"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			rec := serveBundles(t, &stubBundles{err: c.err}, nil, http.MethodPatch, "/api/v1/admin/upgrade-bundles/b-1/status", `{"status":"revoked"}`)
			assert.Equal(t, c.status, rec.Code)
			var out struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
			assert.Equal(t, c.code, out.Error.Code)
		})
	}
}
