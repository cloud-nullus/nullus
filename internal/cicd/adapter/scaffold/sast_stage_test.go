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
)

const testSASTEndpoint = "http://sonarqube.sast-demo.svc.cluster.local:9000"

func sastInput(ci port.CIPlatform, sastEndpoint, scanEndpoint string) Input {
	return Input{
		AppName:              "api",
		Platform:             port.SCMPlatformGitLab,
		CIPlatform:           ci,
		ImageTarget:          jenkinsTarget(),
		ImageScannerEndpoint: scanEndpoint,
		SASTServerEndpoint:   sastEndpoint,
	}
}

// 스택에 SonarQube 가 있으면 세 CI 모두 소스 정적 분석 단계를 만든다.
func TestRenderPipeline_IncludesSASTStage(t *testing.T) {
	tests := []struct {
		name string
		ci   port.CIPlatform
		want string
	}{
		{"GitLab CI", port.CIPlatformGitLabCI, "  - " + sastStageID},
		{"Jenkins", port.CIPlatformJenkins, "stage('" + sastStageName + "')"},
		{"GitHub Actions", port.CIPlatformGitHubActions, "  " + sastStageID + ":"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, content := renderPipelineFor(sastInput(tc.ci, testSASTEndpoint, ""))

			assert.Contains(t, content, tc.want, "분석 단계가 없으면 Quality Gate 가 배포를 막을 수 없다")
			assert.Contains(t, content, testSASTEndpoint, "서버 주소가 없으면 스캐너가 붙을 곳을 모른다")
			assert.Contains(t, content, "sonar.qualitygate.wait=true",
				"기다리지 않으면 스캐너는 업로드만 하고 성공으로 끝나 게이트가 판정에 끼지 못한다")
			assert.Contains(t, content, "sonar.projectKey=api")
		})
	}
}

// SonarQube 가 없으면 단계를 만들지 않는다 — 돌지 않을 단계를 선언하면 화면이 그것을
// 성공으로 보여준다(마이그레이션 000070).
func TestRenderPipeline_OmitsSASTStageWithoutSonarQube(t *testing.T) {
	for _, ci := range []port.CIPlatform{port.CIPlatformGitLabCI, port.CIPlatformJenkins, port.CIPlatformGitHubActions} {
		t.Run(string(ci), func(t *testing.T) {
			_, content := renderPipelineFor(sastInput(ci, "", ""))
			assert.NotContains(t, content, "sonar-scanner")
			assert.NotContains(t, strings.ToLower(content), "stage('sast')")
		})
	}
}

// 토큰은 파일에 박지 않는다. 플랫폼이 CI 변수(시크릿)로 등록한다.
func TestRenderPipeline_SASTTokenIsNotInFile(t *testing.T) {
	for _, ci := range []port.CIPlatform{port.CIPlatformGitLabCI, port.CIPlatformJenkins, port.CIPlatformGitHubActions} {
		_, content := renderPipelineFor(sastInput(ci, testSASTEndpoint, ""))
		assert.NotContains(t, content, "sqa_", string(ci))
	}
	_, gh := renderPipelineFor(sastInput(port.CIPlatformGitHubActions, testSASTEndpoint, ""))
	assert.Contains(t, gh, "SONAR_TOKEN: ${{ secrets."+port.SASTTokenVariable+" }}")
}

// 배포는 분석을 기다린다. 분석과 상관없이 배포되면 게이트가 아무것도 막지 못한다.
func TestRenderGitLabPipeline_DeployWaitsForSAST(t *testing.T) {
	cases := []struct {
		name      string
		scan      string
		wantNeeds []string
	}{
		{"SAST 만", "", []string{"build", sastStageID}},
		{"SAST + 이미지 스캔", testScannerEndpoint, []string{imageScanStageID, sastStageID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, content := renderPipelineFor(sastInput(port.CIPlatformGitLabCI, testSASTEndpoint, tc.scan))
			var doc map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(content), &doc))

			deploy, _ := doc["deploy"].(map[string]any)
			assert.ElementsMatch(t, tc.wantNeeds, deploy["needs"])

			sast, _ := doc[sastStageID].(map[string]any)
			require.NotNil(t, sast)
			assert.Equal(t, []any{}, sast["needs"], "분석은 소스만 보므로 빌드를 기다리지 않는다")
			image, _ := sast["image"].(map[string]any)
			assert.Equal(t, []any{""}, image["entrypoint"],
				"스캐너 이미지의 entrypoint 가 잡 스크립트를 가로채면 분석이 돌지 않는다")
		})
	}
}

func TestRenderGitHubWorkflow_DeployWaitsForSAST(t *testing.T) {
	_, content := renderPipelineFor(sastInput(port.CIPlatformGitHubActions, testSASTEndpoint, testScannerEndpoint))
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(content), &doc))
	jobs, _ := doc["jobs"].(map[string]any)
	deploy, _ := jobs["deploy"].(map[string]any)
	assert.ElementsMatch(t, []any{imageScanStageID, sastStageID}, deploy["needs"])
}

// 화면과 실행 기록이 쓰는 단계 이름. 렌더러와 같은 판단이어야 한다.
func TestPipelineStageNames_IncludesSAST(t *testing.T) {
	assert.Equal(t, []string{"Build", sastStageName, "Deploy"},
		PipelineStageNamesFor(sastInput(port.CIPlatformGitLabCI, testSASTEndpoint, "")))
	assert.Equal(t, []string{"Build", sastStageName, imageScanStageName, "Deploy"},
		PipelineStageNamesFor(sastInput(port.CIPlatformGitLabCI, testSASTEndpoint, testScannerEndpoint)))
}

// 분석 명령을 가짜 sonar-scanner 로 실제 실행해, 종료 코드와 정책에 따라 단계가 어떻게
// 끝나는지 본다. sonar-scanner 는 Quality Gate 실패에 3, 서버에 닿지 못하면 1 로 끝난다(실측).
func runSASTScript(t *testing.T, scannerExit int, env map[string]string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	fake := "#!/bin/sh\necho \"$*\" > '" + argsFile + "'\nexit \"${FAKE_SCANNER_EXIT:-0}\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sonar-scanner"), []byte(fake), 0o755))

	cmd := exec.Command("sh", "-c", "set -eu\n"+strings.Join(sastScriptLines("api"), "\n"))
	// 스크립트는 작업 디렉터리에 리포트를 쓴다. 패키지 디렉터리를 더럽히지 않는다.
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"FAKE_SCANNER_EXIT=" + strconv.Itoa(scannerExit),
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	args, _ := os.ReadFile(argsFile)
	return code, string(args)
}

func TestSASTScript_PassesWhenGatePasses(t *testing.T) {
	code, args := runSASTScript(t, 0, nil)
	assert.Equal(t, 0, code)
	assert.Contains(t, args, "-Dsonar.projectKey=api")
	assert.Contains(t, args, "-Dsonar.qualitygate.wait=true")
}

// 기본은 차단이다 — Trivy 와 같다.
func TestSASTScript_GateFailureBlocksByDefault(t *testing.T) {
	code, _ := runSASTScript(t, 3, nil)
	assert.Equal(t, 1, code)
}

// 스택 정책이 warn 이면 기록만 남기고 통과시킨다.
func TestSASTScript_GateFailureWarnsByPolicy(t *testing.T) {
	code, _ := runSASTScript(t, 3, map[string]string{port.SASTOnGateFailureVariable: "warn"})
	assert.Equal(t, 0, code)
}

// 분석을 못 했을 때(서버 장애·인증 실패)는 이미지 스캔과 같은 장애 정책을 따른다.
func TestSASTScript_UnreachableFollowsScannerPolicy(t *testing.T) {
	code, _ := runSASTScript(t, 1, nil)
	assert.Equal(t, 1, code, "기본은 막는다 — 분석기가 죽은 동안 모든 배포가 초록불로 지나가면 안 된다")

	code, _ = runSASTScript(t, 1, map[string]string{port.ScanOnUnreachableVariable: "allow"})
	assert.Equal(t, 0, code)

	// warn 은 Quality Gate 실패에만 적용된다. 분석을 못 한 것을 경고로 넘기지 않는다.
	code, _ = runSASTScript(t, 1, map[string]string{port.SASTOnGateFailureVariable: "warn"})
	assert.Equal(t, 1, code)
}

// Jenkins 에는 CI 변수 저장소가 없다. 정책은 ConfigMap, 토큰은 파이프라인 Secret 에서 온다.
func TestRenderJenkinsfile_SASTReadsPolicyAndToken(t *testing.T) {
	_, content := renderPipelineFor(sastInput(port.CIPlatformJenkins, testSASTEndpoint, ""))
	assert.Contains(t, content, "- name: sonar-scanner")
	assert.Contains(t, content, "configMapRef: {name: "+scanPolicyConfigMap+", optional: true}")
	assert.Contains(t, content, "container('sonar-scanner')")
}

// sonar-scanner 이미지는 비루트로 돈다. GitHub 컨테이너 잡에서 actions/checkout 이 러너 소유의
// 작업 디렉터리에 쓰다 권한 오류로 죽으면, 배포가 분석을 기다리므로 모든 배포가 막힌다.
func TestRenderGitHubWorkflow_SASTContainerRunsAsRoot(t *testing.T) {
	_, content := renderPipelineFor(sastInput(port.CIPlatformGitHubActions, testSASTEndpoint, ""))
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(content), &doc))
	jobs, _ := doc["jobs"].(map[string]any)
	sast, _ := jobs[sastStageID].(map[string]any)
	container, _ := sast["container"].(map[string]any)
	assert.Equal(t, "--user root", container["options"])
}

// Community Edition 은 브랜치 분석이 없다. 기능 브랜치를 분석하면 그 결과가 같은 프로젝트에
// 덮여 기본 브랜치의 판정과 새 코드 기준이 흐트러진다. 배포와 같은 기본 브랜치에서만 돈다.
func TestRenderGitLabPipeline_SASTRunsOnDefaultBranchOnly(t *testing.T) {
	_, content := renderPipelineFor(sastInput(port.CIPlatformGitLabCI, testSASTEndpoint, ""))
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(content), &doc))
	sast, _ := doc[sastStageID].(map[string]any)
	deploy, _ := doc["deploy"].(map[string]any)
	assert.Equal(t, deploy["rules"], sast["rules"])
	assert.NotNil(t, sast["rules"])
}
