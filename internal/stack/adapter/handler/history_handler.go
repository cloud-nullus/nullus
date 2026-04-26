package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// HistoryHandler handles HTTP requests for stack version history operations.
type HistoryHandler struct {
	historyRepo   port.HistoryRepository
	stackRepo     port.StackRepository
	manageHistory *usecase.ManageHistory
	diffVersions  *usecase.DiffVersions
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
		diffVersions:  usecase.NewDiffVersions(historyRepo),
	}
}

// RegisterRoutes registers history routes on the given Echo group.
func (h *HistoryHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/:stackId/history", h.ListHistory)
	g.GET("/:id/history/diff", h.GetVersionsDiff)
	g.GET("/:stackId/diff", h.GetDiff)
	g.POST("/:stackId/rollback", h.Rollback)
}

func (h *HistoryHandler) GetVersionsDiff(c echo.Context) error {
	stackID := c.Param("id")
	if stackID == "" {
		stackID = c.Param("stackId")
	}

	versionA, err := strconv.Atoi(c.QueryParam("versionA"))
	if err != nil || versionA <= 0 {
		return errorResponse(c, http.StatusBadRequest, "VERSION_DIFF_REQUEST_INVALID", "versionA must be a positive integer")
	}
	versionB, err := strconv.Atoi(c.QueryParam("versionB"))
	if err != nil || versionB <= 0 {
		return errorResponse(c, http.StatusBadRequest, "VERSION_DIFF_REQUEST_INVALID", "versionB must be a positive integer")
	}

	result, err := h.diffVersions.Execute(c.Request().Context(), usecase.DiffVersionsInput{
		StackID:  stackID,
		VersionA: versionA,
		VersionB: versionB,
	})
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, result)
}

// ListHistory handles GET /api/v1/stacks/:id/history.
func (h *HistoryHandler) ListHistory(c echo.Context) error {
	stackID := c.Param("stackId")

	out, err := h.manageHistory.ListVersions(c.Request().Context(), usecase.ListVersionsInput{
		StackID: stackID,
	})
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "HISTORY_LIST_FAILED", err.Error())
	}

	return c.JSON(http.StatusOK, out.Versions)
}

func (h *HistoryHandler) GetDiff(c echo.Context) error {
	stackID := c.Param("stackId")
	versionID := c.QueryParam("versionId")
	if versionID == "" {
		versions, err := h.historyRepo.ListVersions(c.Request().Context(), stackID)
		if err != nil {
			return errorResponse(c, http.StatusInternalServerError, "HISTORY_LIST_FAILED", err.Error())
		}
		if len(versions) == 0 {
			return c.JSON(http.StatusOK, map[string]any{"stackId": stackID, "versionId": "", "diffs": []any{}})
		}
		versionID = versions[len(versions)-1].ID
	}

	out, err := h.manageHistory.GetDiff(c.Request().Context(), usecase.GetDiffInput{
		StackID:   stackID,
		VersionID: versionID,
	})
	if err != nil {
		return errorResponse(c, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{"stackId": stackID, "versionId": versionID, "diffs": out.Diffs})
}

type rollbackRequest struct {
	VersionID string `json:"versionId"`
	Reason    string `json:"reason"`
}

func (h *HistoryHandler) Rollback(c echo.Context) error {
	stackID := c.Param("stackId")

	var req rollbackRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ROLLBACK_REQUEST_INVALID", err.Error())
	}
	if req.VersionID == "" {
		return errorResponse(c, http.StatusBadRequest, "ROLLBACK_REQUEST_INVALID", "versionId is required")
	}

	// Load the target version
	targetVersion, err := h.historyRepo.GetVersion(c.Request().Context(), stackID, req.VersionID)
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
		reason = "rollback to version " + req.VersionID
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

	return c.JSON(http.StatusOK, map[string]any{"id": out.Version.ID})
}
