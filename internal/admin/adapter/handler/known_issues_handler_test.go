package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKnownIssuesHandler_ListKnownIssues_200(t *testing.T) {
	t.Parallel()

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	h := &KnownIssuesHandler{}
	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/known-issues", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	items, ok := resp["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 3)

	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "KI-001", first["id"])
	assert.Equal(t, "medium", first["severity"])
	assert.Equal(t, "open", first["status"])
}
