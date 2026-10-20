package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SecretScanResult 는 실행 하나의 시크릿 탐지(gitleaks) 결과 요약이다
// (nullus-plan#80, Security Dashboard 설계 §8 G2).
//
// 비밀값은 담지 않는다 — 탐지 위치(규칙 · 파일 · 줄 · 커밋 · Fingerprint)만 둔다. 이것만으로 무엇을
// 폐기·회전할지, 오탐이면 .gitleaksignore 에 무엇을 적을지 알 수 있다.
type SecretScanResult struct {
	ID           string `json:"id"`
	PipelineID   string `json:"pipeline_id"`
	DeploymentID string `json:"deployment_id"`

	Scanner        string `json:"scanner"`
	ScannerVersion string `json:"scanner_version,omitempty"`

	// FindingsCount 는 리포트를 읽었을 때만 있다. nil 은 "모름" 이다 — 0 으로 채우면 "탐지 없음" 으로 읽힌다.
	FindingsCount *int `json:"findings_count,omitempty"`
	// Rules 는 규칙별 탐지 건수다(잘리지 않는다). Findings 는 앞의 MaxStoredSecretFindings 건만이다.
	Rules    []SecretRuleCount `json:"rules"`
	Findings []SecretFinding   `json:"findings"`

	GateResult GateResult `json:"gate_result"`
	ReportURI  string     `json:"report_uri,omitempty"`
	ScannedAt  time.Time  `json:"scanned_at"`

	// ReportRef 는 CI 리포트를 다시 읽을 위치다. 화면에는 내보내지 않는다.
	ReportRef *ScanReportRef `json:"-"`
}

// SecretRuleCount 는 규칙 하나의 탐지 건수다.
type SecretRuleCount struct {
	RuleID string `json:"rule_id"`
	Count  int    `json:"count"`
}

// SecretFinding 은 탐지 하나의 위치다. 비밀값(gitleaks 의 Secret · Match)은 필드로 두지 않는다 —
// 둘 자리가 없으면 실수로라도 저장되지 않는다.
type SecretFinding struct {
	RuleID    string `json:"rule_id"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	// Commit 은 이력 검사(gitleaks git)일 때만 있다. 작업 트리 검사(gitleaks dir)는 커밋을 모른다.
	Commit      string `json:"commit,omitempty"`
	Fingerprint string `json:"fingerprint"`
}

// MaxStoredSecretFindings 는 결과 하나에 남기는 탐지 위치의 상한이다. 건수(FindingsCount)와 규칙별
// 건수는 자르지 않는다 — 오탐 규칙 하나가 수천 건을 내도 행이 커지지 않게 위치만 자른다.
const MaxStoredSecretFindings = 200

// SecretScanSkippedReport 는 스택 정책이 시크릿 탐지를 끈 실행이 남기는 리포트다. 탐지 없음([])과
// 구별해야 한다 — 같은 [] 를 남기면 나중에 정책을 켠 뒤의 동기화가 그 실행을 "통과" 로 기록한다.
// 산출물 업로드가 실패하지 않도록 파일은 남긴다. 스캐폴딩(scaffold/secret_scan.go)이 같은 값을 쓴다.
const SecretScanSkippedReport = `{"nullus_skipped":"policy_off"}`

// SecretScanResultID 는 실행 하나에 대응하는 시크릿 탐지 결과 ID 다.
func SecretScanResultID(deploymentID string) string {
	return "secretscan_" + deploymentID
}

// SecretScanReport 는 gitleaks JSON 리포트를 요약한 것이다.
type SecretScanReport struct {
	// Count 는 전체 탐지 건수다.
	Count int
	// Rules 는 규칙별 건수다 — 건수가 많은 규칙부터, 같으면 규칙 이름 순.
	Rules []SecretRuleCount
	// Findings 는 리포트 순서대로 앞의 MaxStoredSecretFindings 건이다.
	Findings []SecretFinding
	// Skipped 는 정책 off 로 스캔하지 않은 실행의 리포트(SecretScanSkippedReport)라는 뜻이다. 기록하지 않는다.
	Skipped bool
}

// gitleaksFinding 은 gitleaks 리포트 항목에서 읽는 필드다. Secret · Match 는 일부러 두지 않는다 —
// --redact 로 가려져 있어도 디코딩조차 하지 않는다.
type gitleaksFinding struct {
	RuleID      string `json:"RuleID"`
	File        string `json:"File"`
	StartLine   int    `json:"StartLine"`
	Commit      string `json:"Commit"`
	Fingerprint string `json:"Fingerprint"`
}

// ParseGitleaksReport 는 gitleaks 의 JSON 리포트(--report-format json, 탐지 배열)를 요약한다.
//
// 탐지가 없으면 gitleaks 는 [] 를 쓴다. 정책 off 인 스크립트는 SecretScanSkippedReport 를 쓰고, 그것은
// Skipped 로 돌려준다. 빈 파일이나 그 밖의 것은 형식 오류다 — 0 건으로 읽으면 깨진 리포트가 "탐지 없음" 이
// 된다. 이 표식 전에 스캐폴딩한 파이프라인은 off 에서도 [] 를 쓰므로, 정의를 갱신하기 전까지는 구별되지 않는다.
func ParseGitleaksReport(raw []byte) (*SecretScanReport, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "{") {
		var marker struct {
			NullusSkipped string `json:"nullus_skipped"`
		}
		if err := json.Unmarshal([]byte(trimmed), &marker); err == nil && strings.TrimSpace(marker.NullusSkipped) != "" {
			return &SecretScanReport{Rules: []SecretRuleCount{}, Findings: []SecretFinding{}, Skipped: true}, nil
		}
		return nil, fmt.Errorf("gitleaks 리포트 형식 오류: JSON 배열이 아닙니다")
	}
	if !strings.HasPrefix(trimmed, "[") {
		return nil, fmt.Errorf("gitleaks 리포트 형식 오류: JSON 배열이 아닙니다")
	}
	var items []gitleaksFinding
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, fmt.Errorf("gitleaks 리포트 형식 오류: %w", err)
	}

	r := &SecretScanReport{Count: len(items), Rules: []SecretRuleCount{}, Findings: []SecretFinding{}}
	byRule := map[string]int{}
	for i, it := range items {
		rule := strings.TrimSpace(it.RuleID)
		byRule[rule]++
		if i < MaxStoredSecretFindings {
			r.Findings = append(r.Findings, SecretFinding{
				RuleID:      rule,
				File:        strings.TrimSpace(it.File),
				StartLine:   it.StartLine,
				Commit:      strings.TrimSpace(it.Commit),
				Fingerprint: strings.TrimSpace(it.Fingerprint),
			})
		}
	}
	for rule, n := range byRule {
		r.Rules = append(r.Rules, SecretRuleCount{RuleID: rule, Count: n})
	}
	sort.Slice(r.Rules, func(i, j int) bool {
		if r.Rules[i].Count != r.Rules[j].Count {
			return r.Rules[i].Count > r.Rules[j].Count
		}
		return r.Rules[i].RuleID < r.Rules[j].RuleID
	})
	return r, nil
}

// SecretScanGate 는 시크릿 탐지 단계 결과와 탐지 건수로 판정한다(설계 §8 G2 표).
//
// gitleaks 리포트에는 종료 코드가 없어 추론한다. 스크립트(scaffold/secret_scan.go)는 탐지 시 정책에
// 따라 차단(exit 1)·경고(exit 0)하고, 스캔 자체를 못 하면 장애 정책을 따른다.
//
//	리포트 없음(findings nil) · 끝남 → error  — 스캔했다는 근거가 없다(GateWhenReportMissing 과 같다)
//	탐지 > 0 · 실패             → block
//	탐지 > 0 · 성공             → warn   — 경고 정책이 통과시켰다
//	탐지 0  · 성공             → pass
//	탐지 0  · 실패             → error  — 탐지 없이 실패했다. 스캔이 실패한 것이다
//	스택 정책 off              → 기록하지 않는다(대시보드의 "미적용")
//
// 끝나지 않은 단계(실행 중·대기·취소·건너뜀)는 판정하지 않는다.
func SecretScanGate(stageStatus string, findings *int, policy ScanPolicy) (GateResult, bool) {
	if policy.SecretDetectionOrDefault() == SecretDetectionOff {
		return "", false
	}
	fromStage, ok := GateResultFromStageStatus(stageStatus)
	if !ok {
		return "", false
	}
	if findings == nil {
		return GateResultError, true
	}
	if *findings > 0 {
		if fromStage == GateResultPass {
			return GateResultWarn, true
		}
		return GateResultBlock, true
	}
	if fromStage == GateResultPass {
		return GateResultPass, true
	}
	return GateResultError, true
}
