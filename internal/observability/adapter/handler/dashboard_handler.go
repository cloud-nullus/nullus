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
	g.GET("/dashboard", h.GetDashboard)
}

// GetDashboard handles GET /api/v1/monitoring/dashboard.
func (h *DashboardHandler) GetDashboard(c echo.Context) error {
	out, err := h.getDashboard.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "DASHBOARD_FETCH_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, out.Dashboard)
}
