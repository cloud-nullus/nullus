package handler

import (
	"net/http"

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
	g.POST("/resources/estimate", h.Estimate)
}

// estimateRequest is the request body for POST /resources/estimate.
type estimateRequest struct {
	Tools    []toolInstanceRequest `json:"tools"`
	Workload workloadRequest       `json:"workload"`
}

type toolInstanceRequest struct {
	Name      string `json:"name"`
	Instances int    `json:"instances"`
}

type workloadRequest struct {
	Developers        int    `json:"developers"`
	ConcurrentRunners int    `json:"concurrent_runners"`
	WeeklyCommits     int    `json:"weekly_commits"`
	BuildFrequency    string `json:"build_frequency"`
}

// Estimate handles POST /api/v1/resources/estimate.
func (h *ResourceHandler) Estimate(c echo.Context) error {
	var req estimateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "RESOURCE_INVALID_VALUE", err.Error())
	}

	tools := make([]usecase.ToolInstance, len(req.Tools))
	for i, t := range req.Tools {
		tools[i] = usecase.ToolInstance{Name: t.Name, Instances: t.Instances}
	}

	out, err := h.calculateResources.Execute(c.Request().Context(), usecase.EstimateResourcesInput{
		Tools: tools,
		Workload: usecase.WorkloadInput{
			Developers:        req.Workload.Developers,
			ConcurrentRunners: req.Workload.ConcurrentRunners,
			WeeklyCommits:     req.Workload.WeeklyCommits,
			BuildFrequency:    req.Workload.BuildFrequency,
		},
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "RESOURCE_INVALID_VALUE", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": map[string]any{
			"summary":  out.Summary,
			"per_tool": out.PerTool,
			"scaling_notes": out.Notes,
		},
	})
}
