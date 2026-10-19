package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type stubUpgrades struct {
	candidates []domain.UpgradeCandidate
	checks     []domain.UpgradeCheck
	run        *domain.UpgradeRun
	err        error
	started    []usecase.StartUpgradeInput
	preflights []string
}

func (s *stubUpgrades) Candidates(context.Context, string) ([]domain.UpgradeCandidate, error) {
	return s.candidates, s.err
}

func (s *stubUpgrades) Preflight(_ context.Context, _, bundleID string) ([]domain.UpgradeCheck, error) {
	s.preflights = append(s.preflights, bundleID)
	return s.checks, s.err
}

func (s *stubUpgrades) Start(_ context.Context, in usecase.StartUpgradeInput) (*domain.UpgradeRun, error) {
	s.started = append(s.started, in)
	return s.run, s.err
}

func (s *stubUpgrades) GetRun(context.Context, string, string) (*domain.UpgradeRun, error) {
	return s.run, s.err
}

type recordingAudit struct{ entries []audit.AuditEntry }

func (r *recordingAudit) Log(_ context.Context, e audit.AuditEntry) error {
	r.entries = append(r.entries, e)
	return nil
}

func serveUpgrade(t *testing.T, svc *stubUpgrades, sink AuditSink, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	NewUpgradeHandler(svc, sink).RegisterRoutes(e.Group("/api/v1/stacks"))
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestUpgradeHandler_ListCandidates(t *testing.T) {
	svc := &stubUpgrades{candidates: []domain.UpgradeCandidate{{Tool: "argocd", CurrentChartVersion: "7.7.16", CheckedAt: time.Now()}}}
	rec := serveUpgrade(t, svc, nil, http.MethodGet, "/api/v1/stacks/stk_1/upgrades", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Items []domain.UpgradeCandidate `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Items, 1)
	assert.Equal(t, "7.7.16", out.Items[0].CurrentChartVersion)
}

func TestUpgradeHandler_Preflight(t *testing.T) {
	svc := &stubUpgrades{checks: []domain.UpgradeCheck{domain.PassedCheck("backup", "ok")}}
	rec := serveUpgrade(t, svc, nil, http.MethodPost, "/api/v1/stacks/stk_1/upgrades/preflight", `{"bundle_id":"b-20"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"b-20"}, svc.preflights)
	assert.Contains(t, rec.Body.String(), `"checks"`)

	rec = serveUpgrade(t, svc, nil, http.MethodPost, "/api/v1/stacks/stk_1/upgrades/preflight", `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// 실행은 백그라운드로 이어지므로 202 다. 누가 · 왜 · 무엇을 올렸는지 감사 로그에 남긴다.
func TestUpgradeHandler_Start_AcceptedAndAudited(t *testing.T) {
	run := &domain.UpgradeRun{ID: "run-1", StackID: "stk_1", BundleID: "b-20", Status: domain.UpgradeStatusUpgrading,
		SourceChartVersion: "7.7.16", TargetChartVersion: "7.7.20", Reason: "CVE 패치", IdempotencyKey: "k1"}
	svc := &stubUpgrades{run: run}
	sink := &recordingAudit{}
	rec := serveUpgrade(t, svc, sink, http.MethodPost, "/api/v1/stacks/stk_1/upgrade-runs",
		`{"bundle_id":"b-20","reason":"CVE 패치"}`, map[string]string{"Idempotency-Key": "k1"})

	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	require.Len(t, svc.started, 1)
	assert.Equal(t, usecase.StartUpgradeInput{StackID: "stk_1", BundleID: "b-20", Reason: "CVE 패치",
		IdempotencyKey: "k1", RequestedBy: svc.started[0].RequestedBy}, svc.started[0])
	assert.NotContains(t, rec.Body.String(), "k1", "Idempotency-Key 는 응답에 싣지 않는다")

	require.Len(t, sink.entries, 1)
	e := sink.entries[0]
	assert.Equal(t, "start_stack_upgrade", e.Action)
	assert.Equal(t, "stack", e.ResourceType)
	assert.Equal(t, "stk_1", e.ResourceID)
	assert.Equal(t, "CVE 패치", e.Details["reason"])
	assert.Equal(t, "7.7.20", e.Details["to"])
	assert.Equal(t, domain.UpgradeStatusUpgrading, e.Details["status"])
}

// 막힌 시도는 실행이 만들어졌으므로 200 으로 돌려준다(본문의 status 가 blocked).
func TestUpgradeHandler_Start_BlockedRunIsOK(t *testing.T) {
	svc := &stubUpgrades{run: &domain.UpgradeRun{ID: "run-1", Status: domain.UpgradeStatusBlocked}}
	rec := serveUpgrade(t, svc, nil, http.MethodPost, "/api/v1/stacks/stk_1/upgrade-runs",
		`{"bundle_id":"b-20","reason":"x"}`, map[string]string{"Idempotency-Key": "k1"})
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpgradeHandler_Start_RequiresIdempotencyKey(t *testing.T) {
	svc := &stubUpgrades{}
	rec := serveUpgrade(t, svc, nil, http.MethodPost, "/api/v1/stacks/stk_1/upgrade-runs", `{"bundle_id":"b-20","reason":"x"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, svc.started)
}

func TestUpgradeHandler_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{usecase.ErrStackNotFound, http.StatusNotFound},
		{domain.ErrUpgradeBundleNotFound, http.StatusNotFound},
		{domain.ErrUpgradeRunNotFound, http.StatusNotFound},
		{domain.ErrUpgradeReleaseNotFound, http.StatusNotFound},
		{domain.ErrUpgradeInProgress, http.StatusConflict},
		{domain.ErrUpgradeReasonRequired, http.StatusBadRequest},
		{domain.ErrUpgradeIdempotencyKeyRequired, http.StatusBadRequest},
		{domain.ErrUpgradeIdempotencyKeyInvalid, http.StatusBadRequest},
		{domain.ErrUpgradeIdempotencyKeyReused, http.StatusUnprocessableEntity},
		{errors.New("kubeconfig 를 읽지 못했다"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		svc := &stubUpgrades{err: tc.err}
		rec := serveUpgrade(t, svc, nil, http.MethodGet, "/api/v1/stacks/stk_1/upgrade-runs/run-1", "", nil)
		assert.Equal(t, tc.code, rec.Code, tc.err.Error())
	}
}

// 화면은 fixes_cves(번들이 고치는 것)와 fixes_found(그중 이 스택에서 발견된 것)를 읽는다.
func TestUpgradeHandler_ListCandidates_CarriesCVEFixes(t *testing.T) {
	svc := &stubUpgrades{candidates: []domain.UpgradeCandidate{{
		Tool: "argocd", CurrentChartVersion: "7.7.16", CheckedAt: time.Now(),
		Bundle:     &domain.UpgradeBundle{ID: "b-20", FixesCVEs: []string{"CVE-A", "CVE-B"}},
		FixesFound: []string{"CVE-A"}, FixesChecked: true,
	}}}
	rec := serveUpgrade(t, svc, nil, http.MethodGet, "/api/v1/stacks/stk_1/upgrades", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Items []struct {
			Bundle struct {
				FixesCVEs []string `json:"fixes_cves"`
			} `json:"bundle"`
			FixesFound   []string `json:"fixes_found"`
			FixesChecked bool     `json:"fixes_checked"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Items, 1)
	assert.Equal(t, []string{"CVE-A", "CVE-B"}, out.Items[0].Bundle.FixesCVEs)
	assert.Equal(t, []string{"CVE-A"}, out.Items[0].FixesFound)
	assert.True(t, out.Items[0].FixesChecked)
}
