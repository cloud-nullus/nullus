package handler

import (
	"net/http"

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
	g.GET("/cicd/templates", h.ListTemplates)
	g.GET("/cicd/templates/:id", h.GetTemplate)
}

// ListTemplates handles GET /api/v1/cicd/templates.
func (h *CICDTemplateHandler) ListTemplates(c echo.Context) error {
	templates, err := h.templateRepo.List(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "CICD_TEMPLATE_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"data": templates})
}

// GetTemplate handles GET /api/v1/cicd/templates/:id.
func (h *CICDTemplateHandler) GetTemplate(c echo.Context) error {
	id := c.Param("id")

	tmpl, err := h.templateRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "CICD_TEMPLATE_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": tmpl})
}
