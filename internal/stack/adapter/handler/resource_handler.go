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
	listDefaultsUC     *usecase.ListResourceDefaults
	upsertResourceUC   *usecase.UpsertResourceDefault
}

// NewResourceHandler constructs a ResourceHandler.
func NewResourceHandler(
	calculateResources *usecase.CalculateResources,
	listDefaultsUC *usecase.ListResourceDefaults,
	upsertResourceUC *usecase.UpsertResourceDefault,
) *ResourceHandler {
	return &ResourceHandler{
		calculateResources: calculateResources,
		listDefaultsUC:     listDefaultsUC,
		upsertResourceUC:   upsertResourceUC,
	}
}

// RegisterRoutes registers resource routes on the given Echo group.
func (h *ResourceHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/estimate", h.Estimate)
	g.GET("/resource-defaults", h.ListResourceDefaults)
	g.POST("/resource-defaults", h.UpsertResourceDefault)
}

type upsertResourceDefaultRequest struct {
	ToolKey          string  `json:"tool_key"`
	DisplayName      string  `json:"display_name"`
	CPURequest       float64 `json:"cpu_request"`
	CPULimit         float64 `json:"cpu_limit"`
	MemoryRequestGi  float64 `json:"memory_request_gi"`
	MemoryLimitGi    float64 `json:"memory_limit_gi"`
	StorageRequestGi float64 `json:"storage_request_gi"`
	StorageLimitGi   float64 `json:"storage_limit_gi"`
	IsDefault        bool    `json:"is_default"`
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

func (h *ResourceHandler) ListResourceDefaults(c echo.Context) error {
	out, err := h.listDefaultsUC.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "RESOURCE_DEFAULT_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"items": out.Items,
		"total": len(out.Items),
	})
}

func (h *ResourceHandler) UpsertResourceDefault(c echo.Context) error {
	var req upsertResourceDefaultRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "RESOURCE_DEFAULT_INVALID", err.Error())
	}

	out, err := h.upsertResourceUC.Execute(c.Request().Context(), usecase.UpsertResourceDefaultInput{
		ToolKey:          req.ToolKey,
		DisplayName:      req.DisplayName,
		CPURequest:       req.CPURequest,
		CPULimit:         req.CPULimit,
		MemoryRequestGi:  req.MemoryRequestGi,
		MemoryLimitGi:    req.MemoryLimitGi,
		StorageRequestGi: req.StorageRequestGi,
		StorageLimitGi:   req.StorageLimitGi,
		IsDefault:        req.IsDefault,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "RESOURCE_DEFAULT_INVALID", err.Error())
	}

	return c.JSON(http.StatusCreated, out.Item)
}
