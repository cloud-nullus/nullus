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

func secretScanInput(ci port.CIPlatform) Input {
	return Input{AppName: "api", Platform: port.SCMPlatformGitLab, CIPlatform: ci, ImageTarget: jenkinsTarget()}
}

// 시크릿 탐지는 스택 의존이 없다 — 세 CI 모두, 스캐너·SonarQube 가 없는 스택에서도 단계가 있다.
func TestRenderPipeline_IncludesSecretScanStage(t *testing.T) {
	tests := []struct {
		name string
		ci   port.CIPlatform
		want string
	}{
		{"GitLab CI", port.CIPlatformGitLabCI, "  - " + secretScanStageID},
		{"Jenkins", port.CIPlatformJenkins, "stage('" + secretScanStageName + "')"},
		{"GitHub Actions", port.CIPlatformGitHubActions, "  " + secretScanStageID + ":"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, content := renderPipelineFor(secretScanInput(tc.ci))
			assert.Contains(t, content, tc.want, "탐지 단계가 없으면 유출된 시크릿이 배포까지 간다")
			assert.Contains(t, content, "gitleaks git ", "신규 커밋 범위를 본다")
			assert.Contains(t, content, "gitleaks dir ", "기준 커밋이 없으면 작업 트리로 떨어진다")
			assert.Contains(t, content, "--redact", "리포트가 또 다른 유출 경로가 되면 안 된다")
			assert.Contains(t, content, "폐기·회전", "CI 단계는 사후 통보다 — 회전 안내가 없으면 탐지가 의미 없다")
			assert.Contains(t, content, port.SecretScanBaseVariable)
		})
	}
}

// 빌드 앞에 선다 — 시크릿이 든 소스로 이미지를 만들어 레지스트리에 올리지 않는다.
func TestRenderPipeline_BuildWaitsForSecretScan(t *testing.T) {
	_, gitlab := renderPipelineFor(secretScanInput(port.CIPlatformGitLabCI))
	stages := gitlab[strings.Index(gitlab, "stages:"):]
	assert.Less(t, strings.Index(stages, "- "+secretScanStageID), strings.Index(stages, "- build"),
		"GitLab 은 stage 순서가 실행 순서다 — 탐지가 build 앞이어야 한다")

	_, github := renderPipelineFor(secretScanInput(port.CIPlatformGitHubActions))
	var doc struct {
		Jobs map[string]struct {
			Needs any `yaml:"needs"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(github), &doc))
	assert.Equal(t, secretScanStageID, doc.Jobs["build"].Needs, "GitHub 은 needs 로 순서를 만든다")

	jenkins := renderJenkinsfile(secretScanInput(port.CIPlatformJenkins))
	assert.Less(t, strings.Index(jenkins, "stage('"+secretScanStageName+"')"), strings.Index(jenkins, "stage('Build')"),
		"Jenkins 는 stage 선언 순서가 실행 순서다")
}

// 화면과 실행 기록이 쓰는 단계 이름 — 렌더러와 같은 판단이어야 한다.
func TestPipelineStageNames_IncludesSecretScanFirst(t *testing.T) {
	assert.Equal(t, []string{secretScanStageName, "Build", "Deploy"},
		PipelineStageNamesFor(secretScanInput(port.CIPlatformGitLabCI)))
	assert.Equal(t, []string{"Build", "Deploy"}, PipelineStageNames(StageOptions{}),
		"옵션이 꺼진 이름 목록은 그대로다 — 옛 파이프라인의 저장된 stages 와 맞는다")
}

// 이미지는 변수로 읽고 기본값을 파일에 둔다 — 플랫폼이 이미지를 올리면 정의를 다시 쓰지 않아도 따라온다.
func TestRenderPipeline_SecretScanImageFromVariable(t *testing.T) {
	_, gitlab := renderPipelineFor(secretScanInput(port.CIPlatformGitLabCI))
	assert.Contains(t, gitlab, port.SecretScanImageVariable+": \""+shareddomain.GitleaksImage+"\"")
	assert.Contains(t, gitlab, "name: $"+port.SecretScanImageVariable)

	_, github := renderPipelineFor(secretScanInput(port.CIPlatformGitHubActions))
	assert.Contains(t, github, githubImageExpression(port.SecretScanImageVariable, shareddomain.GitleaksImage))
	assert.Contains(t, github, "options: --user root", "비루트 이미지에서 checkout 이 작업 디렉터리에 쓰려면 root 여야 한다")

	jenkins := renderJenkinsfile(secretScanInput(port.CIPlatformJenkins))
	assert.Contains(t, jenkins, "- name: gitleaks")
	assert.Contains(t, jenkins, "configMapRef: {name: "+scanPolicyConfigMap+", optional: true}")
	assert.Contains(t, jenkins, "container('gitleaks')")
}

// 개발자 로컬 훅 설정과 allowlist 는 처음 만들 때 함께 들어간다. 훅의 rev 는 CI 이미지 태그와 같다 —
// 로컬에서 통과한 것이 CI 에서 막히면 안 된다.
func TestRender_IncludesPreCommitHookAndGitleaksConfig(t *testing.T) {
	files, err := Render(secretScanInput(port.CIPlatformGitLabCI))
	require.NoError(t, err)
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}
	require.Contains(t, byPath, PreCommitConfigPath)
	require.Contains(t, byPath, GitleaksConfigPath)
	assert.Contains(t, byPath[PreCommitConfigPath], "rev: v8.30.1")
	assert.Contains(t, byPath[PreCommitConfigPath], "id: gitleaks")
	assert.Contains(t, byPath[GitleaksConfigPath], "useDefault = true")
	assert.NotContains(t, byPath[GitleaksConfigPath], "\npaths = [", "기본 allowlist 는 비어 있다 — 미리 풀어 둔 경로는 진짜 Secret 을 가린다")
	assert.Contains(t, byPath["README.md"], "pre-commit install")
	// 정의 갱신은 이 둘을 덮어쓰지 않는다 — allowlist 는 사용자가 다듬는 파일이다.
	assert.False(t, IsPipelineDefinitionPath(PreCommitConfigPath))
	assert.False(t, IsPipelineDefinitionPath(GitleaksConfigPath))
}

// 탐지 명령을 가짜 gitleaks 로 실제 실행해, 종료 코드와 정책에 따라 단계가 어떻게 끝나는지 본다.
// gitleaks 는 --exit-code 2 로 탐지와 장애(1)를 가른다.
func runSecretScanScript(t *testing.T, scannerExit int, env map[string]string, withRepo bool) (int, string) {
	t.Helper()
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	fake := "#!/bin/sh\necho \"$*\" > '" + argsFile + "'\nexit \"${FAKE_SCANNER_EXIT:-0}\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gitleaks"), []byte(fake), 0o755))
	if withRepo {
		// 기준 커밋이 실제로 있는 저장소 — 신규 커밋 범위를 타야 한다.
		for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "base"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			require.NoError(t, cmd.Run(), strings.Join(args, " "))
		}
		out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		require.NoError(t, err)
		if env == nil {
			env = map[string]string{}
		}
		env[port.SecretScanBaseVariable] = strings.TrimSpace(string(out))
	}
	cmd := exec.Command("sh", "-c", "set -eu\n"+strings.Join(secretScanScriptLines(), "\n"))
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + dir, // git config --global 이 테스트 머신의 설정을 건드리지 않게
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

func TestSecretScanScript_PassesWhenClean(t *testing.T) {
	code, args := runSecretScanScript(t, 0, nil, false)
	assert.Equal(t, 0, code)
	assert.Contains(t, args, "dir ", "기준 커밋이 없으면 작업 트리를 본다")
	assert.Contains(t, args, "--exit-code 2")
	assert.Contains(t, args, "--report-path "+port.SecretScanReportFile)
}

// 기준 커밋이 있으면 그 뒤의 커밋만 본다 — 전체 이력은 이 게이트의 일이 아니다.
func TestSecretScanScript_ScansNewCommitsOnlyWhenBaseExists(t *testing.T) {
	code, args := runSecretScanScript(t, 0, nil, true)
	assert.Equal(t, 0, code)
	assert.Contains(t, args, "git ")
	assert.Contains(t, args, "--log-opts")
	assert.Contains(t, args, "..HEAD")
}

// 첫 푸시의 0000… 기준은 범위로 쓸 수 없다 — 작업 트리로 떨어진다.
func TestSecretScanScript_ZeroBaseFallsBackToWorkingTree(t *testing.T) {
	_, args := runSecretScanScript(t, 0, map[string]string{port.SecretScanBaseVariable: zeroSHA}, false)
	assert.Contains(t, args, "dir ")
	assert.NotContains(t, args, "--log-opts")
}

// 기본은 차단이다. 경고 정책이면 기록만 남기고 통과시킨다.
func TestSecretScanScript_FindingBlocksByDefaultAndWarnsByPolicy(t *testing.T) {
	code, _ := runSecretScanScript(t, 2, nil, false)
	assert.Equal(t, 1, code)
	code, _ = runSecretScanScript(t, 2, map[string]string{port.SecretDetectionVariable: "warn"}, false)
	assert.Equal(t, 0, code)
}

// 끄면 스캐너를 부르지 않고 바로 통과한다 — 리포트는 빈 배열로 남겨 산출물 업로드가 실패하지 않는다.
func TestSecretScanScript_OffSkipsScanner(t *testing.T) {
	code, args := runSecretScanScript(t, 2, map[string]string{port.SecretDetectionVariable: "off"}, false)
	assert.Equal(t, 0, code)
	assert.Empty(t, args, "꺼져 있으면 gitleaks 를 부르지 않는다")
}

// 스캔을 못 한 것(이미지·git 장애)은 이미지 스캔과 같은 장애 정책을 따른다. 경고 정책은 탐지에만 적용된다.
func TestSecretScanScript_ScannerFailureFollowsUnreachablePolicy(t *testing.T) {
	code, _ := runSecretScanScript(t, 1, nil, false)
	assert.Equal(t, 1, code, "기본은 막는다 — 스캐너가 죽은 동안 모든 배포가 초록불로 지나가면 안 된다")
	code, _ = runSecretScanScript(t, 1, map[string]string{port.ScanOnUnreachableVariable: "allow"}, false)
	assert.Equal(t, 0, code)
	code, _ = runSecretScanScript(t, 1, map[string]string{port.SecretDetectionVariable: "warn"}, false)
	assert.Equal(t, 1, code)
}
