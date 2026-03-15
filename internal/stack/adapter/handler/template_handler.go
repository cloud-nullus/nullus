package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
)

// TemplateHandler handles HTTP requests for Golden Path template operations.
type TemplateHandler struct {
	getTemplate   *usecase.GetTemplate
	listTemplates *usecase.ListTemplates
	templateRepo  port.TemplateRepository
}

// NewTemplateHandler constructs a TemplateHandler.
func NewTemplateHandler(
	getTemplate *usecase.GetTemplate,
	listTemplates *usecase.ListTemplates,
	templateRepo port.TemplateRepository,
) *TemplateHandler {
	return &TemplateHandler{
		getTemplate:   getTemplate,
		listTemplates: listTemplates,
		templateRepo:  templateRepo,
	}
}

// RegisterRoutes registers template routes on the given Echo group.
func (h *TemplateHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/templates", h.ListTemplates)
	g.GET("/templates/:id", h.GetTemplate)
	g.POST("/templates", h.CreateTemplate)
	g.PUT("/templates/:id", h.UpdateTemplate)
	g.DELETE("/templates/:id", h.DeleteTemplate)
}

type templateRequest struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Tools                json.RawMessage `json:"tools"`
	EstimatedInstallTime int64           `json:"estimated_install_time"`
	RecommendedUseCase   string          `json:"recommended_use_case"`
	MinResources         string          `json:"min_resources"`
}

// ListTemplates handles GET /api/v1/templates.
func (h *TemplateHandler) ListTemplates(c echo.Context) error {
	out, err := h.listTemplates.Execute(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "TEMPLATE_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, out.Templates)
}

// GetTemplate handles GET /api/v1/templates/:id.
func (h *TemplateHandler) GetTemplate(c echo.Context) error {
	id := c.Param("id")

	out, err := h.getTemplate.Execute(c.Request().Context(), usecase.GetTemplateInput{ID: id})
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, out.Template)
}

func (h *TemplateHandler) CreateTemplate(c echo.Context) error {
	var req templateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "TEMPLATE_INVALID", err.Error())
	}

	template, err := templateFromRequest(req)
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "TEMPLATE_INVALID", err.Error())
	}

	if err := h.templateRepo.Create(c.Request().Context(), template); err != nil {
		return errorResponse(c, http.StatusBadRequest, "TEMPLATE_CREATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusCreated, template)
}

func (h *TemplateHandler) UpdateTemplate(c echo.Context) error {
	var req templateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "TEMPLATE_INVALID", err.Error())
	}

	template, err := templateFromRequest(req)
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "TEMPLATE_INVALID", err.Error())
	}
	template.ID = c.Param("id")

	if err := h.templateRepo.Update(c.Request().Context(), template); err != nil {
		return errorResponse(c, http.StatusNotFound, "TEMPLATE_UPDATE_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, template)
}

func (h *TemplateHandler) DeleteTemplate(c echo.Context) error {
	id := c.Param("id")

	if err := h.templateRepo.Delete(c.Request().Context(), id); err != nil {
		return errorResponse(c, http.StatusNotFound, "TEMPLATE_DELETE_FAILED", err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

func templateFromRequest(req templateRequest) (*domain.Template, error) {
	tools := make([]domain.ToolConfig, 0)
	if len(req.Tools) > 0 {
		if err := json.Unmarshal(req.Tools, &tools); err != nil {
			return nil, err
		}
	}

	return &domain.Template{
		ID:                   req.ID,
		Name:                 req.Name,
		Description:          req.Description,
		Tools:                tools,
		EstimatedInstallTime: time.Duration(req.EstimatedInstallTime),
		RecommendedUseCase:   req.RecommendedUseCase,
		MinResources:         req.MinResources,
	}, nil
}
