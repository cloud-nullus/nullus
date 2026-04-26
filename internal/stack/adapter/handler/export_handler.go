package handler

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// ExportHandler handles stack configuration export requests.
type ExportHandler struct {
	exportConfig *usecase.ExportConfig
}

// NewExportHandler constructs an ExportHandler.
func NewExportHandler(exportConfig *usecase.ExportConfig) *ExportHandler {
	return &ExportHandler{exportConfig: exportConfig}
}

// RegisterRoutes registers export routes on the given Echo group.
func (h *ExportHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/stacks/:id/export", h.ExportStack)
}

// ExportStack handles GET /api/v1/stacks/:id/export?format=json|yaml.
func (h *ExportHandler) ExportStack(c echo.Context) error {
	id := c.Param("id")
	format := c.QueryParam("format")
	if format == "" {
		format = "json"
	}

	ctx := c.Request().Context()

	switch format {
	case "json":
		data, err := h.exportConfig.ExportAsJSON(ctx, id)
		if err != nil {
			return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
		}
		filename := fmt.Sprintf("stack-%s.json", id)
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		return c.Blob(http.StatusOK, "application/json", data)

	case "yaml":
		data, err := h.exportConfig.ExportAsYAML(ctx, id)
		if err != nil {
			return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
		}
		filename := fmt.Sprintf("stack-%s.yaml", id)
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		return c.Blob(http.StatusOK, "application/x-yaml", data)

	default:
		return errorResponse(c, http.StatusBadRequest, "INVALID_FORMAT", fmt.Sprintf("unsupported format %q, use json or yaml", format))
	}
}
