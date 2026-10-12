package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

var (
	// ErrScanPipelineNotFound 는 파이프라인이 없다는 뜻이다.
	ErrScanPipelineNotFound = errors.New("파이프라인을 찾지 못했습니다")
	// ErrImageScanNotFound 는 그 파이프라인에 그 스캔 결과가 없다는 뜻이다.
	ErrImageScanNotFound = errors.New("스캔 결과를 찾지 못했습니다")
)

// ScanVulnerabilities 는 파이프라인 스캔 결과의 취약점 목록을 돌려준다.
//
// 목록은 저장하지 않는다. 볼 때 CI 가 남긴 리포트를 다시 읽어 만든다 — 원본 리포트는
// DB 에 넣지 않는다는 결정(이미지 스캔 설계 §8)을 지키고, 목록이 리포트와 어긋날 일이
// 없다. 대신 CI 가 보관 기간이 지나 리포트를 지우면 목록은 볼 수 없고 건수만 남는다.
type ScanVulnerabilities struct {
	pipelines port.PipelineRepository
	scans     port.ImageScanResultRepository
	factory   port.SCMBundleFactory
}

// NewScanVulnerabilities 는 유스케이스를 만든다.
func NewScanVulnerabilities(
	pipelines port.PipelineRepository,
	scans port.ImageScanResultRepository,
	factory port.SCMBundleFactory,
) *ScanVulnerabilities {
	return &ScanVulnerabilities{pipelines: pipelines, scans: scans, factory: factory}
}

// Execute 는 조건에 맞는 한 쪽을 돌려준다. 목록을 보일 수 없으면 이유를 담는다 —
// 빈 목록은 취약점 0건으로 읽힌다.
func (uc *ScanVulnerabilities) Execute(
	ctx context.Context,
	pipelineID, scanID string,
	filter shareddomain.VulnerabilityFilter,
) (*shareddomain.VulnerabilityPage, error) {
	pipeline, err := uc.pipelines.GetByID(ctx, strings.TrimSpace(pipelineID))
	if err != nil || pipeline == nil {
		return nil, fmt.Errorf("%w: %s", ErrScanPipelineNotFound, pipelineID)
	}
	scan, err := uc.scans.GetByID(ctx, strings.TrimSpace(scanID))
	if err != nil {
		return nil, fmt.Errorf("스캔 결과 조회 실패: %w", err)
	}
	// 다른 파이프라인의 스캔을 이 경로로 보이지 않는다.
	if scan == nil || scan.PipelineID != pipeline.ID {
		return nil, fmt.Errorf("%w: %s", ErrImageScanNotFound, scanID)
	}

	unavailable := func(reason string) (*shareddomain.VulnerabilityPage, error) {
		page := shareddomain.UnavailableVulnerabilityPage(reason, filter)
		return &page, nil
	}
	if scan.ReportRef == nil {
		// 리포트를 읽지 못한 스캔(스캐너에 닿지 못함)이거나 위치 기록 전의 기록이다.
		return unavailable(shareddomain.VulnerabilityReasonReportMissing)
	}
	if uc.factory == nil || strings.TrimSpace(pipeline.StackID) == "" {
		return unavailable(shareddomain.VulnerabilityReasonCIUnreachable)
	}
	bundle, err := uc.factory.For(ctx, pipeline.StackID)
	if err != nil || bundle == nil || bundle.CIArtifacts == nil {
		slog.Warn("취약점 목록: CI 번들을 만들지 못했습니다",
			"pipeline_id", pipeline.ID, "stack_id", pipeline.StackID, "error", err)
		return unavailable(shareddomain.VulnerabilityReasonCIUnreachable)
	}

	raw, found, err := bundle.CIArtifacts.ReadArtifact(ctx, artifactRefFrom(scan.ReportRef))
	if err != nil {
		slog.Warn("취약점 목록: CI 리포트를 읽지 못했습니다", "scan_id", scan.ID, "error", err)
		return unavailable(shareddomain.VulnerabilityReasonCIUnreachable)
	}
	if !found {
		return unavailable(shareddomain.VulnerabilityReasonReportExpired)
	}
	vulns, err := shareddomain.ParseTrivyVulnerabilities(raw)
	if err != nil {
		slog.Warn("취약점 목록: 리포트 형식이 맞지 않습니다", "scan_id", scan.ID, "error", err)
		return unavailable(shareddomain.VulnerabilityReasonReportMissing)
	}
	page := shareddomain.NewVulnerabilityPage(vulns, filter)
	return &page, nil
}
