package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanPolicy_DefaultMatchesDesign(t *testing.T) {
	p := DefaultScanPolicy()
	assert.Equal(t, SeverityCritical, p.BlockSeverity)
	assert.True(t, p.IgnoreUnfixed)
	// 스캐너가 죽은 동안 모든 배포가 초록불로 지나가는 것이 가장 나쁜 실패다 (설계 §6.1).
	assert.Equal(t, UnreachableBlock, p.OnScannerUnreachable)
	assert.NoError(t, p.Validate())
}

func TestScanPolicy_ValidateRejectsUnknownValues(t *testing.T) {
	cases := map[string]ScanPolicy{
		"모르는 심각도":              {BlockSeverity: "SEVERE", OnScannerUnreachable: UnreachableBlock},
		"빈 심각도":                {BlockSeverity: "", OnScannerUnreachable: UnreachableBlock},
		"모르는 장애 동작":            {BlockSeverity: SeverityHigh, OnScannerUnreachable: "ignore"},
		"UNKNOWN 은 기준이 될 수 없다": {BlockSeverity: "UNKNOWN", OnScannerUnreachable: UnreachableBlock},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			assert.True(t, errors.Is(p.Validate(), ErrInvalidScanPolicy))
		})
	}
}

// Trivy 의 --severity 는 "이 등급만" 이다. "이 등급 이상" 을 뜻하려면 위 등급을 모두 적어야 한다.
func TestScanPolicy_TrivySeverities(t *testing.T) {
	cases := map[Severity]string{
		SeverityCritical: "CRITICAL",
		SeverityHigh:     "HIGH,CRITICAL",
		SeverityMedium:   "MEDIUM,HIGH,CRITICAL",
		SeverityLow:      "LOW,MEDIUM,HIGH,CRITICAL",
	}
	for sev, want := range cases {
		assert.Equal(t, want, ScanPolicy{BlockSeverity: sev}.TrivySeverities(), string(sev))
	}
}

// 차단 등급을 낮추면 판정도 그 등급부터 막고, 바로 아래 등급을 경고로 남긴다.
func TestEvaluateGate_FollowsBlockSeverity(t *testing.T) {
	policy := ScanPolicy{BlockSeverity: SeverityHigh, OnScannerUnreachable: UnreachableBlock}

	assert.Equal(t, GateResultBlock, EvaluateGate(&TrivyReportSummary{All: SeverityCounts{High: 1}}, policy))
	assert.Equal(t, GateResultBlock, EvaluateGate(&TrivyReportSummary{All: SeverityCounts{Critical: 1}}, policy))
	assert.Equal(t, GateResultWarn, EvaluateGate(&TrivyReportSummary{All: SeverityCounts{Medium: 2}}, policy))
	assert.Equal(t, GateResultPass, EvaluateGate(&TrivyReportSummary{All: SeverityCounts{Low: 5}}, policy))
}

// 정책을 명시하지 않은 호출(0 값)은 기본 기준(CRITICAL 차단 · HIGH 경고)으로 판정한다.
func TestEvaluateGate_ZeroPolicyUsesDefaultThreshold(t *testing.T) {
	assert.Equal(t, GateResultBlock, EvaluateGate(&TrivyReportSummary{All: SeverityCounts{Critical: 1}}, ScanPolicy{}))
	assert.Equal(t, GateResultWarn, EvaluateGate(&TrivyReportSummary{All: SeverityCounts{High: 1}}, ScanPolicy{}))
}

// 가장 낮은 등급을 막으면 경고할 아래 등급이 없다. 등급을 모르는 것은 세지 않는다.
func TestEvaluateGate_LowBlockHasNoWarnLevel(t *testing.T) {
	policy := ScanPolicy{BlockSeverity: SeverityLow, OnScannerUnreachable: UnreachableBlock}
	assert.Equal(t, GateResultBlock, EvaluateGate(&TrivyReportSummary{All: SeverityCounts{Low: 1}}, policy))
	assert.Equal(t, GateResultPass, EvaluateGate(&TrivyReportSummary{All: SeverityCounts{Unknown: 3}}, policy))
}
