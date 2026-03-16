package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
	"github.com/labstack/echo/v4"
)

// CICDTemplateHandler handles HTTP requests for CI/CD pipeline template operations.
type CICDTemplateHandler struct {
	templateRepo port.PipelineTemplateRepository
}

// NewCICDTemplateHandler constructs a CICDTemplateHandler.
func NewCICDTemplateHandler(templateRepo port.PipelineTemplateRepository) *CICDTemplateHandler {
	return &CICDTemplateHandler{templateRepo: templateRepo}
}

// RegisterRoutes registers CI/CD template routes on the given Echo group.
func (h *CICDTemplateHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/templates", h.ListTemplates)
	g.GET("/templates/:id", h.GetTemplate)
	g.POST("/templates", h.CreateTemplate)
	g.PUT("/templates/:id", h.UpdateTemplate)
	g.DELETE("/templates/:id", h.DeleteTemplate)
}

type cicdTemplateRequest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AppType     string   `json:"app_type"`
	Stages      []string `json:"stages"`
	CreatedBy   string   `json:"created_by"`
}

// ListTemplates handles GET /api/v1/cicd/templates.
func (h *CICDTemplateHandler) ListTemplates(c echo.Context) error {
	templates, err := h.templateRepo.List(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "CICD_TEMPLATE_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, templates)
}

// GetTemplate handles GET /api/v1/cicd/templates/:id.
func (h *CICDTemplateHandler) GetTemplate(c echo.Context) error {
	id := c.Param("id")

	tmpl, err := h.templateRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "CICD_TEMPLATE_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, tmpl)
}

// CreateTemplate handles POST /api/v1/cicd/templates.
func (h *CICDTemplateHandler) CreateTemplate(c echo.Context) error {
	var req cicdTemplateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_TEMPLATE_INVALID", err.Error())
	}

	tmpl := &domain.PipelineTemplate{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		AppType:     domain.AppType(req.AppType),
		Stages:      req.Stages,
		CreatedBy:   req.CreatedBy,
	}

	if err := h.templateRepo.Create(c.Request().Context(), tmpl); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_TEMPLATE_CREATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusCreated, tmpl)
}

// UpdateTemplate handles PUT /api/v1/cicd/templates/:id.
func (h *CICDTemplateHandler) UpdateTemplate(c echo.Context) error {
	var req cicdTemplateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_TEMPLATE_INVALID", err.Error())
	}

	tmpl := &domain.PipelineTemplate{
		ID:          c.Param("id"),
		Name:        req.Name,
		Description: req.Description,
		AppType:     domain.AppType(req.AppType),
		Stages:      req.Stages,
		CreatedBy:   req.CreatedBy,
	}

	if err := h.templateRepo.Update(c.Request().Context(), tmpl); err != nil {
		return errorResponse(c, http.StatusNotFound, "CICD_TEMPLATE_UPDATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, tmpl)
}

// DeleteTemplate handles DELETE /api/v1/cicd/templates/:id.
func (h *CICDTemplateHandler) DeleteTemplate(c echo.Context) error {
	id := c.Param("id")

	if err := h.templateRepo.Delete(c.Request().Context(), id); err != nil {
		return errorResponse(c, http.StatusNotFound, "CICD_TEMPLATE_DELETE_FAILED", err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}
