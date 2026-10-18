package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// UpgradeService 는 설치된 OSS 업그레이드 유스케이스다(*usecase.UpgradeStack).
type UpgradeService interface {
	Candidates(ctx context.Context, stackID string) ([]domain.UpgradeCandidate, error)
	Preflight(ctx context.Context, stackID, bundleID string) ([]domain.UpgradeCheck, error)
	Start(ctx context.Context, in usecase.StartUpgradeInput) (*domain.UpgradeRun, error)
	GetRun(ctx context.Context, stackID, runID string) (*domain.UpgradeRun, error)
}

type UpgradeHandler struct {
	upgrades UpgradeService
	audit    AuditSink
}

// NewUpgradeHandler 는 핸들러를 조립한다. auditSink 는 없어도 된다.
func NewUpgradeHandler(upgrades UpgradeService, auditSink AuditSink) *UpgradeHandler {
	return &UpgradeHandler{upgrades: upgrades, audit: auditSink}
}

func (h *UpgradeHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/:stackId/upgrades", h.ListCandidates)
	// 사전 점검은 클러스터를 바꾸지 않지만 본문(번들)이 필요하므로 POST 다.
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

// Start 는 실행을 만들어 돌려준다. 통과한 실행은 백그라운드에서 이어지므로 202 이고, 화면은
// GET /upgrade-runs/:runId 로 끝날 때까지 본다. 막힌 시도는 그 자체로 끝난 실행이라 200 이다.
func (h *UpgradeHandler) Start(c echo.Context) error {
	var req upgradeBundleRequest
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.BundleID) == "" {
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_INVALID", "bundle_id is required")
	}
	key := strings.TrimSpace(c.Request().Header.Get("Idempotency-Key"))
	if key == "" {
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_INVALID", "Idempotency-Key header is required")
	}
	actor := middleware.ActorFromContext(c)
	stackID := stackIDParam(c)
	run, err := h.upgrades.Start(c.Request().Context(), usecase.StartUpgradeInput{StackID: stackID, BundleID: req.BundleID,
		Reason: req.Reason, RequestedBy: actor.Label(), IdempotencyKey: key})
	h.logStart(c, actor, stackID, req, run, err)
	if err != nil {
		return upgradeError(c, err)
	}
	status := http.StatusOK
	if run.Active() {
		status = http.StatusAccepted
	}
	return c.JSON(status, run)
}

func (h *UpgradeHandler) GetRun(c echo.Context) error {
	run, err := h.upgrades.GetRun(c.Request().Context(), stackIDParam(c), c.Param("runId"))
	if err != nil {
		return upgradeError(c, err)
	}
	return c.JSON(http.StatusOK, run)
}

func (h *UpgradeHandler) logStart(c echo.Context, actor middleware.Actor, stackID string, req upgradeBundleRequest, run *domain.UpgradeRun, startErr error) {
	if h.audit == nil {
		return
	}
	details := map[string]any{"bundle_id": req.BundleID, "reason": req.Reason, "actor": actor.Label()}
	if run != nil {
		details["run_id"] = run.ID
		details["from"] = run.SourceChartVersion
		details["to"] = run.TargetChartVersion
		details["status"] = run.Status
	}
	if startErr != nil {
		details["status"] = "rejected"
		details["error"] = truncateAuditMessage(startErr.Error())
	}
	_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
		UserID:       actor.ID,
		Action:       "start_stack_upgrade",
		ResourceType: "stack",
		ResourceID:   stackID,
		Details:      details,
		IPAddress:    c.RealIP(),
	})
}

func upgradeError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrStackNotFound),
		errors.Is(err, domain.ErrUpgradeBundleNotFound),
		errors.Is(err, domain.ErrUpgradeRunNotFound),
		errors.Is(err, domain.ErrUpgradeReleaseNotFound):
		return errorResponse(c, http.StatusNotFound, "UPGRADE_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrUpgradeInProgress):
		return errorResponse(c, http.StatusConflict, "UPGRADE_IN_PROGRESS", err.Error())
	case errors.Is(err, domain.ErrUpgradeReasonRequired), errors.Is(err, domain.ErrUpgradeIdempotencyKeyRequired):
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_INVALID", err.Error())
	default:
		return errorResponse(c, http.StatusInternalServerError, "UPGRADE_FAILED", err.Error())
	}
}
