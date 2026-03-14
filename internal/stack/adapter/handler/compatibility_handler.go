package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
)

// CompatibilityHandler handles HTTP requests for compatibility matrix operations.
type CompatibilityHandler struct {
	compatRepo          port.CompatibilityRepository
	validateCompatibility *usecase.ValidateCompatibility
}

// NewCompatibilityHandler constructs a CompatibilityHandler.
func NewCompatibilityHandler(
	compatRepo port.CompatibilityRepository,
	validateCompatibility *usecase.ValidateCompatibility,
) *CompatibilityHandler {
	return &CompatibilityHandler{
		compatRepo:          compatRepo,
		validateCompatibility: validateCompatibility,
	}
}

// RegisterRoutes registers compatibility routes on the given Echo group.
func (h *CompatibilityHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/compatibility/matrix", h.GetMatrix)
	g.POST("/compatibility/validate", h.Validate)
}

// GetMatrix handles GET /api/v1/compatibility/matrix.
func (h *CompatibilityHandler) GetMatrix(c echo.Context) error {
	matrices, err := h.compatRepo.GetAll(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "COMPATIBILITY_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"data": matrices})
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

	status := http.StatusOK
	if !out.Compatible {
		status = http.StatusUnprocessableEntity
	}

	return c.JSON(status, map[string]any{
		"data": map[string]any{
			"compatible": out.Compatible,
			"matrix":     out.Matrix,
			"message":    out.Message,
		},
	})
}
