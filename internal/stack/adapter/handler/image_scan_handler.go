package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type stackImageScanReporter interface {
	Report(ctx context.Context, stackID string) (*domain.StackImageScanReport, error)
	Vulnerabilities(ctx context.Context, stackID, digest string, filter shareddomain.VulnerabilityFilter) (*shareddomain.VulnerabilityPage, error)
}

// ImageScanHandler 는 스택이 설치한 OSS 이미지의 취약점 보고서를 내보낸다.
type ImageScanHandler struct {
	reporter stackImageScanReporter
	now      func() time.Time
}

// NewImageScanHandler 는 핸들러를 만든다.
func NewImageScanHandler(reporter stackImageScanReporter) *ImageScanHandler {
	return &ImageScanHandler{reporter: reporter, now: time.Now}
}

// RegisterRoutes 는 /stacks 그룹에 붙는다.
func (h *ImageScanHandler) RegisterRoutes(stacks *echo.Group) {
	stacks.GET("/:stackId/image-scans", h.GetImageScans)
	stacks.GET("/:stackId/image-scans/vulnerabilities", h.GetImageVulnerabilities)
}

// GetImageVulnerabilities handles GET /api/v1/stacks/:stackId/image-scans/vulnerabilities?digest=...
//
// 설치 이미지 하나의 취약점 목록을 한 쪽씩 돌려준다. 목록을 보일 수 없으면 200 과 함께
// 이유(status=unavailable)를 담는다 — 빈 목록은 0건으로 읽힌다.
func (h *ImageScanHandler) GetImageVulnerabilities(c echo.Context) error {
	stackID := strings.TrimSpace(c.Param("stackId"))
	digest := strings.TrimSpace(c.QueryParam("digest"))
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack id is required")
	}
	if digest == "" {
		return errorResponse(c, http.StatusBadRequest, "IMAGE_DIGEST_REQUIRED", "digest query parameter is required")
	}
	filter := shareddomain.NewVulnerabilityFilter(c.QueryParam("severity"), c.QueryParam("class"),
		c.QueryParam("fixable"), c.QueryParam("q"), c.QueryParam("limit"), c.QueryParam("offset"))

	page, err := h.reporter.Vulnerabilities(c.Request().Context(), stackID, digest, filter)
	switch {
	case errors.Is(err, usecase.ErrStackImageScanNotFound):
		return errorResponse(c, http.StatusNotFound, "IMAGE_SCAN_NOT_FOUND", err.Error())
	case err != nil && strings.Contains(strings.ToLower(err.Error()), "not found"):
		return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
	case err != nil:
		return errorResponse(c, http.StatusInternalServerError, "STACK_IMAGE_VULNERABILITIES_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, page)
}

type stackImageScanItem struct {
	Image          string                       `json:"image"`
	ImageDigest    string                       `json:"image_digest"`
	Release        string                       `json:"release,omitempty"`
	Workloads      []string                     `json:"workloads"`
	Status         string                       `json:"status"`
	Error          string                       `json:"error,omitempty"`
	Counts         *shareddomain.SeverityCounts `json:"counts,omitempty"`
	FixableCounts  *shareddomain.SeverityCounts `json:"fixable_counts,omitempty"`
	ScannerVersion string                       `json:"scanner_version,omitempty"`
	DBUpdatedAt    *time.Time                   `json:"db_updated_at,omitempty"`
	DBStale        bool                         `json:"db_stale"`
	ScannedAt      time.Time                    `json:"scanned_at"`
}

type stackImageScanResponse struct {
	StackID       string                       `json:"stack_id"`
	Status        string                       `json:"status"`
	Reason        string                       `json:"reason"`
	LastScannedAt *time.Time                   `json:"last_scanned_at"`
	Summary       *shareddomain.SeverityCounts `json:"summary"`
	Items         []stackImageScanItem         `json:"items"`
	Total         int                          `json:"total"`
}

// GetImageScans handles GET /api/v1/stacks/:stackId/image-scans.
//
// 보고용이다. 스캔하지 않는 스택도 같은 모양으로 이유를 담아 답한다 — 화면이
// "0건" 과 "스캔 안 함" 을 구분해야 한다.
func (h *ImageScanHandler) GetImageScans(c echo.Context) error {
	stackID := strings.TrimSpace(c.Param("stackId"))
	if stackID == "" {
		return errorResponse(c, http.StatusBadRequest, "STACK_ID_REQUIRED", "stack id is required")
	}
	report, err := h.reporter.Report(c.Request().Context(), stackID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return errorResponse(c, http.StatusNotFound, "STACK_NOT_FOUND", err.Error())
		}
		return errorResponse(c, http.StatusInternalServerError, "STACK_IMAGE_SCANS_FAILED", err.Error())
	}

	now := h.now()
	resp := stackImageScanResponse{
		StackID:       report.StackID,
		Status:        string(report.Status),
		Reason:        string(report.Reason),
		LastScannedAt: report.LastScannedAt,
		Summary:       report.Summary,
		Items:         make([]stackImageScanItem, 0, len(report.Items)),
	}
	for _, s := range report.Items {
		workloads := s.Workloads
		if workloads == nil {
			workloads = []string{}
		}
		resp.Items = append(resp.Items, stackImageScanItem{
			Image:          s.Image,
			ImageDigest:    s.ImageDigest,
			Release:        s.Release,
			Workloads:      workloads,
			Status:         string(s.Status),
			Error:          s.Error,
			Counts:         s.Counts,
			FixableCounts:  s.FixableCounts,
			ScannerVersion: s.ScannerVersion,
			DBUpdatedAt:    s.DBUpdatedAt,
			DBStale:        shareddomain.IsDBStale(s.DBUpdatedAt, now),
			ScannedAt:      s.ScannedAt,
		})
	}
	resp.Total = len(resp.Items)
	return c.JSON(http.StatusOK, resp)
}
