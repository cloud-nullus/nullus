package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateGate(t *testing.T) {
	policy := DefaultScanPolicy()

	tests := []struct {
		name    string
		fixable SeverityCounts
		all     SeverityCounts
		want    GateResult
	}{
		{"아무것도 없으면 통과", SeverityCounts{}, SeverityCounts{}, GateResultPass},
		{"고칠 수 있는 CRITICAL 이 있으면 차단", SeverityCounts{Critical: 1}, SeverityCounts{Critical: 1}, GateResultBlock},
		{"HIGH 만 있으면 경고", SeverityCounts{High: 2}, SeverityCounts{High: 2}, GateResultWarn},
		// 수정본 없는 CRITICAL 로 막으면 사용자가 할 수 있는 일이 없다.
		// debian:11 이 실제로 CRITICAL 5건 전부 unfixed 였다.
		{"수정본 없는 CRITICAL 은 막지 않는다", SeverityCounts{}, SeverityCounts{Critical: 5}, GateResultPass},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateGate(&TrivyReportSummary{All: tc.all, Fixable: tc.fixable}, policy)
			assert.Equal(t, tc.want, got)
		})
	}
}

// unfixed 를 제외하지 않는 정책이면 전체 건수로 판정한다.
func TestEvaluateGate_CountsUnfixedWhenPolicySaysSo(t *testing.T) {
	policy := DefaultScanPolicy()
	policy.IgnoreUnfixed = false

	got := EvaluateGate(&TrivyReportSummary{All: SeverityCounts{Critical: 5}}, policy)
	assert.Equal(t, GateResultBlock, got)
}

// 리포트가 없으면 판정할 수 없다. 통과로 뭉치면 스캐너가 죽은 동안 모든 배포가
// 초록불로 지나간다.
func TestEvaluateGate_NilSummaryIsError(t *testing.T) {
	assert.Equal(t, GateResultError, EvaluateGate(nil, DefaultScanPolicy()))
}

// 리포트를 아직 못 읽은 실행은 CI 단계 상태로 판정만 남긴다.
// 성공/실패가 아닌 상태는 판정하지 않는다 — 도는 중인 것을 통과로 적으면 안 된다.
func TestGateResultFromStageStatus(t *testing.T) {
	tests := []struct {
		status string
		want   GateResult
		ok     bool
	}{
		{"success", GateResultPass, true},
		{"failed", GateResultBlock, true},
		{"running", "", false},
		{"queued", "", false},
		{"skipped", "", false},
		{"unknown", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			got, ok := GateResultFromStageStatus(tc.status)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
