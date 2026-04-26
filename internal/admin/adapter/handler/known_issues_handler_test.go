package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/admin/port"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
)

type mockKnownIssuesRepository struct {
	listFn func(ctx context.Context) ([]port.KnownIssue, error)
}

func (m *mockKnownIssuesRepository) List(ctx context.Context) ([]port.KnownIssue, error) {
	return m.listFn(ctx)
}

func TestKnownIssuesHandler_ListKnownIssues_200(t *testing.T) {
	t.Parallel()

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	h := NewKnownIssuesHandler(&mockKnownIssuesRepository{
		listFn: func(ctx context.Context) ([]port.KnownIssue, error) {
			return []port.KnownIssue{
				{
					ID:          "KI-001",
					Severity:    "medium",
					Title:       "Helm install requires cluster admin",
					Description: "Helm-based stack installation currently requires cluster-admin role to create CRDs and cluster-scoped resources.",
					Workaround:  "Use a temporary cluster-admin service account during installation, then rotate to least-privilege RBAC.",
					Status:      "open",
				},
			}, nil
		},
	})
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
	require.Len(t, items, 1)

	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "KI-001", first["id"])
	assert.Equal(t, "medium", first["severity"])
	assert.Equal(t, "open", first["status"])
}

func TestKnownIssuesHandler_ListKnownIssues_500(t *testing.T) {
	t.Parallel()

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	h := NewKnownIssuesHandler(&mockKnownIssuesRepository{
		listFn: func(ctx context.Context) ([]port.KnownIssue, error) {
			return nil, errors.New("db unavailable")
		},
	})
	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/known-issues", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
