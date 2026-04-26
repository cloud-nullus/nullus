package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// CICDGoldenPathHandler handles HTTP requests for CI/CD Golden Path operations.
type CICDGoldenPathHandler struct {
	goldenPathRepo port.CICDGoldenPathRepository
}

// NewCICDGoldenPathHandler constructs a CICDGoldenPathHandler.
func NewCICDGoldenPathHandler(goldenPathRepo port.CICDGoldenPathRepository) *CICDGoldenPathHandler {
	return &CICDGoldenPathHandler{goldenPathRepo: goldenPathRepo}
}

// RegisterRoutes registers CI/CD Golden Path routes on the given Echo group.
func (h *CICDGoldenPathHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/golden-paths", h.ListGoldenPaths)
	g.GET("/golden-paths/:id", h.GetGoldenPath)
	g.POST("/golden-paths", h.CreateGoldenPath)
	g.PUT("/golden-paths/:id", h.UpdateGoldenPath)
	g.DELETE("/golden-paths/:id", h.DeleteGoldenPath)
}

type cicdGoldenPathRequest struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	Tools                []domain.CICDTool `json:"tools"`
	EstimatedInstallTime int               `json:"estimated_install_time"`
	RecommendedUseCase   string            `json:"recommended_use_case"`
	MinResources         string            `json:"min_resources"`
}

// ListGoldenPaths handles GET /api/v1/cicd/golden-paths.
func (h *CICDGoldenPathHandler) ListGoldenPaths(c echo.Context) error {
	goldenPaths, err := h.goldenPathRepo.List(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "CICD_GOLDEN_PATH_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, goldenPaths)
}

// GetGoldenPath handles GET /api/v1/cicd/golden-paths/:id.
func (h *CICDGoldenPathHandler) GetGoldenPath(c echo.Context) error {
	id := c.Param("id")

	goldenPath, err := h.goldenPathRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "CICD_GOLDEN_PATH_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, goldenPath)
}

// CreateGoldenPath handles POST /api/v1/cicd/golden-paths.
func (h *CICDGoldenPathHandler) CreateGoldenPath(c echo.Context) error {
	var req cicdGoldenPathRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_GOLDEN_PATH_INVALID", err.Error())
	}

	goldenPath := &domain.CICDGoldenPath{
		ID:                   req.ID,
		Name:                 req.Name,
		Description:          req.Description,
		Tools:                req.Tools,
		EstimatedInstallTime: req.EstimatedInstallTime,
		RecommendedUseCase:   req.RecommendedUseCase,
		MinResources:         req.MinResources,
	}

	if err := h.goldenPathRepo.Create(c.Request().Context(), goldenPath); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_GOLDEN_PATH_CREATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusCreated, goldenPath)
}

// UpdateGoldenPath handles PUT /api/v1/cicd/golden-paths/:id.
func (h *CICDGoldenPathHandler) UpdateGoldenPath(c echo.Context) error {
	var req cicdGoldenPathRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_GOLDEN_PATH_INVALID", err.Error())
	}

	goldenPath := &domain.CICDGoldenPath{
		ID:                   c.Param("id"),
		Name:                 req.Name,
		Description:          req.Description,
		Tools:                req.Tools,
		EstimatedInstallTime: req.EstimatedInstallTime,
		RecommendedUseCase:   req.RecommendedUseCase,
		MinResources:         req.MinResources,
	}

	if err := h.goldenPathRepo.Update(c.Request().Context(), goldenPath); err != nil {
		return errorResponse(c, http.StatusNotFound, "CICD_GOLDEN_PATH_UPDATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, goldenPath)
}

// DeleteGoldenPath handles DELETE /api/v1/cicd/golden-paths/:id.
func (h *CICDGoldenPathHandler) DeleteGoldenPath(c echo.Context) error {
	id := c.Param("id")

	if err := h.goldenPathRepo.Delete(c.Request().Context(), id); err != nil {
		return errorResponse(c, http.StatusNotFound, "CICD_GOLDEN_PATH_DELETE_FAILED", err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}
