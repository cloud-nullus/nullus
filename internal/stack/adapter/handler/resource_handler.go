package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
)

// ResourceHandler handles HTTP requests for resource estimation.
type ResourceHandler struct {
	calculateResources *usecase.CalculateResources
}

// NewResourceHandler constructs a ResourceHandler.
func NewResourceHandler(calculateResources *usecase.CalculateResources) *ResourceHandler {
	return &ResourceHandler{calculateResources: calculateResources}
}

// RegisterRoutes registers resource routes on the given Echo group.
func (h *ResourceHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/estimate", h.Estimate)
}

// Estimate handles POST /api/v1/resources/estimate.
func (h *ResourceHandler) Estimate(c echo.Context) error {
	var req domain.StackConfig
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "RESOURCE_INVALID_VALUE", err.Error())
	}

	tools := []usecase.ToolInstance{}
	for _, tool := range []domain.ToolSelection{
		req.Artifacts.PackageRegistry,
		req.Artifacts.SourceRepository,
		req.Artifacts.ContainerRegistry,
		req.Artifacts.StorageBackend,
		req.Pipeline.CIPlatform,
		req.Pipeline.CDTool,
		req.Monitoring.Collection,
		req.Monitoring.Visualization,
		req.Logging.Collection,
		req.Logging.Search,
	} {
		if !tool.Enabled || tool.Name == "" {
			continue
		}
		tools = append(tools, usecase.ToolInstance{Name: tool.Name, Instances: 1})
	}

	out, err := h.calculateResources.Execute(c.Request().Context(), usecase.EstimateResourcesInput{
		Tools: tools,
		Workload: usecase.WorkloadInput{
			Developers:        req.Resources.DevCount,
			ConcurrentRunners: req.Resources.ConcurrentRunners,
			WeeklyCommits:     req.Resources.CommitsPerWeek,
			BuildFrequency:    req.Resources.BuildFrequency,
		},
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "RESOURCE_INVALID_VALUE", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"cpu":     out.Summary.CPUCores,
		"memory":  out.Summary.MemoryGi,
		"storage": out.Summary.StorageGi,
		"cost":    out.Summary.MonthlyCostUSD,
	})
}
