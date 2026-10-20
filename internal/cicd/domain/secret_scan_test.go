package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitleaks 8.x 의 --redact JSON 리포트 모양이다. Secret · Match 는 가려져 있어도 읽지 않는다.
const gitleaksTwoRules = `[
 {"Description":"GitHub Personal Access Token","StartLine":3,"EndLine":3,"StartColumn":10,"EndColumn":50,
  "Match":"token = REDACTED","Secret":"REDACTED","File":"config/app.env","SymlinkFile":"",
  "Commit":"9f1c2ab","Entropy":4.5,"Author":"dev","Email":"dev@example.com","Date":"2026-10-19T00:00:00Z",
  "Message":"add config","Tags":[],"RuleID":"github-pat","Fingerprint":"9f1c2ab:config/app.env:github-pat:3"},
 {"Description":"AWS","StartLine":7,"Match":"REDACTED","Secret":"REDACTED","File":"deploy/aws.sh",
  "Commit":"9f1c2ab","RuleID":"aws-access-token","Fingerprint":"9f1c2ab:deploy/aws.sh:aws-access-token:7"},
 {"Description":"GitHub Personal Access Token","StartLine":12,"Match":"REDACTED","Secret":"REDACTED",
  "File":"README.md","Commit":"","RuleID":"github-pat","Fingerprint":"README.md:github-pat:12"}
]`

func TestParseGitleaksReport_CountsByRule(t *testing.T) {
	r, err := ParseGitleaksReport([]byte(gitleaksTwoRules))
	require.NoError(t, err)

	assert.Equal(t, 3, r.Count)
	assert.Equal(t, []SecretRuleCount{{RuleID: "github-pat", Count: 2}, {RuleID: "aws-access-token", Count: 1}}, r.Rules,
		"건수가 많은 규칙부터")
	require.Len(t, r.Findings, 3)
	assert.Equal(t, SecretFinding{
		RuleID: "github-pat", File: "config/app.env", StartLine: 3, Commit: "9f1c2ab",
		Fingerprint: "9f1c2ab:config/app.env:github-pat:3",
	}, r.Findings[0])
	assert.Empty(t, r.Findings[2].Commit, "작업 트리 검사는 커밋을 모른다")
}

// 비밀값은 파싱하지도 남기지도 않는다 — 요약을 다시 직렬화해도 Secret · Match 의 흔적이 없어야 한다.
func TestParseGitleaksReport_NeverCarriesSecretValues(t *testing.T) {
	raw := strings.ReplaceAll(gitleaksTwoRules, "REDACTED", "ghp_live_value_must_not_leak")
	r, err := ParseGitleaksReport([]byte(raw))
	require.NoError(t, err)

	out, err := json.Marshal(r)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "ghp_live_value_must_not_leak")
	assert.NotContains(t, strings.ToLower(string(out)), `"secret"`)
	assert.NotContains(t, strings.ToLower(string(out)), `"match"`)
}

// 건수가 같으면 규칙 이름 순이다 — 화면이 동기화마다 순서를 바꾸지 않는다.
func TestParseGitleaksReport_TiesByRuleName(t *testing.T) {
	r, err := ParseGitleaksReport([]byte(`[{"RuleID":"slack-token"},{"RuleID":"aws-access-token"},{"RuleID":"github-pat"}]`))
	require.NoError(t, err)
	assert.Equal(t, []SecretRuleCount{{RuleID: "aws-access-token", Count: 1}, {RuleID: "github-pat", Count: 1}, {RuleID: "slack-token", Count: 1}}, r.Rules)
}

func TestParseGitleaksReport_EmptyMeansNoFindings(t *testing.T) {
	r, err := ParseGitleaksReport([]byte("[]\n"))
	require.NoError(t, err)
	assert.Zero(t, r.Count)
	assert.NotNil(t, r.Rules)
	assert.NotNil(t, r.Findings)
}

// 깨진 리포트를 0 건으로 읽으면 "탐지 없음" 이 된다.
func TestParseGitleaksReport_RejectsMalformed(t *testing.T) {
	for _, raw := range []string{"", "   ", "null", `{"RuleID":"x"}`, `[{"RuleID":`} {
		_, err := ParseGitleaksReport([]byte(raw))
		assert.Error(t, err, "%q", raw)
	}
}

// 오탐 규칙 하나가 수천 건을 내도 행이 커지지 않게 위치만 자른다. 건수는 정확해야 한다.
func TestParseGitleaksReport_CapsStoredFindings(t *testing.T) {
	items := make([]string, 0, MaxStoredSecretFindings+50)
	for i := 0; i < MaxStoredSecretFindings+50; i++ {
		items = append(items, fmt.Sprintf(`{"RuleID":"generic-api-key","File":"f.txt","StartLine":%d,"Fingerprint":"f.txt:generic-api-key:%d"}`, i+1, i+1))
	}
	r, err := ParseGitleaksReport([]byte("[" + strings.Join(items, ",") + "]"))
	require.NoError(t, err)

	assert.Equal(t, MaxStoredSecretFindings+50, r.Count)
	assert.Len(t, r.Findings, MaxStoredSecretFindings)
	assert.Equal(t, []SecretRuleCount{{RuleID: "generic-api-key", Count: MaxStoredSecretFindings + 50}}, r.Rules)
}

func TestSecretScanResultID(t *testing.T) {
	assert.Equal(t, "secretscan_dep_ci_pip_1_7", SecretScanResultID("dep_ci_pip_1_7"))
	assert.Equal(t, "depscan_dep_ci_pip_1_7", DependencyScanResultID("dep_ci_pip_1_7"))
}

// 설계 §8 G2 의 판정 추론 표를 그대로 옮긴다.
func TestSecretScanGate_VerdictTable(t *testing.T) {
	n := func(v int) *int { return &v }
	block := DefaultScanPolicy()
	warn := ScanPolicy{SecretDetection: SecretDetectionWarn}
	off := ScanPolicy{SecretDetection: SecretDetectionOff}

	cases := []struct {
		name     string
		status   string
		findings *int
		policy   ScanPolicy
		want     GateResult
		ok       bool
	}{
		{"리포트 없음 · 성공", "success", nil, block, GateResultError, true},
		{"리포트 없음 · 실패", "failed", nil, block, GateResultError, true},
		{"탐지 · 실패", "failed", n(2), block, GateResultBlock, true},
		{"탐지 · 성공(경고 정책이 통과시켰다)", "success", n(2), warn, GateResultWarn, true},
		{"탐지 · 성공(차단 정책이어도 단계가 통과면 막지 않았다)", "success", n(1), block, GateResultWarn, true},
		{"탐지 0 · 성공", "success", n(0), block, GateResultPass, true},
		{"탐지 0 · 실패(스캔 실패)", "failed", n(0), block, GateResultError, true},
		{"정책 off 는 남기지 않는다", "success", n(0), off, "", false},
		{"정책 off 는 실패여도 남기지 않는다", "failed", nil, off, "", false},
		{"빈 정책은 기본값(차단)이다", "failed", n(1), ScanPolicy{}, GateResultBlock, true},
		{"도는 중", "running", n(1), block, "", false},
		{"취소", "canceled", n(1), block, "", false},
		{"건너뜀", "skipped", nil, block, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SecretScanGate(tc.status, tc.findings, tc.policy)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
