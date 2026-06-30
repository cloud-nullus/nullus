package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// ExportHandler handles stack configuration export requests.
type ExportHandler struct {
	exportConfig *usecase.ExportConfig
	importConfig *usecase.ImportConfig
}

// NewExportHandler constructs an ExportHandler.
func NewExportHandler(exportConfig *usecase.ExportConfig, importConfig *usecase.ImportConfig) *ExportHandler {
	return &ExportHandler{exportConfig: exportConfig, importConfig: importConfig}
}

// RegisterRoutes registers export routes on the given Echo group.
func (h *ExportHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/stacks/:id/export", h.ExportStack)
	g.POST("/stacks/import/preview", h.PreviewImport)
	g.POST("/stacks/import", h.ImportStack)
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

// ImportStack handles POST /api/v1/stacks/import.
func (h *ExportHandler) ImportStack(c echo.Context) error {
	if h.importConfig == nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_IMPORT_FAILED", "import usecase not configured")
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "STACK_IMPORT_INVALID", err.Error())
	}
	replaceExisting := strings.EqualFold(c.QueryParam("replace_existing"), "true")

	out, err := h.importConfig.Execute(c.Request().Context(), usecase.ImportConfigInput{
		OrgID:   resolveOrgID(c),
		Payload: body,
		ReplaceExisting: replaceExisting,
	})
	if err != nil {
		if err == usecase.ErrImportConfirmationRequired {
			return errorResponse(c, http.StatusConflict, "STACK_IMPORT_CONFIRM_REQUIRED", err.Error())
		}
		return errorResponse(c, http.StatusBadRequest, "STACK_IMPORT_INVALID", err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]any{"id": out.Stack.ID})
}

// PreviewImport handles POST /api/v1/stacks/import/preview.
func (h *ExportHandler) PreviewImport(c echo.Context) error {
	if h.importConfig == nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_IMPORT_FAILED", "import usecase not configured")
	}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "STACK_IMPORT_INVALID", err.Error())
	}
	out, err := h.importConfig.Preview(c.Request().Context(), usecase.ImportConfigInput{
		OrgID:   resolveOrgID(c),
		Payload: body,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "STACK_IMPORT_INVALID", err.Error())
	}
	return c.JSON(http.StatusOK, out)
}
