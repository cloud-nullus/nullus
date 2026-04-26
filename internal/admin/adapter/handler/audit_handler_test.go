package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
)

func TestAuditHandler_ListAuditLogs_200(t *testing.T) {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	h := &AuditHandler{listFn: func(_ context.Context, _, _ int) ([]audit.AuditEntry, int, error) {
		return []audit.AuditEntry{{UserID: "admin-1", Action: "login"}}, 1, nil
	}}

	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?limit=50&offset=0", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	items, ok := resp["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.EqualValues(t, 1, resp["total"])
}
