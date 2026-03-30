package handler

import (
	"net/http"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
)

// CompatibilityHandler handles HTTP requests for compatibility matrix operations.
type CompatibilityHandler struct {
	compatRepo            port.CompatibilityRepository
	validateCompatibility *usecase.ValidateCompatibility
}

// NewCompatibilityHandler constructs a CompatibilityHandler.
func NewCompatibilityHandler(
	compatRepo port.CompatibilityRepository,
	validateCompatibility *usecase.ValidateCompatibility,
) *CompatibilityHandler {
	return &CompatibilityHandler{
		compatRepo:            compatRepo,
		validateCompatibility: validateCompatibility,
	}
}

// RegisterRoutes registers compatibility routes on the given Echo group.
func (h *CompatibilityHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/compatibility", h.GetMatrix)
	g.POST("/:stackId/validate", h.Validate)
}

// GetMatrix handles GET /api/v1/compatibility/matrix.
func (h *CompatibilityHandler) GetMatrix(c echo.Context) error {
	matrices, err := h.compatRepo.GetAll(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "COMPATIBILITY_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, matrices)
}

// validateRequest is the request body for POST /compatibility/validate.
type validateRequest struct {
	// Tools maps tool category to tool name.
	Tools map[string]string `json:"tools"`
}

// Validate handles POST /api/v1/compatibility/validate.
func (h *CompatibilityHandler) Validate(c echo.Context) error {
	var req validateRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "COMPATIBILITY_REQUEST_INVALID", err.Error())
	}

	out, err := h.validateCompatibility.Execute(c.Request().Context(), usecase.ValidateCompatibilityInput{
		Tools: req.Tools,
	})
	if err != nil {
		return errorResponse(c, http.StatusBadRequest, "COMPATIBILITY_REQUEST_INVALID", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"compatible": out.Compatible,
		"matrix":     out.Matrix,
		"message":    out.Message,
		"overall":    out.Overall,
		"issues":     out.Issues,
		"checkedAt":  out.CheckedAt.Format(time.RFC3339),
	})
}
