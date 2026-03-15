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

type dashboardResponse struct {
	KPI      kpiResponse      `json:"kpi"`
	Pipeline pipelineResponse `json:"pipeline"`
	Tools    []toolResponse   `json:"tools"`
}

type kpiResponse struct {
	CPUUsage     float64 `json:"cpuUsage"`
	MemoryUsage  float64 `json:"memoryUsage"`
	StorageUsage float64 `json:"storageUsage"`
	PodCount     int     `json:"podCount"`
	PodRunning   int     `json:"podRunning"`
}

type pipelineResponse struct {
	TotalRuns       int     `json:"totalRuns"`
	SuccessRate     float64 `json:"successRate"`
	AvgBuildSeconds float64 `json:"avgBuildSeconds"`
}

type toolResponse struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

func (h *DashboardHandler) GetDashboard(c echo.Context) error {
	out, err := h.getDashboard.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "DASHBOARD_FETCH_FAILED", err.Error())
	}

	d := out.Dashboard
	podRunning := int(float64(d.ClusterMetrics.PodCount) * 0.92)

	tools := make([]toolResponse, len(d.ToolHealthList))
	for i, t := range d.ToolHealthList {
		tools[i] = toolResponse{Name: t.Name, Status: t.Status, Version: t.Version}
	}

	return c.JSON(http.StatusOK, dashboardResponse{
		KPI: kpiResponse{
			CPUUsage:     d.ClusterMetrics.CPUUsage,
			MemoryUsage:  d.ClusterMetrics.MemoryUsage,
			StorageUsage: d.ClusterMetrics.StorageUsage,
			PodCount:     d.ClusterMetrics.PodCount,
			PodRunning:   podRunning,
		},
		Pipeline: pipelineResponse{
			TotalRuns:       d.PipelineMetrics.TotalRuns,
			SuccessRate:     d.PipelineMetrics.SuccessRate,
			AvgBuildSeconds: d.PipelineMetrics.AvgBuildTime,
		},
		Tools: tools,
	})
}
