package domain

import (
	"strings"
	"time"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// GateResult 는 이미지 스캔 게이트의 판정이다.
type GateResult string

const (
	GateResultPass  GateResult = "pass"
	GateResultWarn  GateResult = "warn"
	GateResultBlock GateResult = "block"
	// GateResultError 는 판정할 수 없었다는 뜻이다(스캐너 도달 불가, 리포트 없음).
	//
	// 통과와 뭉치지 않는다 — 스캔이 못 돈 것과 취약점이 없는 것은 다르다.
	// 뭉치면 스캐너가 죽은 동안 모든 배포가 초록불로 지나간다.
	GateResultError GateResult = "error"
)

// SeverityCounts 는 심각도별 취약점 건수다. 스택 모듈도 설치 이미지를 같은 어휘로
// 세므로 shared 가 소유한다.
type SeverityCounts = shareddomain.SeverityCounts

// ScanPolicy 는 스택 하나의 차단 기준이다.
//
// 스캐너가 스택마다 서고 장애 영향도 스택 하나로 막히므로 정책도 스택 단위다.
// 플랫폼이 CI 변수로 푸시하고, CI 가 그 값으로 스스로 판정한다.
type ScanPolicy struct {
	// BlockSeverity 이상의 취약점이 있으면 차단한다. 바로 아래 등급은 경고로 남긴다.
	BlockSeverity Severity `json:"block_severity"`
	// IgnoreUnfixed 가 참이면 수정본이 없는 취약점은 판정에서 뺀다.
	IgnoreUnfixed bool `json:"ignore_unfixed"`
	// OnScannerUnreachable 은 스캔을 수행하지 못했을 때의 동작이다.
	OnScannerUnreachable UnreachableAction `json:"on_scanner_unreachable"`
}

// DefaultScanPolicy 는 기본 차단 기준이다 (설계 §6).
//
// CRITICAL 차단 · HIGH 경고 · 수정본 없는 것 제외 · 스캔 불가 시 차단. HIGH 까지
// 막으면 흔한 베이스 이미지로 첫 배포가 안 되고, 수정본 없는 CVE 로 막으면
// 사용자가 할 수 있는 일이 없다 — debian:11 은 CRITICAL 5건이 전부 unfixed 였다(실측).
func DefaultScanPolicy() ScanPolicy {
	return ScanPolicy{
		BlockSeverity:        SeverityCritical,
		IgnoreUnfixed:        true,
		OnScannerUnreachable: UnreachableBlock,
	}
}

// StaleDBAge 는 취약점 DB 를 낡았다고 보는 나이다 (설계 §6.4).
const StaleDBAge = shareddomain.StaleDBAge

// ImageScanResult 는 이미지 하나에 대한 스캔 결과 기록이다.
type ImageScanResult struct {
	ID           string `json:"id"`
	PipelineID   string `json:"pipeline_id"`
	DeploymentID string `json:"deployment_id,omitempty"`

	ImageRepository string `json:"image_repository,omitempty"`
	ImageTag        string `json:"image_tag,omitempty"`
	// ImageDigest 에 결과를 붙인다 — 태그는 움직인다.
	ImageDigest string `json:"image_digest,omitempty"`

	ScanSource     string     `json:"scan_source"`
	Scanner        string     `json:"scanner"`
	ScannerVersion string     `json:"scanner_version,omitempty"`
	DBUpdatedAt    *time.Time `json:"db_updated_at,omitempty"`

	// Counts 는 리포트를 읽었을 때만 있다. nil 은 "모름" 이다 —
	// 0 으로 채우면 "취약점 0건" 으로 읽힌다.
	Counts *SeverityCounts `json:"counts,omitempty"`

	GateResult GateResult `json:"gate_result"`
	ReportURI  string     `json:"report_uri,omitempty"`
	ScannedAt  time.Time  `json:"scanned_at"`
}

// TrivyReportSummary 는 Trivy JSON 리포트에서 판정에 필요한 것만 뽑은 것이다.
type TrivyReportSummary = shareddomain.TrivyReportSummary

// ParseTrivyReport 는 Trivy JSON 리포트(SchemaVersion 2)를 요약한다.
func ParseTrivyReport(raw []byte) (*TrivyReportSummary, error) {
	return shareddomain.ParseTrivyReport(raw)
}

// EvaluateGate 는 요약을 정책으로 판정한다.
//
// 리포트가 없으면 error 다 — 통과로 뭉치지 않는다.
func EvaluateGate(s *TrivyReportSummary, policy ScanPolicy) GateResult {
	if s == nil {
		return GateResultError
	}
	counts := s.All
	if policy.IgnoreUnfixed {
		counts = s.Fixable
	}

	// 차단 등급 이상이 하나라도 있으면 차단, 바로 아래 등급이 있으면 경고다.
	block := policy.blockRank()
	for i := 0; i <= block; i++ {
		if countAt(counts, severityOrder[i]) > 0 {
			return GateResultBlock
		}
	}
	if warn := block + 1; warn < len(severityOrder) && countAt(counts, severityOrder[warn]) > 0 {
		return GateResultWarn
	}
	return GateResultPass
}

// GateResultFromStageStatus 는 리포트 없이 CI 단계 상태만으로 판정을 남긴다.
//
// 스캔 단계는 --exit-code 1 로 차단하므로 성공이면 통과, 실패면 차단이다.
// 그 밖의 상태(실행 중·대기·건너뜀·모름)는 판정하지 않는다 — 도는 중인 것을
// 통과로 적으면 안 된다.
func GateResultFromStageStatus(status string) (GateResult, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success":
		return GateResultPass, true
	case "failed":
		return GateResultBlock, true
	default:
		return "", false
	}
}

// IsDBStale 은 취약점 DB 가 낡았는지 본다. 시각을 모르면 낡은 것으로 본다.
func IsDBStale(updatedAt *time.Time, now time.Time) bool {
	return shareddomain.IsDBStale(updatedAt, now)
}
