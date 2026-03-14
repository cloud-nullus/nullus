package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
)

// TemplateHandler handles HTTP requests for Golden Path template operations.
type TemplateHandler struct {
	getTemplate   *usecase.GetTemplate
	listTemplates *usecase.ListTemplates
}

// NewTemplateHandler constructs a TemplateHandler.
func NewTemplateHandler(getTemplate *usecase.GetTemplate, listTemplates *usecase.ListTemplates) *TemplateHandler {
	return &TemplateHandler{
		getTemplate:   getTemplate,
		listTemplates: listTemplates,
	}
}

// RegisterRoutes registers template routes on the given Echo group.
func (h *TemplateHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/templates", h.ListTemplates)
	g.GET("/templates/:id", h.GetTemplate)
}

// ListTemplates handles GET /api/v1/templates.
func (h *TemplateHandler) ListTemplates(c echo.Context) error {
	out, err := h.listTemplates.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "TEMPLATE_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": out.Templates})
}

// GetTemplate handles GET /api/v1/templates/:id.
func (h *TemplateHandler) GetTemplate(c echo.Context) error {
	id := c.Param("id")

	out, err := h.getTemplate.Execute(c.Request().Context(), usecase.GetTemplateInput{ID: id})
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": out.Template})
}
