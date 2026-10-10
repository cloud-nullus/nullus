package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 파이프라인 SAST 잡이 남기는 리포트다. quality_gate·measures 는 SonarQube 응답을 그대로 담는다
// (api/qualitygates/project_status, api/measures/component — SonarQube 26.9 실측 형식).
const sastReportGateFailed = `{"scanner_exit_code":3,"project_key":"api","ce_task_id":"ce-1","analysis_id":"an-1",
"dashboard_url":"https://sonarqube.example.com/dashboard?id=api",
"quality_gate":{"projectStatus":{"status":"ERROR","conditions":[
 {"status":"OK","metricKey":"new_coverage","comparator":"LT","errorThreshold":"80","actualValue":"0.0"},
 {"status":"ERROR","metricKey":"new_violations","comparator":"GT","errorThreshold":"0","actualValue":"1"}]}},
"measures":{"component":{"key":"api","measures":[
 {"metric":"bugs","value":"0","bestValue":true},
 {"metric":"vulnerabilities","value":"3"},
 {"metric":"code_smells","value":"2"},
 {"metric":"security_hotspots","value":"0"},
 {"metric":"coverage","value":"12.5"},
 {"metric":"duplicated_lines_density","value":"0.0"},
 {"metric":"ncloc","value":"85"}]}}}`

func TestParseSASTReport_GateFailed(t *testing.T) {
	r, err := ParseSASTReport([]byte(sastReportGateFailed))
	require.NoError(t, err)

	assert.Equal(t, 3, r.ScannerExitCode)
	assert.Equal(t, "api", r.ProjectKey)
	assert.Equal(t, "an-1", r.AnalysisID)
	assert.Equal(t, "https://sonarqube.example.com/dashboard?id=api", r.DashboardURL)
	assert.Equal(t, QualityGateError, r.QualityGateStatus)
	require.Len(t, r.Conditions, 2)
	assert.Equal(t, SASTCondition{Metric: "new_violations", Comparator: "GT", Threshold: "0", Actual: "1", Status: "ERROR"}, r.Conditions[1])

	require.NotNil(t, r.Metrics)
	assert.Equal(t, 0, *r.Metrics.Bugs, "0건은 0 이다 — 모름(nil)과 다르다")
	assert.Equal(t, 3, *r.Metrics.Vulnerabilities)
	assert.Equal(t, 2, *r.Metrics.CodeSmells)
	assert.Equal(t, 0, *r.Metrics.SecurityHotspots)
	assert.InDelta(t, 12.5, *r.Metrics.Coverage, 1e-9)
	assert.InDelta(t, 0.0, *r.Metrics.DuplicatedLinesDensity, 1e-9)
	assert.Equal(t, 85, *r.Metrics.Ncloc)
}

// 분석을 못 하면(서버 장애·인증 실패) 스캐너가 작업을 남기지 않아 SonarQube 응답이 없다.
// 지표를 0 으로 채우면 "문제 0건" 으로 읽힌다 — 비워 둔다.
func TestParseSASTReport_AnalysisFailedHasNoMetrics(t *testing.T) {
	r, err := ParseSASTReport([]byte(`{"scanner_exit_code":1,"project_key":"api","ce_task_id":"","analysis_id":"","dashboard_url":"","quality_gate":null,"measures":null}`))
	require.NoError(t, err)

	assert.Equal(t, 1, r.ScannerExitCode)
	assert.Empty(t, r.QualityGateStatus)
	assert.Nil(t, r.Metrics)
	assert.Empty(t, r.Conditions)
}

// 응답에 없는 지표는 nil 이다. 측정하지 않은 커버리지를 0% 로 보이면 안 된다.
func TestParseSASTReport_MissingMeasureIsNil(t *testing.T) {
	r, err := ParseSASTReport([]byte(`{"scanner_exit_code":0,"quality_gate":{"projectStatus":{"status":"OK"}},
"measures":{"component":{"measures":[{"metric":"bugs","value":"1"}]}}}`))
	require.NoError(t, err)

	require.NotNil(t, r.Metrics)
	assert.Equal(t, 1, *r.Metrics.Bugs)
	assert.Nil(t, r.Metrics.Coverage)
	assert.Nil(t, r.Metrics.Vulnerabilities)
	assert.Equal(t, QualityGateOK, r.QualityGateStatus)
}

func TestParseSASTReport_RejectsMalformed(t *testing.T) {
	for _, raw := range []string{``, `not json`, `{}`, `{"project_key":"api"}`} {
		_, err := ParseSASTReport([]byte(raw))
		assert.Error(t, err, "%q", raw)
	}
}

// 판정은 파이프라인 관점이다 — 단계 결과에 이미 정책(차단·경고·장애 허용)이 반영돼 있고,
// 리포트의 스캐너 종료 코드가 "게이트에 걸렸다" 와 "분석을 못 했다" 를 가른다.
func TestSASTGateFromStageAndReport(t *testing.T) {
	report := func(exit int) *SASTReport { return &SASTReport{ScannerExitCode: exit} }

	cases := []struct {
		name   string
		status string
		report *SASTReport
		want   GateResult
		ok     bool
	}{
		{"게이트 통과", "success", report(0), GateResultPass, true},
		{"게이트 실패·차단 정책", "failed", report(3), GateResultBlock, true},
		// 경고 정책이 통과시켰다. 배포는 나갔으므로 차단으로 세지 않지만 통과도 아니다.
		{"게이트 실패·경고 정책", "success", report(3), GateResultWarn, true},
		{"분석 실패·차단", "failed", report(1), GateResultError, true},
		// 장애 허용 정책이 통과시켜도 분석하지 않은 코드를 통과로 적지 않는다.
		{"분석 실패·장애 허용", "success", report(1), GateResultError, true},
		{"도는 중이면 판정하지 않는다", "running", report(3), "", false},
		{"취소는 판정하지 않는다", "canceled", report(3), "", false},
		{"리포트가 없으면 판정하지 않는다", "failed", nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SASTGateFromStageAndReport(tc.status, tc.report)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSASTResultID_IsOnePerRun(t *testing.T) {
	assert.Equal(t, "sast_dep_ci_pl_1_7", SASTResultID("dep_ci_pl_1_7"))
}

// 결과 화면의 SonarQube 링크. 스캐너가 남기는 주소는 스캐너가 붙은 서버 주소(클러스터 안)라
// 브라우저에서 열리지 않는다 — 스택의 공개 주소로 만든다.
func TestSASTDashboardURL(t *testing.T) {
	cases := []struct {
		name, web, key, reported, want string
	}{
		{"공개 주소로 만든다", "https://sonarqube.nullus.local", "app",
			"http://sonarqube.ns.svc.cluster.local:9000/dashboard?id=app", "https://sonarqube.nullus.local/dashboard?id=app"},
		{"프로젝트 키는 쿼리로 이스케이프한다", "https://sonarqube.nullus.local/", "a b&c", "",
			"https://sonarqube.nullus.local/dashboard?id=a+b%26c"},
		{"공개 주소를 모르면 리포트 주소를 쓴다", "", "app",
			"https://sonar.example.com/dashboard?id=app", "https://sonar.example.com/dashboard?id=app"},
		{"클러스터 내 주소는 링크로 남기지 않는다", "", "app",
			"http://sonarqube.ns.svc.cluster.local:9000/dashboard?id=app", ""},
		{"서비스 짧은 이름도 클러스터 안이다", "", "app", "http://sonarqube:9000/dashboard?id=app", ""},
		{"아무것도 모르면 비운다", "", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SASTDashboardURL(tc.web, tc.key, tc.reported))
		})
	}
}
