package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 단계 결과만으로는 "막혔다" 와 "스캔이 못 돌았다" 를 구분할 수 없다 — Trivy 는
// 서버에 못 붙어도 exit 1 로 끝난다. 리포트가 있으면 둘을 가른다.
func TestGateFromStageAndReport(t *testing.T) {
	policy := DefaultScanPolicy()
	critical := &TrivyReportSummary{All: SeverityCounts{Critical: 2}, Fixable: SeverityCounts{Critical: 1}}
	highOnly := &TrivyReportSummary{All: SeverityCounts{High: 3}, Fixable: SeverityCounts{High: 3}}
	clean := &TrivyReportSummary{}

	cases := []struct {
		name   string
		status string
		report *TrivyReportSummary
		want   GateResult
		ok     bool
	}{
		{"성공·깨끗하면 통과", "success", clean, GateResultPass, true},
		{"성공·HIGH 는 경고", "success", highOnly, GateResultWarn, true},
		// 게이트가 막지 않았다(정책 변수가 달랐다). 배포는 나갔으므로 차단으로 세지 않는다.
		{"성공인데 CRITICAL 이면 경고", "success", critical, GateResultWarn, true},
		{"실패·CRITICAL 이면 차단", "failed", critical, GateResultBlock, true},
		{"실패인데 차단 사유가 없으면 스캔 오류", "failed", clean, GateResultError, true},
		{"실패·HIGH 만 있으면 스캔 오류", "failed", highOnly, GateResultError, true},
		{"리포트를 모르면 단계 결과를 따른다(성공)", "success", nil, GateResultPass, true},
		{"리포트를 모르면 단계 결과를 따른다(실패)", "failed", nil, GateResultBlock, true},
		{"도는 중이면 판정하지 않는다", "running", critical, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := GateFromStageAndReport(tc.status, tc.report, policy)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

// 리포트가 확실히 없을 때다. 스캔 명령은 리포트를 먼저 쓰므로, 실패했는데 리포트가
// 없으면 스캐너에 닿지 못한 것이다 — 차단으로 세면 스캐너 장애가 "취약한 배포" 로 보인다.
func TestGateWhenReportMissing(t *testing.T) {
	got, ok := GateWhenReportMissing("failed")
	assert.True(t, ok)
	assert.Equal(t, GateResultError, got)

	// 스캐너 장애를 허용(allow)한 정책에서는 스캔을 못 돌리고도 단계가 성공한다.
	// 리포트 없는 성공을 통과로 적으면 스캔하지 않은 이미지가 "통과" 로 보인다.
	got, ok = GateWhenReportMissing("success")
	assert.True(t, ok)
	assert.Equal(t, GateResultError, got, "리포트가 없으면 스캔했다는 근거가 없다")

	_, ok = GateWhenReportMissing("running")
	assert.False(t, ok)
}
