package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// AuditSink 는 감사 기록을 받는 쪽이다.
//
// 구현체(*audit.AuditLogger)가 아니라 동작에 의존한다. 설정 변경은 "누가 무엇을
// 바꿨나" 가 기능의 일부라서 무엇이 기록되는지를 테스트가 직접 확인해야 하는데,
// 구현체는 DB 풀을 요구해 그럴 수 없다.
type AuditSink interface {
	Log(ctx context.Context, entry audit.AuditEntry) error
}

// ReleaseValuesHandler 는 배포된 스택의 OSS 설정(Helm values)을 편집하는 API 다.
type ReleaseValuesHandler struct {
	manageValues *usecase.ManageReleaseValues
	audit        AuditSink
}

// NewReleaseValuesHandler 는 핸들러를 조립한다. auditSink 는 없어도 된다.
func NewReleaseValuesHandler(manageValues *usecase.ManageReleaseValues, auditSink AuditSink) *ReleaseValuesHandler {
	return &ReleaseValuesHandler{manageValues: manageValues, audit: auditSink}
}

// RegisterRoutes 는 stacks 그룹에 라우트를 붙인다.
func (h *ReleaseValuesHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/:stackId/releases", h.ListReleases)
	g.GET("/:stackId/releases/:releaseName/values", h.GetValues)
	// 미리보기는 클러스터를 바꾸지 않지만 본문이 필요하므로 POST 다.
	g.POST("/:stackId/releases/:releaseName/values/preview", h.PreviewValues)
	g.PUT("/:stackId/releases/:releaseName/values", h.ApplyValues)
}

type applyValuesRequest struct {
	Mode string `json:"mode"`
	YAML string `json:"yaml"`
}

func (h *ReleaseValuesHandler) ListReleases(c echo.Context) error {
	releases, err := h.manageValues.ListReleases(c.Request().Context(), stackIDParam(c))
	if err != nil {
		return releaseValuesError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"releases": releases})
}

func (h *ReleaseValuesHandler) GetValues(c echo.Context) error {
	out, err := h.manageValues.GetValues(c.Request().Context(), usecase.GetReleaseValuesInput{
		StackID:     stackIDParam(c),
		ReleaseName: c.Param("releaseName"),
		Mode:        usecase.ReleaseValuesMode(c.QueryParam("mode")),
	})
	if err != nil {
		return releaseValuesError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *ReleaseValuesHandler) PreviewValues(c echo.Context) error {
	return h.apply(c, true)
}

func (h *ReleaseValuesHandler) ApplyValues(c echo.Context) error {
	return h.apply(c, false)
}

func (h *ReleaseValuesHandler) apply(c echo.Context, dryRun bool) error {
	var req applyValuesRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "RELEASE_VALUES_INVALID", err.Error())
	}

	stackID := stackIDParam(c)
	releaseName := c.Param("releaseName")
	actor := middleware.ActorFromContext(c)

	out, err := h.manageValues.Apply(c.Request().Context(), usecase.ApplyReleaseValuesInput{
		StackID:     stackID,
		ReleaseName: releaseName,
		Mode:        usecase.ReleaseValuesMode(req.Mode),
		YAML:        req.YAML,
		DryRun:      dryRun,
		ChangedBy:   actor.Label(),
	})

	// 미리보기는 클러스터를 바꾸지 않으므로 남기지 않는다. 실패한 적용은 남긴다 —
	// 성공만 기록하는 감사 기록은 "누가 무엇을 시도했나" 에 답하지 못한다.
	if !dryRun {
		h.logApply(c, actor, stackID, releaseName, req, out, err)
	}

	if err != nil {
		return releaseValuesError(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *ReleaseValuesHandler) logApply(
	c echo.Context,
	actor middleware.Actor,
	stackID, releaseName string,
	req applyValuesRequest,
	out *usecase.ApplyReleaseValuesOutput,
	applyErr error,
) {
	if h.audit == nil {
		return
	}

	details := map[string]any{
		"release": releaseName,
		"mode":    req.Mode,
		"actor":   actor.Label(),
		// 어떤 키를 건드렸는지만 남긴다. 값은 남기지 않는다 — values 에는
		// 사용자가 적어 넣은 자격증명이 들어갈 수 있고, 감사 로그는 그보다
		// 넓게 읽힌다.
		"changed_paths": usecase.TopLevelValuePaths(req.YAML),
		"result":        "applied",
	}
	if applyErr != nil {
		details["result"] = "failed"
		details["error"] = truncateAuditMessage(applyErr.Error())
	}
	if out != nil {
		details["revision"] = out.Revision
		details["step"] = out.StepName
		if len(out.Warnings) > 0 {
			paths := make([]string, 0, len(out.Warnings))
			for _, warning := range out.Warnings {
				paths = append(paths, warning.Path)
			}
			details["protected_paths_touched"] = paths
		}
	}

	_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
		UserID:       actor.ID,
		Action:       "update_release_values",
		ResourceType: "stack",
		ResourceID:   stackID,
		Details:      details,
		IPAddress:    c.RealIP(),
	})
}

// stackIDParam 은 :stackId 와 :id 양쪽을 받는다. stacks 그룹에는 두 이름이
// 섞여 등록돼 있어 어느 쪽으로 바인딩될지 라우터 등록 순서에 달려 있다.
func stackIDParam(c echo.Context) string {
	if id := strings.TrimSpace(c.Param("stackId")); id != "" {
		return id
	}
	return strings.TrimSpace(c.Param("id"))
}

// auditMessageLimit 은 감사 상세에 남길 오류 메시지의 상한이다.
//
// Kubernetes 의 패치 실패는 요청 본문 전체를 메시지에 담아 되돌려준다 —
// 실제로 한 건이 9.8KB 였다. 그대로 넣으면 감사 테이블이 부풀고 목록 조회가
// 무거워지는데, 원인 파악에 필요한 부분은 언제나 앞머리다.
const auditMessageLimit = 500

func truncateAuditMessage(message string) string {
	if len(message) <= auditMessageLimit {
		return message
	}
	return message[:auditMessageLimit] + "… (truncated)"
}

func releaseValuesError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrStackNotFound):
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	case errors.Is(err, usecase.ErrReleaseNotFound):
		return errorResponse(c, http.StatusNotFound, "RELEASE_NOT_FOUND", err.Error())
	case errors.Is(err, usecase.ErrReleaseValuesInvalidYAML):
		return errorResponse(c, http.StatusBadRequest, "RELEASE_VALUES_INVALID_YAML", err.Error())
	case errors.Is(err, usecase.ErrReleaseValuesInvalidMode):
		return errorResponse(c, http.StatusBadRequest, "RELEASE_VALUES_INVALID_MODE", err.Error())
	default:
		return errorResponse(c, http.StatusInternalServerError, "RELEASE_VALUES_FAILED", err.Error())
	}
}
