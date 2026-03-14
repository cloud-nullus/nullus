package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
)

// HistoryHandler handles HTTP requests for stack version history operations.
type HistoryHandler struct {
	historyRepo   port.HistoryRepository
	stackRepo     port.StackRepository
	manageHistory *usecase.ManageHistory
}

// NewHistoryHandler constructs a HistoryHandler.
func NewHistoryHandler(
	historyRepo port.HistoryRepository,
	stackRepo port.StackRepository,
	manageHistory *usecase.ManageHistory,
) *HistoryHandler {
	return &HistoryHandler{
		historyRepo:   historyRepo,
		stackRepo:     stackRepo,
		manageHistory: manageHistory,
	}
}

// RegisterRoutes registers history routes on the given Echo group.
func (h *HistoryHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/stacks/:id/history", h.ListHistory)
	g.GET("/stacks/:id/history/:versionId", h.GetVersion)
	g.GET("/stacks/:id/history/:versionId/diff", h.GetDiff)
	g.POST("/stacks/:id/rollback/:versionId", h.Rollback)
}

// ListHistory handles GET /api/v1/stacks/:id/history.
func (h *HistoryHandler) ListHistory(c echo.Context) error {
	stackID := c.Param("id")

	out, err := h.manageHistory.ListVersions(c.Request().Context(), usecase.ListVersionsInput{
		StackID: stackID,
	})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "HISTORY_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": out.Versions})
}

// GetVersion handles GET /api/v1/stacks/:id/history/:versionId.
func (h *HistoryHandler) GetVersion(c echo.Context) error {
	stackID := c.Param("id")
	versionID := c.Param("versionId")

	version, err := h.historyRepo.GetVersion(c.Request().Context(), stackID, versionID)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": version})
}

// GetDiff handles GET /api/v1/stacks/:id/history/:versionId/diff.
func (h *HistoryHandler) GetDiff(c echo.Context) error {
	stackID := c.Param("id")
	versionID := c.Param("versionId")

	out, err := h.manageHistory.GetDiff(c.Request().Context(), usecase.GetDiffInput{
		StackID:   stackID,
		VersionID: versionID,
	})
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": out.Diffs})
}

// rollbackRequest is the request body for POST /stacks/:id/rollback/:versionId.
type rollbackRequest struct {
	Reason string `json:"reason"`
}

// Rollback handles POST /api/v1/stacks/:id/rollback/:versionId.
// It loads the target version's config, applies it to the stack, and saves a new history entry.
func (h *HistoryHandler) Rollback(c echo.Context) error {
	stackID := c.Param("id")
	versionID := c.Param("versionId")

	var req rollbackRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ROLLBACK_REQUEST_INVALID", err.Error())
	}

	// Load the target version
	targetVersion, err := h.historyRepo.GetVersion(c.Request().Context(), stackID, versionID)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
	}

	// Apply config to stack
	stack, err := h.stackRepo.GetByID(c.Request().Context(), stackID)
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	}
	stack.Config = targetVersion.Config
	if err := h.stackRepo.Update(c.Request().Context(), stack); err != nil {
		return errorResponse(c, http.StatusInternalServerError, "STACK_UPDATE_FAILED", err.Error())
	}

	// Record the rollback as a new history version
	reason := req.Reason
	if reason == "" {
		reason = "rollback to version " + versionID
	}

	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		userID = "system"
	}

	out, err := h.manageHistory.SaveVersion(c.Request().Context(), usecase.SaveVersionInput{
		StackID:      stackID,
		Config:       targetVersion.Config,
		ChangedBy:    userID,
		ChangeReason: reason,
	})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "HISTORY_SAVE_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"data": out.Version})
}
