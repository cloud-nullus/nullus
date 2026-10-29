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

// UpgradeBundleService 는 번들 관리 유스케이스다(*usecase.ManageUpgradeBundles).
type UpgradeBundleService interface {
	List(ctx context.Context) ([]domain.UpgradeBundle, error)
	Register(ctx context.Context, in usecase.RegisterUpgradeBundleInput) (*domain.UpgradeBundle, error)
	ChangeStatus(ctx context.Context, id, status string, rehearsalConfirmed bool) (*domain.UpgradeBundle, error)
}

// UpgradeBundleHandler 는 관리자(admin 그룹)의 업그레이드 번들 등록 · 상태 변경이다. 번들은
// 플랫폼 전체의 것이라 조직 헤더를 보지 않는다. 등록과 상태 변경은 감사 로그에 남긴다 — "누가
// 이 경로를 verified 라고 했나" 에 답해야 한다.
type UpgradeBundleHandler struct {
	bundles UpgradeBundleService
	audit   AuditSink
}

func NewUpgradeBundleHandler(bundles UpgradeBundleService, auditSink AuditSink) *UpgradeBundleHandler {
	return &UpgradeBundleHandler{bundles: bundles, audit: auditSink}
}

func (h *UpgradeBundleHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/upgrade-bundles", h.List)
	g.POST("/upgrade-bundles", h.Register)
	g.PATCH("/upgrade-bundles/:id/status", h.ChangeStatus)
}

func (h *UpgradeBundleHandler) List(c echo.Context) error {
	items, err := h.bundles.List(c.Request().Context())
	if err != nil {
		return upgradeBundleError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

type registerUpgradeBundleRequest struct {
	ID                     string   `json:"id"`
	Tool                   string   `json:"tool"`
	ReleaseName            string   `json:"release_name"`
	ChartName              string   `json:"chart_name"`
	RepoURL                string   `json:"repo_url"`
	SourceChartVersion     string   `json:"source_chart_version"`
	SourceAppVersion       string   `json:"source_app_version"`
	TargetChartVersion     string   `json:"target_chart_version"`
	TargetAppVersion       string   `json:"target_app_version"`
	RiskLevel              string   `json:"risk_level"`
	ExpectedDisruption     string   `json:"expected_disruption"`
	ReleaseNotes           string   `json:"release_notes"`
	RequiresBackup         *bool    `json:"requires_backup"`
	SupportedArchitectures []string `json:"supported_architectures"`
	FixesCVEs              []string `json:"fixes_cves"`
	Status                 string   `json:"status"`
	RehearsalConfirmed     bool     `json:"rehearsal_confirmed"`
}

func (h *UpgradeBundleHandler) Register(c echo.Context) error {
	var req registerUpgradeBundleRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_BUNDLE_INVALID", "invalid request body")
	}
	// 백업 요구는 비우면 참이다 — 안전 조건은 일부러 끄는 것이지 잊어서 빠지는 것이 아니다.
	requiresBackup := true
	if req.RequiresBackup != nil {
		requiresBackup = *req.RequiresBackup
	}
	in := usecase.RegisterUpgradeBundleInput{
		ID: req.ID, Tool: req.Tool, ReleaseName: req.ReleaseName, ChartName: req.ChartName, RepoURL: req.RepoURL,
		SourceChartVersion: req.SourceChartVersion, SourceAppVersion: req.SourceAppVersion,
		TargetChartVersion: req.TargetChartVersion, TargetAppVersion: req.TargetAppVersion,
		RiskLevel: req.RiskLevel, ExpectedDisruption: req.ExpectedDisruption, ReleaseNotes: req.ReleaseNotes,
		RequiresBackup: requiresBackup, SupportedArchitectures: req.SupportedArchitectures, FixesCVEs: req.FixesCVEs,
		Status: req.Status, RehearsalConfirmed: req.RehearsalConfirmed,
	}
	actor := middleware.ActorFromContext(c)
	bundle, err := h.bundles.Register(c.Request().Context(), in)
	h.log(c, actor, "register_upgrade_bundle", bundleIDOf(bundle, req.ID), map[string]any{
		"tool": req.Tool, "from": req.SourceChartVersion, "to": req.TargetChartVersion,
		"requested_status": req.Status, "rehearsal_confirmed": req.RehearsalConfirmed,
	}, bundle, err)
	if err != nil {
		return upgradeBundleError(c, err)
	}
	return c.JSON(http.StatusCreated, bundle)
}

type upgradeBundleStatusRequest struct {
	Status             string `json:"status"`
	RehearsalConfirmed bool   `json:"rehearsal_confirmed"`
}

func (h *UpgradeBundleHandler) ChangeStatus(c echo.Context) error {
	var req upgradeBundleStatusRequest
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Status) == "" {
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_BUNDLE_INVALID", "status is required")
	}
	id := strings.TrimSpace(c.Param("id"))
	actor := middleware.ActorFromContext(c)
	bundle, err := h.bundles.ChangeStatus(c.Request().Context(), id, req.Status, req.RehearsalConfirmed)
	h.log(c, actor, "change_upgrade_bundle_status", id, map[string]any{
		"to_status": req.Status, "rehearsal_confirmed": req.RehearsalConfirmed,
	}, bundle, err)
	if err != nil {
		return upgradeBundleError(c, err)
	}
	return c.JSON(http.StatusOK, bundle)
}

func bundleIDOf(b *domain.UpgradeBundle, fallback string) string {
	if b != nil {
		return b.ID
	}
	return strings.TrimSpace(fallback)
}

func (h *UpgradeBundleHandler) log(c echo.Context, actor middleware.Actor, action, bundleID string, details map[string]any, bundle *domain.UpgradeBundle, opErr error) {
	if h.audit == nil {
		return
	}
	details["actor"] = actor.Label()
	details["bundle_id"] = bundleID
	if bundle != nil {
		details["status"] = bundle.Status
		details["tool"] = bundle.Tool
		details["from"] = bundle.SourceChartVersion
		details["to"] = bundle.TargetChartVersion
	}
	if opErr != nil {
		details["status"] = "rejected"
		details["error"] = truncateAuditMessage(opErr.Error())
	}
	_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
		UserID:       actor.ID,
		Action:       action,
		ResourceType: "upgrade_bundle",
		ResourceID:   bundleID,
		Details:      details,
		IPAddress:    c.RealIP(),
	})
}

func upgradeBundleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrUpgradeBundleUnknown):
		return errorResponse(c, http.StatusNotFound, "UPGRADE_BUNDLE_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrUpgradeBundleRehearsalRequired):
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_BUNDLE_REHEARSAL_REQUIRED", err.Error())
	case errors.Is(err, domain.ErrUpgradeBundleInvalid):
		return errorResponse(c, http.StatusBadRequest, "UPGRADE_BUNDLE_INVALID", err.Error())
	case errors.Is(err, domain.ErrUpgradeBundleExists):
		return errorResponse(c, http.StatusConflict, "UPGRADE_BUNDLE_EXISTS", err.Error())
	case errors.Is(err, domain.ErrUpgradeBundleStatusTransition):
		return errorResponse(c, http.StatusConflict, "UPGRADE_BUNDLE_STATUS_CONFLICT", err.Error())
	default:
		return errorResponse(c, http.StatusInternalServerError, "UPGRADE_BUNDLE_FAILED", err.Error())
	}
}
