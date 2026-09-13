package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// 이미지 취약점 스캔 리포트의 공용 어휘다.
//
// 파이프라인(cicd)은 CI 가 남긴 리포트를, 스택(stack)은 설치한 OSS 이미지의 스캔
// 결과를 센다. 모듈끼리는 서로 import 할 수 없으므로 여기 둔다 — 각자 세면 같은
// 이미지가 화면마다 다른 건수로 보인다.

// SeverityCounts 는 심각도별 취약점 건수다.
type SeverityCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
}

// Add 는 취약점 하나를 심각도에 따라 센다.
func (c *SeverityCounts) Add(severity string) {
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

// Plus 는 두 건수를 더한다.
func (c SeverityCounts) Plus(o SeverityCounts) SeverityCounts {
	return SeverityCounts{
		Critical: c.Critical + o.Critical,
		High:     c.High + o.High,
		Medium:   c.Medium + o.Medium,
		Low:      c.Low + o.Low,
		Unknown:  c.Unknown + o.Unknown,
	}
}

// StaleDBAge 는 취약점 DB 를 낡았다고 보는 나이다 (이미지 스캔 설계 §6.4).
//
// 에어갭에서 DB 는 사람이 가져다 넣는 만큼만 새것이다. 강제로 막을 수는 없지만
// 언제 것인지 숨기지 않는다.
const StaleDBAge = 30 * 24 * time.Hour

// IsDBStale 은 취약점 DB 가 낡았는지 본다. 시각을 모르면 낡은 것으로 본다 —
// 모르는 채로 초록불을 켜지 않는다.
func IsDBStale(updatedAt *time.Time, now time.Time) bool {
	if updatedAt == nil || updatedAt.IsZero() {
		return true
	}
	return now.Sub(*updatedAt) > StaleDBAge
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
			s.All.Add(v.Severity)
			if strings.TrimSpace(v.FixedVersion) != "" {
				s.Fixable.Add(v.Severity)
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
