package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	obshandler "github.com/cloud-nullus/draft/internal/observability/adapter/handler"
	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDashboardRepository struct {
	dashboard *domain.Dashboard
	err       error
}

func (m *mockDashboardRepository) GetDashboard(_ context.Context) (*domain.Dashboard, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.dashboard == nil {
		return &domain.Dashboard{}, nil
	}
	copied := *m.dashboard
	copied.ToolHealthList = append([]domain.ToolHealth(nil), m.dashboard.ToolHealthList...)
	return &copied, nil
}

func newDashboardEcho(repo *mockDashboardRepository) *echo.Echo {
	e := echo.New()
	getDashboardUC := usecase.NewGetDashboard(repo)
	h := obshandler.NewDashboardHandler(getDashboardUC)
	v1 := e.Group("/api/v1/monitoring")
	h.RegisterRoutes(v1)
	return e
}

func TestDashboardHandler_Get_Success(t *testing.T) {
	repo := &mockDashboardRepository{dashboard: &domain.Dashboard{
		ClusterMetrics:  domain.ClusterMetrics{CPUUsage: 42.5, MemoryUsage: 60.1, StorageUsage: 33.3, PodCount: 18},
		PipelineMetrics: domain.PipelineMetrics{TotalRuns: 120, SuccessRate: 96.2, AvgBuildTime: 74.5},
		ToolHealthList:  []domain.ToolHealth{{Name: "ArgoCD", Status: "running", Version: "v2.14.0"}},
	}}
	e := newDashboardEcho(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/dashboard", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp domain.Dashboard
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 42.5, resp.ClusterMetrics.CPUUsage)
	assert.Equal(t, 120, resp.PipelineMetrics.TotalRuns)
	require.Len(t, resp.ToolHealthList, 1)
	assert.Equal(t, "ArgoCD", resp.ToolHealthList[0].Name)
}

func TestDashboardHandler_Get_RepoError(t *testing.T) {
	repo := &mockDashboardRepository{err: errors.New("prometheus unavailable")}
	e := newDashboardEcho(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/dashboard", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "DASHBOARD_FETCH_FAILED", resp["error"]["code"])
	assert.Contains(t, resp["error"]["message"], "prometheus unavailable")
}
