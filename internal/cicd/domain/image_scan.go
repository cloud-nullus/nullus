package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
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

// SeverityCounts 는 심각도별 취약점 건수다.
type SeverityCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
}

// ScanPolicy 는 차단 기준이다.
type ScanPolicy struct {
	// IgnoreUnfixed 가 참이면 수정본이 없는 취약점은 판정에서 뺀다.
	IgnoreUnfixed bool
}

// DefaultScanPolicy 는 기본 차단 기준이다 (설계 §6).
//
// CRITICAL 차단 · HIGH 경고 · 수정본 없는 것 제외. HIGH 까지 막으면 흔한 베이스
// 이미지로 첫 배포가 안 되고, 수정본 없는 CVE 로 막으면 사용자가 할 수 있는
// 일이 없다 — debian:11 은 CRITICAL 5건이 전부 unfixed 였다(실측).
func DefaultScanPolicy() ScanPolicy {
	return ScanPolicy{IgnoreUnfixed: true}
}

// StaleDBAge 는 취약점 DB 를 낡았다고 보는 나이다 (설계 §6.4).
//
// 에어갭에서 DB 는 사람이 가져다 넣는 만큼만 새것이다. 강제로 막을 수는 없지만
// 언제 것인지 숨기지 않는다.
const StaleDBAge = 30 * 24 * time.Hour

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
type TrivyReportSummary struct {
	ImageRepository string
	ImageTag        string
	ImageDigest     string
	ScannerVersion  string
	// DBUpdatedAt 은 서버 모드에서 서버가 알려준 취약점 DB 시각이다.
	// 리포트에 없으면 nil 이다.
	DBUpdatedAt *time.Time
	// All 은 모든 취약점, Fixable 은 수정본이 있는 것만 센 값이다.
	All     SeverityCounts
	Fixable SeverityCounts
}

type trivyReport struct {
	ArtifactName string `json:"ArtifactName"`
	Trivy        struct {
		Version string `json:"Version"`
		Server  struct {
			Version         string `json:"Version"`
			VulnerabilityDB struct {
				UpdatedAt *time.Time `json:"UpdatedAt"`
			} `json:"VulnerabilityDB"`
		} `json:"Server"`
	} `json:"Trivy"`
	Metadata struct {
		RepoDigests []string `json:"RepoDigests"`
	} `json:"Metadata"`
	Results []struct {
		Vulnerabilities []struct {
			Severity     string `json:"Severity"`
			FixedVersion string `json:"FixedVersion"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

// ParseTrivyReport 는 Trivy JSON 리포트(SchemaVersion 2)를 요약한다.
//
// 형식은 kind 의 Trivy 서버로 실제 스캔한 리포트에 맞췄다
// (testdata/trivy-report-node16.json).
func ParseTrivyReport(raw []byte) (*TrivyReportSummary, error) {
	var r trivyReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("trivy 리포트를 읽지 못했습니다: %w", err)
	}

	s := &TrivyReportSummary{ScannerVersion: r.Trivy.Version}
	s.ImageRepository, s.ImageTag = splitImageRef(r.ArtifactName)
	for _, d := range r.Metadata.RepoDigests {
		if i := strings.Index(d, "@"); i >= 0 {
			s.ImageDigest = d[i+1:]
			break
		}
	}
	if t := r.Trivy.Server.VulnerabilityDB.UpdatedAt; t != nil && !t.IsZero() {
		updated := *t
		s.DBUpdatedAt = &updated
	}

	for _, res := range r.Results {
		for _, v := range res.Vulnerabilities {
			addSeverity(&s.All, v.Severity)
			if strings.TrimSpace(v.FixedVersion) != "" {
				addSeverity(&s.Fixable, v.Severity)
			}
		}
	}
	return s, nil
}

// splitImageRef 는 "repo:tag" 를 나눈다. 레지스트리 포트의 콜론과 헷갈리지 않도록
// 마지막 슬래시 뒤에서만 태그를 찾는다.
func splitImageRef(ref string) (repo, tag string) {
	ref = strings.TrimSpace(ref)
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	slash := strings.LastIndex(ref, "/")
	if colon := strings.LastIndex(ref, ":"); colon > slash {
		return ref[:colon], ref[colon+1:]
	}
	return ref, ""
}

func addSeverity(c *SeverityCounts, severity string) {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "CRITICAL":
		c.Critical++
	case "HIGH":
		c.High++
	case "MEDIUM":
		c.Medium++
	case "LOW":
		c.Low++
	default:
		// 등급을 매기지 못한 것을 낮음으로 넘겨짚지 않는다.
		c.Unknown++
	}
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
	switch {
	case counts.Critical > 0:
		return GateResultBlock
	case counts.High > 0:
		return GateResultWarn
	default:
		return GateResultPass
	}
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

// IsDBStale 은 취약점 DB 가 낡았는지 본다. 시각을 모르면 낡은 것으로 본다 —
// 모르는 채로 초록불을 켜지 않는다.
func IsDBStale(updatedAt *time.Time, now time.Time) bool {
	if updatedAt == nil || updatedAt.IsZero() {
		return true
	}
	return now.Sub(*updatedAt) > StaleDBAge
}
