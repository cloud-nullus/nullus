package scaffold

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const testSASTToken = "sqa_0123456789abcdef"

// sastRun 은 분석 명령을 가짜 sonar-scanner·curl 로 실행한 결과다.
type sastRun struct {
	code     int
	report   string // 작업 디렉터리에 남은 리포트. 없으면 빈 문자열이다.
	curlArgs string // curl 이 받은 인자(호출마다 한 줄)
	curlAuth string // curl 이 표준입력으로 받은 설정
	trace    string // sh -x 트레이스(표준 오류)
}

type sastFake struct {
	scannerExit int
	// htmlBody 면 curl 이 200 과 함께 HTML 을 준다(프록시·로그인 화면).
	htmlBody bool
	// writeTask 면 스캐너가 실제처럼 .scannerwork/report-task.txt 를 남긴다.
	// 서버에 닿지 못한 분석은 남기지 않는다.
	writeTask bool
	curlFails bool
	env       map[string]string
}

// runSASTScriptWithServer 는 Jenkins 처럼 sh -x 로 돌린다 — 트레이스에 토큰이 찍히는지도 본다.
func runSASTScriptWithServer(t *testing.T, f sastFake) sastRun {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	work := filepath.Join(dir, "work")
	require.NoError(t, os.MkdirAll(work, 0o755))

	scanner := "#!/bin/sh\n"
	if f.writeTask {
		scanner += "mkdir -p .scannerwork\n" +
			"printf 'projectKey=api\\nserverUrl=http://sonarqube:9000\\n" +
			"dashboardUrl=https://sonarqube.example.com/dashboard?id=api\\nceTaskId=ce-1\\n' > .scannerwork/report-task.txt\n"
	}
	scanner += "exit " + strconv.Itoa(f.scannerExit) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "sonar-scanner"), []byte(scanner), 0o755))

	argsFile := filepath.Join(dir, "curl-args")
	authFile := filepath.Join(dir, "curl-auth")
	curl := "#!/bin/sh\n" +
		"echo \"$*\" >> '" + argsFile + "'\n" +
		"cat >> '" + authFile + "'\n"
	if f.curlFails {
		curl += "exit 22\n"
	} else if f.htmlBody {
		// 작업 조회까지는 되고, 판정·지표 조회가 로그인 화면으로 돌려진 경우다.
		curl += `case "$*" in
  *api/ce/task*) printf '%s' '{"task":{"id":"ce-1","status":"SUCCESS","analysisId":"an-1"}}' ;;
  *) printf '%s' '<html><body>Sign in</body></html>' ;;
esac
`
	} else {
		curl += `case "$*" in
  *api/ce/task*) printf '%s' '{"task":{"id":"ce-1","status":"SUCCESS","analysisId":"an-1"}}' ;;
  *analysisId=an-1*) printf '%s' '{"projectStatus":{"status":"ERROR","conditions":[{"status":"ERROR","metricKey":"new_violations","comparator":"GT","errorThreshold":"0","actualValue":"1"}]}}' ;;
  *api/measures/component*) printf '%s' '{"component":{"key":"api","measures":[{"metric":"vulnerabilities","value":"3"},{"metric":"bugs","value":"0"}]}}' ;;
  *) exit 22 ;;
esac
`
	}
	require.NoError(t, os.WriteFile(filepath.Join(bin, "curl"), []byte(curl), 0o755))

	cmd := exec.Command("sh", "-c", "set -eux\n"+strings.Join(sastScriptLines("api"), "\n"))
	cmd.Dir = work
	cmd.Env = []string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		port.SASTServerVariable + "=http://sonarqube:9000",
		port.SASTTokenVariable + "=" + testSASTToken,
	}
	for k, v := range f.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	run := sastRun{trace: stderr.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		run.code = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	report, _ := os.ReadFile(filepath.Join(work, port.SASTReportFile))
	args, _ := os.ReadFile(argsFile)
	auth, _ := os.ReadFile(authFile)
	run.report, run.curlArgs, run.curlAuth = string(report), string(args), string(auth)
	return run
}

// 게이트에 걸린 실행도 리포트를 남긴다 — 무엇에 걸렸는지 보려면 실패한 실행의 결과가 필요하다.
// 이번 분석의 판정을 읽는다(최신 판정이 아니라 analysisId 로) — 동시에 돈 다른 분석이 끼어들지 않는다.
func TestSASTScript_WritesReportWhenGateFails(t *testing.T) {
	run := runSASTScriptWithServer(t, sastFake{scannerExit: 3, writeTask: true})

	assert.Equal(t, 1, run.code, "리포트를 남겨도 차단 판정은 그대로다")
	require.NotEmpty(t, run.report, "리포트가 없다")
	r, err := domain.ParseSASTReport([]byte(run.report))
	require.NoError(t, err, run.report)

	assert.Equal(t, 3, r.ScannerExitCode)
	assert.Equal(t, "api", r.ProjectKey)
	assert.Equal(t, "an-1", r.AnalysisID)
	assert.Equal(t, "https://sonarqube.example.com/dashboard?id=api", r.DashboardURL)
	assert.Equal(t, domain.QualityGateError, r.QualityGateStatus)
	require.Len(t, r.Conditions, 1)
	assert.Equal(t, "new_violations", r.Conditions[0].Metric)
	require.NotNil(t, r.Metrics)
	assert.Equal(t, 3, *r.Metrics.Vulnerabilities)

	assert.Contains(t, run.curlArgs, "api/qualitygates/project_status?analysisId=an-1")
	assert.Contains(t, run.curlArgs, "api/measures/component?component=api")
}

// 분석하지 못한 실행(서버 장애)은 스캐너가 작업을 남기지 않는다. 그래도 리포트를 남겨
// "분석 실패" 를 기록하고, 지난 분석의 지표를 이번 것처럼 끌어오지 않는다.
func TestSASTScript_WritesReportWhenAnalysisFails(t *testing.T) {
	run := runSASTScriptWithServer(t, sastFake{scannerExit: 1})

	assert.Equal(t, 1, run.code)
	r, err := domain.ParseSASTReport([]byte(run.report))
	require.NoError(t, err, run.report)
	assert.Equal(t, 1, r.ScannerExitCode)
	assert.Nil(t, r.Metrics)
	assert.Empty(t, r.QualityGateStatus)
	assert.NotContains(t, run.curlArgs, "api/measures/component")
}

// 리포트는 덤이다. SonarQube 조회가 실패해도 단계의 판정을 바꾸지 않는다.
func TestSASTScript_ReportFailureDoesNotChangeOutcome(t *testing.T) {
	run := runSASTScriptWithServer(t, sastFake{scannerExit: 0, writeTask: true, curlFails: true})

	assert.Equal(t, 0, run.code, "조회 실패로 통과한 분석을 막으면 안 된다")
	r, err := domain.ParseSASTReport([]byte(run.report))
	require.NoError(t, err, run.report)
	assert.Equal(t, 0, r.ScannerExitCode)
	assert.Nil(t, r.Metrics)

	run = runSASTScriptWithServer(t, sastFake{scannerExit: 3, writeTask: true, curlFails: true,
		env: map[string]string{port.SASTOnGateFailureVariable: "warn"}})
	assert.Equal(t, 0, run.code, "경고 정책도 그대로다")
}

// Jenkins 는 sh -xe 로 돌고 토큰이 마스킹되지 않는다(파이프라인 Secret 을 envFrom 으로 받는다).
// 토큰을 curl 인자에 실으면 트레이스와 파드의 프로세스 목록에 그대로 남는다 — 표준입력으로 넘긴다.
func TestSASTScript_TokenStaysOutOfArgsAndTrace(t *testing.T) {
	run := runSASTScriptWithServer(t, sastFake{scannerExit: 0, writeTask: true})

	assert.NotContains(t, run.curlArgs, testSASTToken)
	assert.NotContains(t, run.trace, testSASTToken)
	assert.Contains(t, run.curlAuth, testSASTToken+":", "토큰을 넘기지 않으면 SonarQube 가 401 로 답한다")
}

// Jenkins 의 sh ”'…”' 는 Groovy 문자열이라 역슬래시를 이스케이프로 읽는다 — \( 같은 것은
// Jenkinsfile 컴파일 오류다. 분석 명령에 역슬래시를 쓰지 않는다.
func TestSASTScript_HasNoBackslash(t *testing.T) {
	for _, line := range sastScriptLines("api") {
		assert.NotContains(t, line, `\`, line)
	}
}

// 세 CI 모두 리포트를 실패해도 남긴다. 실행 기록 동기화가 같은 이름으로 읽는다.
func TestRenderPipeline_SASTKeepsReportArtifact(t *testing.T) {
	t.Run("GitLab CI", func(t *testing.T) {
		_, content := renderPipelineFor(sastInput(port.CIPlatformGitLabCI, testSASTEndpoint, ""))
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(content), &doc))
		sast, _ := doc[sastStageID].(map[string]any)
		artifacts, _ := sast["artifacts"].(map[string]any)
		require.NotNil(t, artifacts, "분석 잡이 산출물을 남기지 않는다")
		assert.Equal(t, "always", artifacts["when"])
		assert.Equal(t, []any{port.SASTReportFile}, artifacts["paths"])
	})
	t.Run("GitHub Actions", func(t *testing.T) {
		_, content := renderPipelineFor(sastInput(port.CIPlatformGitHubActions, testSASTEndpoint, ""))
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(content), &doc))
		jobs, _ := doc["jobs"].(map[string]any)
		sast, _ := jobs[sastStageID].(map[string]any)
		steps, _ := sast["steps"].([]any)
		var upload map[string]any
		for _, s := range steps {
			if m, ok := s.(map[string]any); ok && strings.HasPrefix(toString(m["uses"]), "actions/upload-artifact@") {
				upload = m
			}
		}
		require.NotNil(t, upload, "리포트 업로드 단계가 없다")
		assert.Equal(t, "always()", upload["if"])
		with, _ := upload["with"].(map[string]any)
		assert.Equal(t, port.SASTReportArtifact, with["name"])
		assert.Equal(t, port.SASTReportFile, with["path"])
	})
	t.Run("Jenkins", func(t *testing.T) {
		_, content := renderPipelineFor(sastInput(port.CIPlatformJenkins, testSASTEndpoint, ""))
		start := strings.Index(content, "stage('"+sastStageName+"')")
		require.GreaterOrEqual(t, start, 0)
		end := strings.Index(content[start+1:], "stage('")
		require.Greater(t, end, 0)
		stage := content[start : start+1+end]
		assert.Contains(t, stage, "always {")
		assert.Contains(t, stage, `archiveArtifacts artifacts: "`+port.SASTReportFile+`", allowEmptyArchive: true`)
	})
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}

// 프록시나 로그인 화면이 200 으로 HTML 을 주면 curl -f 는 거르지 못한다. 그 본문을 그대로 실으면
// 리포트 JSON 이 깨져 스캐너 종료 코드까지 잃는다 — JSON 이 아니면 null 로 둔다.
func TestSASTScript_NonJSONResponseKeepsReportValid(t *testing.T) {
	run := runSASTScriptWithServer(t, sastFake{scannerExit: 3, writeTask: true, htmlBody: true})

	assert.Equal(t, 1, run.code)
	r, err := domain.ParseSASTReport([]byte(run.report))
	require.NoError(t, err, run.report)
	assert.Equal(t, 3, r.ScannerExitCode)
	assert.Nil(t, r.Metrics)
	assert.Empty(t, r.QualityGateStatus)
}
