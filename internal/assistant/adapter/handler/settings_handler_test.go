package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
)

type stubSettings struct {
	view       usecase.SettingsView
	err        error
	updated    *usecase.UpdateSettings
	updatedOrg string
	deleted    string
	test       usecase.TestResult
}

func (s *stubSettings) Get(_ context.Context, orgID string) (usecase.SettingsView, error) {
	return s.view, s.err
}
func (s *stubSettings) Update(_ context.Context, orgID, _ string, in usecase.UpdateSettings) (usecase.SettingsView, error) {
	s.updated, s.updatedOrg = &in, orgID
	return s.view, s.err
}
func (s *stubSettings) Delete(_ context.Context, orgID string) error { s.deleted = orgID; return s.err }
func (s *stubSettings) Test(_ context.Context, _ string, in usecase.UpdateSettings) (usecase.TestResult, error) {
	s.updated = &in
	return s.test, s.err
}

type recordingAudit struct{ entries []audit.AuditEntry }

func (r *recordingAudit) Log(_ context.Context, e audit.AuditEntry) error {
	r.entries = append(r.entries, e)
	return nil
}

func serveSettings(t *testing.T, svc *stubSettings, sink *recordingAudit, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	NewSettingsHandler(svc, sink).RegisterRoutes(e.Group("/api/v1/admin"))
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", "org-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// 조회 응답에 키 원문은 없다 — 있는지와 끝 네 자리, 고를 수 있는 공급자만.
func TestSettingsHandler_Get(t *testing.T) {
	svc := &stubSettings{view: usecase.SettingsView{Enabled: true, Provider: "upstage", HasKey: true, KeyHint: "••••1234",
		EffectiveModel: "solar-pro4", Providers: domain.Providers()}}
	rec := serveSettings(t, svc, nil, http.MethodGet, "/api/v1/admin/assistant", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, true, out["has_key"])
	assert.Equal(t, "••••1234", out["key_hint"])
	assert.Equal(t, "solar-pro4", out["effective_model"])
	assert.NotContains(t, out, "api_key")
	assert.Len(t, out["providers"], len(domain.Providers()))
}

// 키를 안 보내면(nil) 저장된 키를 쓰고, 빈 문자열은 지운다. 감사 로그에는 키를 남기지 않는다.
func TestSettingsHandler_PutDistinguishesMissingKeyAndAudits(t *testing.T) {
	svc := &stubSettings{}
	sink := &recordingAudit{}
	rec := serveSettings(t, svc, sink, http.MethodPut, "/api/v1/admin/assistant",
		`{"enabled":true,"provider":"anthropic","model":"claude-sonnet-5-5"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, svc.updated)
	assert.Nil(t, svc.updated.APIKey)
	assert.Equal(t, "org-1", svc.updatedOrg)

	rec = serveSettings(t, svc, sink, http.MethodPut, "/api/v1/admin/assistant",
		`{"enabled":true,"provider":"anthropic","api_key":"sk-ant-secret-xyz"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, svc.updated.APIKey)
	assert.Equal(t, "sk-ant-secret-xyz", *svc.updated.APIKey)

	require.Len(t, sink.entries, 2)
	assert.Equal(t, "update_assistant_settings", sink.entries[1].Action)
	assert.Equal(t, true, sink.entries[1].Details["key_changed"])
	assert.NotContains(t, fmt.Sprint(sink.entries), "sk-ant-secret-xyz")
}

func TestSettingsHandler_Errors(t *testing.T) {
	rec := serveSettings(t, &stubSettings{err: fmt.Errorf("%w: API 키가 필요하다", domain.ErrInvalidSettings)}, nil,
		http.MethodPut, "/api/v1/admin/assistant", `{"enabled":true,"provider":"upstage"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "ASSISTANT_SETTINGS_INVALID")
	assert.Contains(t, rec.Body.String(), "API 키가 필요하다")

	rec = serveSettings(t, &stubSettings{err: fmt.Errorf("%w: ENCRYPTION_KEY", domain.ErrKeyStorageUnavailable)}, nil,
		http.MethodPut, "/api/v1/admin/assistant", `{"enabled":true,"provider":"upstage","api_key":"k"}`)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "ASSISTANT_KEY_STORAGE_UNAVAILABLE")

	rec = serveSettings(t, &stubSettings{}, nil, http.MethodPut, "/api/v1/admin/assistant", `{`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSettingsHandler_TestAndDelete(t *testing.T) {
	svc := &stubSettings{test: usecase.TestResult{OK: false, Model: "solar-pro4", Error: "모델 응답 401: invalid key"}}
	sink := &recordingAudit{}
	rec := serveSettings(t, svc, sink, http.MethodPost, "/api/v1/admin/assistant/test", `{"provider":"upstage","api_key":"k"}`)
	require.Equal(t, http.StatusOK, rec.Code, "실패해도 200 — 결과는 본문으로")
	assert.JSONEq(t, `{"ok":false,"model":"solar-pro4","error":"모델 응답 401: invalid key","latency_ms":0}`, rec.Body.String())
	// 시험은 바꾸는 것이 없어도 저장된 키를 바깥으로 보내는 일이라 남긴다 — 키는 빼고.
	require.Len(t, sink.entries, 1)
	assert.Equal(t, "test_assistant_settings", sink.entries[0].Action)
	assert.Equal(t, false, sink.entries[0].Details["ok"])
	assert.NotContains(t, fmt.Sprint(sink.entries), `"k"`)

	rec = serveSettings(t, svc, sink, http.MethodDelete, "/api/v1/admin/assistant", "")
	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "org-1", svc.deleted)
	require.Len(t, sink.entries, 2)
	assert.Equal(t, "delete_assistant_settings", sink.entries[1].Action)
}
