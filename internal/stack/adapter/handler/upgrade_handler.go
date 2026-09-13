package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type UpgradeHandler struct{ upgrades *usecase.UpgradeStack }

func NewUpgradeHandler(upgrades *usecase.UpgradeStack) *UpgradeHandler {
	return &UpgradeHandler{upgrades: upgrades}
}

func (h *UpgradeHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/:stackId/upgrades", h.ListCandidates)
	g.POST("/:stackId/upgrades/preflight", h.Preflight)
	g.POST("/:stackId/upgrade-runs", h.Start)
	g.GET("/:stackId/upgrade-runs/:runId", h.GetRun)
}

func (h *UpgradeHandler) ListCandidates(c echo.Context) error {
	items, err := h.upgrades.Candidates(c.Request().Context(), stackIDParam(c))
	if err != nil {
		return upgradeError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

type upgradeBundleRequest struct {
	BundleID string `json:"bundle_id"`
	Reason   string `json:"reason"`
}

func (h *UpgradeHandler) Preflight(c echo.Context) error {
	var req upgradeBundleRequest
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.BundleID) == "" {
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_INVALID", "bundle_id is required")
	}
	checks, err := h.upgrades.Preflight(c.Request().Context(), stackIDParam(c), req.BundleID)
	if err != nil {
		return upgradeError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"checks": checks})
}

func (h *UpgradeHandler) Start(c echo.Context) error {
	var req upgradeBundleRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_INVALID", err.Error())
	}
	idempotencyKey := strings.TrimSpace(c.Request().Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_INVALID", "Idempotency-Key header is required")
	}
	actor := middleware.ActorFromContext(c)
	run, err := h.upgrades.Start(c.Request().Context(), usecase.StartUpgradeInput{StackID: stackIDParam(c), BundleID: req.BundleID,
		Reason: req.Reason, RequestedBy: actor.Label(), IdempotencyKey: idempotencyKey})
	if err != nil {
		return upgradeError(c, err)
	}
	return c.JSON(http.StatusOK, run)
}

func (h *UpgradeHandler) GetRun(c echo.Context) error {
	run, err := h.upgrades.GetRun(c.Request().Context(), stackIDParam(c), c.Param("runId"))
	if err != nil {
		return upgradeError(c, err)
	}
	return c.JSON(http.StatusOK, run)
}

func upgradeError(c echo.Context, err error) error {
	msg := err.Error()
	status := http.StatusUnprocessableEntity
	if strings.Contains(msg, "not found") {
		status = http.StatusNotFound
	}
	if strings.Contains(msg, "already running") {
		status = http.StatusConflict
	}
	return errorResponse(c, status, "UPGRADE_FAILED", msg)
}
