package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/observability/usecase"
	"github.com/labstack/echo/v4"
)

// DashboardHandler handles HTTP requests for observability dashboard operations.
type DashboardHandler struct {
	getDashboard *usecase.GetDashboard
}

// NewDashboardHandler constructs a DashboardHandler.
func NewDashboardHandler(getDashboard *usecase.GetDashboard) *DashboardHandler {
	return &DashboardHandler{getDashboard: getDashboard}
}

// RegisterRoutes registers dashboard routes on the given Echo group.
func (h *DashboardHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/monitoring/dashboard", h.GetDashboard)
	g.GET("/monitoring/metrics/summary", h.GetMetricsSummary)
}

// GetDashboard handles GET /api/v1/monitoring/dashboard.
func (h *DashboardHandler) GetDashboard(c echo.Context) error {
	out, err := h.getDashboard.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "DASHBOARD_FETCH_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"data": out.Dashboard})
}

// GetMetricsSummary handles GET /api/v1/monitoring/metrics/summary.
func (h *DashboardHandler) GetMetricsSummary(c echo.Context) error {
	out, err := h.getDashboard.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "METRICS_FETCH_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"data": map[string]any{
			"cluster_metrics":  out.Dashboard.ClusterMetrics,
			"pipeline_metrics": out.Dashboard.PipelineMetrics,
		},
	})
}
