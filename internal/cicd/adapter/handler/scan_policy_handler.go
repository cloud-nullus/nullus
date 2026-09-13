package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
)

// ScanPolicyHandler 는 스택의 이미지 스캔 정책을 조회·저장한다.
//
// 스택 라우트(/stacks, admin·devops)에 붙는다. 정책은 그 스택의 모든 스캔
// 파이프라인의 배포 차단 여부를 바꾸므로 개발자 권한으로 열지 않는다.
type ScanPolicyHandler struct {
	svc *usecase.ScanPolicyService
}

// NewScanPolicyHandler 는 ScanPolicyHandler 를 만든다.
func NewScanPolicyHandler(svc *usecase.ScanPolicyService) *ScanPolicyHandler {
	return &ScanPolicyHandler{svc: svc}
}

// RegisterStackRoutes 는 /stacks 그룹 아래에 정책 경로를 붙인다.
func (h *ScanPolicyHandler) RegisterStackRoutes(g *echo.Group) {
	g.GET("/:stackId/image-scan-policy", h.GetScanPolicy)
	g.PUT("/:stackId/image-scan-policy", h.UpdateScanPolicy)
}

// GetScanPolicy 는 스택의 현재 정책이다. 저장한 적 없으면 기본 정책과 is_default=true 다.
func (h *ScanPolicyHandler) GetScanPolicy(c echo.Context) error {
	stackID := strings.TrimSpace(c.Param("stackId"))
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack_id path parameter is required")
	}
	view, err := h.svc.Get(c.Request().Context(), stackID)
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "SCAN_POLICY_READ_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, view)
}

// updateScanPolicyRequest 는 필드를 모두 요구한다. 빠진 필드를 0 값으로 받으면
// ignore_unfixed 누락이 false 가 되어, 수정본 없는 CVE 가 운영자 모르게 차단 사유가 된다.
type updateScanPolicyRequest struct {
	BlockSeverity        *string `json:"block_severity"`
	IgnoreUnfixed        *bool   `json:"ignore_unfixed"`
	OnScannerUnreachable *string `json:"on_scanner_unreachable"`
}

// UpdateScanPolicy 는 정책을 저장하고 그 스택의 스캔 파이프라인에 싣는다.
//
// 푸시 일부가 실패해도 200 이다 — 정책은 저장됐고, 파이프라인별 결과(pushes)가
// 어느 파이프라인이 옛 정책으로 도는지 알려준다.
func (h *ScanPolicyHandler) UpdateScanPolicy(c echo.Context) error {
	stackID := strings.TrimSpace(c.Param("stackId"))
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack_id path parameter is required")
	}

	var req updateScanPolicyRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
	}
	if req.BlockSeverity == nil || req.IgnoreUnfixed == nil || req.OnScannerUnreachable == nil {
		return errorResponse(c, http.StatusBadRequest, "INVALID_SCAN_POLICY",
			"block_severity, ignore_unfixed, on_scanner_unreachable 을 모두 보내야 합니다")
	}
	policy := domain.ScanPolicy{
		BlockSeverity:        domain.Severity(strings.ToUpper(strings.TrimSpace(*req.BlockSeverity))),
		IgnoreUnfixed:        *req.IgnoreUnfixed,
		OnScannerUnreachable: domain.UnreachableAction(strings.ToLower(strings.TrimSpace(*req.OnScannerUnreachable))),
	}

	actor := middleware.ActorFromContext(c)
	updatedBy := actor.Email
	if updatedBy == "" {
		updatedBy = actor.ID
	}

	result, err := h.svc.Update(c.Request().Context(), stackID, policy, updatedBy)
	switch {
	case errors.Is(err, domain.ErrInvalidScanPolicy):
		return errorResponse(c, http.StatusBadRequest, "INVALID_SCAN_POLICY", err.Error())
	case err != nil:
		return errorResponse(c, http.StatusInternalServerError, "SCAN_POLICY_UPDATE_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, result)
}
