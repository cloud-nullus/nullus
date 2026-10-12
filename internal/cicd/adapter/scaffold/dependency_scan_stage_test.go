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

	"github.com/cloud-nullus/draft/internal/cicd/port"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

const testDependencyScanEndpoint = "http://trivy.sca-demo.svc.cluster.local:4954"

func dependencyScanInput(ci port.CIPlatform, scaEndpoint, imageEndpoint string) Input {
	return Input{
		AppName: "api", Platform: port.SCMPlatformGitLab, CIPlatform: ci, ImageTarget: jenkinsTarget(),
		DependencyScanEndpoint: scaEndpoint, ImageScannerEndpoint: imageEndpoint,
	}
}

// 스택에서 의존성 스캔을 고르면 세 CI 모두 dependency-scan 단계를 만든다.
func TestRenderPipeline_IncludesDependencyScanStage(t *testing.T) {
	tests := []struct {
		name string
		ci   port.CIPlatform
		want string
	}{
		{"GitLab CI", port.CIPlatformGitLabCI, "  - " + dependencyScanStageID},
		{"Jenkins", port.CIPlatformJenkins, "stage('" + dependencyScanStageName + "')"},
		{"GitHub Actions", port.CIPlatformGitHubActions, "  " + dependencyScanStageID + ":"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, content := renderPipelineFor(dependencyScanInput(tc.ci, testDependencyScanEndpoint, ""))
			assert.Contains(t, content, tc.want)
			assert.Contains(t, content, "trivy fs ", "이미지가 아니라 저장소(락파일)를 본다")
			assert.Contains(t, content, testDependencyScanEndpoint, "스택 Trivy 서버에 client 로 붙는다 — DB 는 서버에 있다")
			assert.Contains(t, content, "--exit-code 1", "게이트")
			assert.Contains(t, content, dependencyScanReportFile, "리포트는 걸려도 남긴다")
		})
	}
}

// 고르지 않으면 단계가 없다 — 없는 서버에 붙는 단계를 만들면 모든 배포가 막힌다.
func TestRenderPipeline_OmitsDependencyScanWithoutSelection(t *testing.T) {
	for _, ci := range []port.CIPlatform{port.CIPlatformGitLabCI, port.CIPlatformJenkins, port.CIPlatformGitHubActions} {
		_, content := renderPipelineFor(dependencyScanInput(ci, "", ""))
		assert.NotContains(t, content, "trivy fs", string(ci))
	}
}

// 빌드 앞에 선다 — 소스만 보므로 빌드를 기다리지 않고 빨리 실패한다.
func TestRenderPipeline_BuildWaitsForDependencyScan(t *testing.T) {
	_, gitlab := renderPipelineFor(dependencyScanInput(port.CIPlatformGitLabCI, testDependencyScanEndpoint, ""))
	stages := gitlab[strings.Index(gitlab, "stages:"):]
	assert.Less(t, strings.Index(stages, "- "+dependencyScanStageID), strings.Index(stages, "- build"))

	_, github := renderPipelineFor(dependencyScanInput(port.CIPlatformGitHubActions, testDependencyScanEndpoint, ""))
	var doc struct {
		Jobs map[string]struct {
			Needs any `yaml:"needs"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(github), &doc))
	assert.ElementsMatch(t, []any{secretScanStageID, dependencyScanStageID}, doc.Jobs["build"].Needs,
		"GitHub build 은 시크릿 탐지와 의존성 스캔을 함께 기다린다")

	jenkins := renderJenkinsfile(dependencyScanInput(port.CIPlatformJenkins, testDependencyScanEndpoint, ""))
	assert.Less(t, strings.Index(jenkins, "stage('"+dependencyScanStageName+"')"), strings.Index(jenkins, "stage('Build')"))
}

// 의존성 스캔만 골라도 GitLab 의 Trivy 서버·이미지 변수와 Jenkins 의 trivy 컨테이너가 있어야 한다.
func TestRenderPipeline_DependencyScanOnlyHasTrivyPlumbing(t *testing.T) {
	_, gitlab := renderPipelineFor(dependencyScanInput(port.CIPlatformGitLabCI, testDependencyScanEndpoint, ""))
	assert.Contains(t, gitlab, port.ScanServerVariable+": \""+testDependencyScanEndpoint+"\"")
	assert.Contains(t, gitlab, port.ScanImageVariable+": \""+shareddomain.TrivyCLIImage+"\"")
	assert.NotContains(t, gitlab, "  - "+imageScanStageID+"\n", "이미지 스캔은 따로 고르는 단계다")

	jenkins := renderJenkinsfile(dependencyScanInput(port.CIPlatformJenkins, testDependencyScanEndpoint, ""))
	assert.Contains(t, jenkins, "- name: scanner")
	assert.Contains(t, jenkins, "container('scanner')")
}

func TestPipelineStageNames_IncludesDependencyScan(t *testing.T) {
	assert.Equal(t, []string{secretScanStageName, dependencyScanStageName, "Build", imageScanStageName, "Deploy"},
		PipelineStageNamesFor(dependencyScanInput(port.CIPlatformGitLabCI, testDependencyScanEndpoint, testScannerEndpoint)))
}

// oneLockfileReport 는 락파일 하나를 본 trivy fs 리포트다.
const oneLockfileReport = `{"Results":[{"Target":"package-lock.json"}]}`

// 스캔 명령을 가짜 trivy 로 실제 실행해 정책에 따라 단계가 어떻게 끝나는지 본다. 가짜는 리포트 호출
// (--format json)과 게이트 호출(--exit-code)을 따로 흉내 낸다.
//
// GitLab Runner 처럼 `set -eo pipefail` 로 돌린다 — 2026-10-12 실측에서 락파일이 없는 저장소의
// 개수 줄(grep 이 1 로 끝남)이 pipefail 에 걸려 잡이 실패했는데, `sh -c "set -eu"` 로 돌리던
// 테스트는 그것을 보지 못했다.
func runDependencyScanScript(t *testing.T, reportExit, gateExit int, env map[string]string) (int, string) {
	t.Helper()
	return runDependencyScanScriptWithReport(t, oneLockfileReport, reportExit, gateExit, env)
}

func runDependencyScanScriptWithReport(t *testing.T, report string, reportExit, gateExit int, env map[string]string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fake-report.json"), []byte(report), 0o600))
	fake := "#!/bin/sh\necho \"$*\" >> '" + calls + "'\n" +
		"case \"$*\" in *--format*) out=\"\"; prev=\"\"; for a in \"$@\"; do [ \"$prev\" = \"--output\" ] && out=\"$a\"; prev=\"$a\"; done; " +
		"[ -n \"$out\" ] && cat '" + filepath.Join(dir, "fake-report.json") + "' > \"$out\"; exit \"${FAKE_REPORT_EXIT:-0}\";; esac\n" +
		"exit \"${FAKE_GATE_EXIT:-0}\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "trivy"), []byte(fake), 0o755))
	cmd := exec.Command("bash", "-c", "set -euo pipefail\n"+strings.Join(dependencyScanScriptLines(), "\n"))
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"FAKE_REPORT_EXIT=" + strconv.Itoa(reportExit),
		"FAKE_GATE_EXIT=" + strconv.Itoa(gateExit),
		port.ScanServerVariable + "=" + testDependencyScanEndpoint,
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err, string(out))
	}
	got, _ := os.ReadFile(calls)
	return code, string(got) + "\n---\n" + string(out)
}

func TestDependencyScanScript_PassesWhenClean(t *testing.T) {
	code, log := runDependencyScanScript(t, 0, 0, nil)
	assert.Equal(t, 0, code)
	assert.Contains(t, log, "fs --server "+testDependencyScanEndpoint)
	assert.Contains(t, log, "--severity CRITICAL", "기본 차단 심각도는 이미지 스캔과 같다")
	assert.Contains(t, log, "--ignore-unfixed")
	assert.Contains(t, log, "의존성 스캔 대상(락파일): 1 개")
}

// 락파일이 없는 저장소(스캐폴딩 직후)는 검사할 것이 없어 통과하고, 그렇다고 말한다. trivy 는 이때
// Results 가 없는 리포트를 쓴다.
func TestDependencyScanScript_PassesAndSaysSoWithoutLockfiles(t *testing.T) {
	code, log := runDependencyScanScriptWithReport(t, `{"SchemaVersion":2,"ArtifactName":".","ArtifactType":"filesystem"}`, 0, 0, nil)
	assert.Equal(t, 0, code, log)
	assert.Contains(t, log, "의존성 스캔 대상(락파일): 0 개")
	assert.Contains(t, log, "락파일을 커밋하세요")
}

// 게이트에 걸리면 막는다. 차단 심각도는 이미지 스캔과 같은 정책 변수를 읽는다.
func TestDependencyScanScript_BlocksOnFindingsAndReadsSharedPolicy(t *testing.T) {
	code, _ := runDependencyScanScript(t, 0, 1, nil)
	assert.Equal(t, 1, code)
	_, log := runDependencyScanScript(t, 0, 0, map[string]string{
		port.ScanSeverityVariable: "HIGH,CRITICAL", port.ScanIgnoreUnfixedVariable: "false",
	})
	assert.Contains(t, log, "--severity HIGH,CRITICAL")
	assert.NotContains(t, strings.Split(log, "---")[0], "--ignore-unfixed")
}

// 스캔을 못 한 것(서버 도달 불가)은 장애 정책을 따른다 — 기본은 막는다.
func TestDependencyScanScript_ScannerFailureFollowsUnreachablePolicy(t *testing.T) {
	code, _ := runDependencyScanScript(t, 1, 0, nil)
	assert.Equal(t, 1, code)
	code, _ = runDependencyScanScript(t, 1, 0, map[string]string{port.ScanOnUnreachableVariable: "allow"})
	assert.Equal(t, 0, code)
}
