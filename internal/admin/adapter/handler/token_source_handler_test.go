package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
)

func newTokenSourceEcho(h *TokenSourceHandler) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler
	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)
	return e
}

func TestTokenSourceHandler_ListSources_200(t *testing.T) {
	t.Parallel()
	h := &TokenSourceHandler{}
	h.listSourcesFn = func(_ context.Context, orgID string) ([]tokenSource, error) {
		assert.Equal(t, "org-1", orgID)
		return []tokenSource{{ID: "ts-1", OrgID: "org-1", Provider: "github", Status: "healthy"}}, nil
	}
	e := newTokenSourceEcho(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/token-sources", nil)
	req.Header.Set("X-Org-ID", "org-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.EqualValues(t, 1, body["total"])
}

func TestTokenSourceHandler_Rotate_200(t *testing.T) {
	t.Parallel()
	h := &TokenSourceHandler{}
	h.actionFn = func(_ context.Context, tokenSourceID, action, reason string, _ map[string]any) error {
		assert.Equal(t, "ts-1", tokenSourceID)
		assert.Equal(t, "rotate", action)
		assert.Equal(t, "manual trigger", reason)
		return nil
	}
	e := newTokenSourceEcho(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/token-sources/ts-1/rotate", strings.NewReader(`{"reason":"manual trigger"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTokenSourceHandler_Approve_200(t *testing.T) {
	t.Parallel()
	h := &TokenSourceHandler{}
	h.actionFn = func(_ context.Context, tokenSourceID, action, reason string, _ map[string]any) error {
		assert.Equal(t, "ts-1", tokenSourceID)
		assert.Equal(t, "approve", action)
		assert.Equal(t, "manual approve", reason)
		return nil
	}
	e := newTokenSourceEcho(h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/token-sources/ts-1/approve", strings.NewReader(`{"reason":"manual approve"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTokenSourceHandler_ReAuthReveal_200(t *testing.T) {
	t.Parallel()
	h := &TokenSourceHandler{stepUpTTL: time.Minute}
	h.actionFn = func(_ context.Context, _, _, _ string, _ map[string]any) error { return nil }
	h.revealFn = func(_ context.Context, tokenSourceID string) (map[string]any, error) {
		assert.Equal(t, "ts-1", tokenSourceID)
		return map[string]any{"token_value": "stored-in-openbao"}, nil
	}
	e := newTokenSourceEcho(h)

	reAuthReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/token-sources/ts-1/re-auth", strings.NewReader(`{"reason":"security"}`))
	reAuthReq.Header.Set("Content-Type", "application/json")
	reAuthReq.Header.Set("X-User-ID", "user-1")
	reAuthRec := httptest.NewRecorder()
	e.ServeHTTP(reAuthRec, reAuthReq)
	assert.Equal(t, http.StatusOK, reAuthRec.Code)
	var reAuthBody map[string]any
	require.NoError(t, json.Unmarshal(reAuthRec.Body.Bytes(), &reAuthBody))
	token, ok := reAuthBody["step_up_token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, token)

	revealReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/token-sources/ts-1/reveal", strings.NewReader(`{"step_up_token":"`+token+`"}`))
	revealReq.Header.Set("Content-Type", "application/json")
	revealReq.Header.Set("X-User-ID", "user-1")
	revealRec := httptest.NewRecorder()
	e.ServeHTTP(revealRec, revealReq)
	assert.Equal(t, http.StatusOK, revealRec.Code)
}
