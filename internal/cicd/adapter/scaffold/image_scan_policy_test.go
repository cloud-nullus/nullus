package scaffold

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// scanRun 은 렌더러가 만드는 스캔 명령을 가짜 trivy 로 실제 실행한 결과다.
type scanRun struct {
	exitCode int
	report   string // 리포트 명령 인자
	gate     string // 게이트 명령 인자
}

// runScanScript 는 스크립트 문자열 검사로는 못 보는 것 — 셸이 실제로 어떤
// 인자로 trivy 를 부르고 어떤 코드로 끝나는지 — 를 확인한다.
//
// 가짜 trivy 는 --output 이 있으면 리포트, --exit-code 가 있으면 게이트로 보고
// FAKE_REPORT_EXIT / FAKE_GATE_EXIT 로 결과를 정한다.
func runScanScript(t *testing.T, env map[string]string) scanRun {
	t.Helper()
	dir := t.TempDir()
	logDir := filepath.Join(dir, "log")
	require.NoError(t, os.Mkdir(logDir, 0o755))
	fake := `#!/bin/sh
case " $* " in
  *" --output "*) echo "$*" > '` + logDir + `/report'; exit "${FAKE_REPORT_EXIT:-0}" ;;
  *" --exit-code "*) echo "$*" > '` + logDir + `/gate'; exit "${FAKE_GATE_EXIT:-0}" ;;
esac
exit 99
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "trivy"), []byte(fake), 0o755))

	cmd := exec.Command("sh", "-c", "set -eu\n"+strings.Join(scanScriptLines(), "\n"))
	cmd.Env = []string{
		"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"IMAGE_REPOSITORY=registry.local/shop/api",
		"IMAGE_TAG=abc123",
		port.ScanServerVariable + "=http://trivy.demo.svc:4954",
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	err := cmd.Run()

	run := scanRun{}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		run.exitCode = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	if b, readErr := os.ReadFile(filepath.Join(logDir, "report")); readErr == nil {
		run.report = strings.TrimSpace(string(b))
	}
	if b, readErr := os.ReadFile(filepath.Join(logDir, "gate")); readErr == nil {
		run.gate = strings.TrimSpace(string(b))
	}
	return run
}

// 리포트는 심각도를 거르지 않는다. 거르면 HIGH 가 리포트에 없어 경고(warn)가
// 영원히 나오지 않고 대시보드의 HIGH 건수가 늘 0 이다.
func TestScanScript_ReportKeepsAllSeverities(t *testing.T) {
	run := runScanScript(t, nil)
	require.Equal(t, 0, run.exitCode)
	require.NotEmpty(t, run.report, "리포트 명령이 돌지 않았다")
	assert.NotContains(t, run.report, "--severity")
}

// 변수가 없으면 설계 §6 의 기본값으로 판정한다 — 정책을 아직 한 번도 푸시하지
// 않은 파이프라인이 그렇다.
func TestScanScript_GateDefaultsToDesignPolicy(t *testing.T) {
	run := runScanScript(t, nil)
	assert.Contains(t, run.gate, "--severity CRITICAL")
	assert.Contains(t, run.gate, "--ignore-unfixed")
	assert.Contains(t, run.gate, "registry.local/shop/api:abc123")
}

// 푸시된 정책이 게이트 인자로 그대로 들어간다.
func TestScanScript_GateFollowsPushedPolicy(t *testing.T) {
	run := runScanScript(t, map[string]string{
		port.ScanSeverityVariable:      "HIGH,CRITICAL",
		port.ScanIgnoreUnfixedVariable: "false",
	})
	assert.Contains(t, run.gate, "--severity HIGH,CRITICAL")
	assert.NotContains(t, run.gate, "--ignore-unfixed")
}

func TestScanScript_GateFailureStopsPipeline(t *testing.T) {
	run := runScanScript(t, map[string]string{"FAKE_GATE_EXIT": "1"})
	assert.NotEqual(t, 0, run.exitCode, "차단인데 파이프라인이 계속 간다")
}

// 스캔을 못 돌리면 기본은 차단이다. 게이트 명령까지 가지 않는다.
func TestScanScript_UnreachableScannerBlocksByDefault(t *testing.T) {
	run := runScanScript(t, map[string]string{"FAKE_REPORT_EXIT": "1"})
	assert.NotEqual(t, 0, run.exitCode, "스캐너 장애 동안 배포가 초록불로 지나간다")
	assert.Empty(t, run.gate)
}

// 운영자가 명시적으로 완화하면 통과시킨다 — 긴급 배포용 우회로 (설계 §6.1).
func TestScanScript_UnreachableScannerAllowedByPolicy(t *testing.T) {
	run := runScanScript(t, map[string]string{
		"FAKE_REPORT_EXIT":             "1",
		port.ScanOnUnreachableVariable: "allow",
	})
	assert.Equal(t, 0, run.exitCode)
	assert.Empty(t, run.gate)
}

// 파이프라인 파일에 정책 값을 박지 않는다. 박힌 값은 GitHub env 나 Jenkins
// environment 에서 플랫폼이 푸시한 값보다 앞서, 정책을 바꿔도 반영되지 않는다.
func TestRenderPipeline_DoesNotHardcodePolicyValues(t *testing.T) {
	for _, ci := range []port.CIPlatform{
		port.CIPlatformGitLabCI, port.CIPlatformJenkins, port.CIPlatformGitHubActions,
	} {
		t.Run(string(ci), func(t *testing.T) {
			_, content := renderPipelineFor(scanInput(ci, testScannerEndpoint))
			for _, v := range []string{port.ScanSeverityVariable, port.ScanIgnoreUnfixedVariable, port.ScanOnUnreachableVariable} {
				assert.NotContains(t, content, v+`: "`, "YAML 에 값이 박혔다: %s", v)
				assert.NotContains(t, content, v+` = "`, "Jenkinsfile 에 값이 박혔다: %s", v)
			}
		})
	}
}

// GitHub 은 리포 Actions 변수(vars)로 받는다. 없으면 빈 값이고 스크립트가 기본값을 쓴다.
func TestRenderGitHubWorkflow_ReadsPolicyFromRepositoryVariables(t *testing.T) {
	_, content := renderPipelineFor(scanInput(port.CIPlatformGitHubActions, testScannerEndpoint))
	for _, v := range []string{port.ScanSeverityVariable, port.ScanIgnoreUnfixedVariable, port.ScanOnUnreachableVariable} {
		assert.Contains(t, content, v+": ${{ vars."+v+" }}")
	}
}

// Jenkins 에는 CI 변수 저장소가 없다. 스택 네임스페이스의 ConfigMap 을 스캐너
// 컨테이너가 읽는다 — optional 이라 정책을 한 번도 푸시하지 않아도 파드가 뜬다.
func TestRenderJenkinsfile_ScannerReadsPolicyConfigMap(t *testing.T) {
	content := renderJenkinsfile(scanInput(port.CIPlatformJenkins, testScannerEndpoint))
	scanner := content[strings.Index(content, "- name: scanner"):]
	scanner = scanner[:strings.Index(scanner, "- name: dind")]
	assert.Contains(t, scanner, "configMapRef: {name: "+port.ScanPolicyConfigMapName+", optional: true}")
}
