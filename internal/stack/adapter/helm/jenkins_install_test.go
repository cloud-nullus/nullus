package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestDefaultChartSpecForStep_Jenkins(t *testing.T) {
	spec, ok := DefaultChartSpecForStep("installing_jenkins")

	require.True(t, ok, "installing_jenkins 에 차트 스펙이 없으면 스텝이 unknown step 으로 죽는다")
	assert.Equal(t, domain.JenkinsReleaseName, spec.ReleaseName)
	assert.Equal(t, "jenkins", spec.ChartName)
	assert.Equal(t, "https://charts.jenkins.io", spec.RepoURL)
	assert.Equal(t, domain.JenkinsChartVersion, spec.Version)
}

func TestIsJenkinsCISelection(t *testing.T) {
	tests := []struct {
		name string
		sel  domain.ToolSelection
		want bool
	}{
		{"Jenkins 선택", domain.ToolSelection{Enabled: true, Name: "Jenkins"}, true},
		{"대소문자 무관", domain.ToolSelection{Enabled: true, Name: "jenkins"}, true},
		{"꺼져 있으면 제외", domain.ToolSelection{Enabled: false, Name: "Jenkins"}, false},
		{"외부 Jenkins 는 설치하지 않는다", domain.ToolSelection{Enabled: true, Name: "Jenkins", Version: "external"}, false},
		{"GitLab CI 는 Jenkins 가 아니다", domain.ToolSelection{Enabled: true, Name: "GitLab CI"}, false},
		{"이름이 비면 기본값 GitLab CI 라 Jenkins 아님", domain.ToolSelection{Enabled: true}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isJenkinsCISelection(tc.sel))
		})
	}
}

// Jenkins 를 CI 로 고르면 gitlab-runner 는 서지 않아야 한다.
// 반대로 Jenkins 스텝은 서야 한다.
func TestJenkinsCISelection_DisablesGitLabRunner(t *testing.T) {
	cfg := domain.StackConfig{}
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Enabled: true, Name: "Gitea"}
	cfg.Pipeline.CIPlatform = domain.ToolSelection{Enabled: true, Name: "Jenkins"}

	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
	o.SetStackConfig(cfg)

	assert.True(t, o.IsStepEnabled("installing_jenkins"))
	assert.False(t, o.IsStepEnabled(stepInstallingRunner),
		"Gitea+Jenkins 스택에 gitlab-runner 가 서면 아무도 고르지 않은 실행기가 뜬다")
}

// 기존 GitLab 경로는 그대로여야 한다.
func TestGitLabCISelection_KeepsRunnerAndSkipsJenkins(t *testing.T) {
	cfg := domain.StackConfig{}
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Enabled: true, Name: "GitLab CE"}
	cfg.Pipeline.CIPlatform = domain.ToolSelection{Enabled: true, Name: "GitLab CI"}

	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
	o.SetStackConfig(cfg)

	assert.True(t, o.IsStepEnabled(stepInstallingRunner))
	assert.False(t, o.IsStepEnabled("installing_jenkins"))
}

// 러너 health check 하드 게이트는 GitLab CI 를 고른 스택에만 적용돼야 한다.
//
// 이 게이트를 좁히지 않으면 Jenkins 스택은 설치가 다 끝나도 gitlab-runner
// 릴리스가 없다는 이유로 completed 에 도달하지 못하고, 그러면 CI/CD 모듈의
// bundle factory 가 파이프라인 생성을 거부한다(state != completed).
func TestRunnerReleaseGate_OnlyAppliesToGitLabCI(t *testing.T) {
	jenkins := domain.StackConfig{}
	jenkins.Pipeline.CIPlatform = domain.ToolSelection{Enabled: true, Name: "Jenkins"}
	assert.False(t, runnerReleaseRequired(jenkins),
		"Jenkins 스택에서 gitlab-runner 릴리스를 요구하면 스택이 영원히 완료되지 않는다")

	gitlab := domain.StackConfig{}
	gitlab.Artifacts.SourceRepository = domain.ToolSelection{Enabled: true, Name: "GitLab CE"}
	gitlab.Pipeline.CIPlatform = domain.ToolSelection{Enabled: true, Name: "GitLab CI"}
	assert.True(t, runnerReleaseRequired(gitlab),
		"GitLab CI 스택은 실행기 없이 완료되면 안 된다")
}

// Jenkins 는 스택 PostgreSQL 을 쓰지 않지만, admin 자격증명은 OpenBao 평면을
// 거쳐야 한다. 또 gitea 플러그인이 Gitea multibranch 스캔에 필요하다.
func TestJenkinsValues_UsesProvisionedAdminSecretAndInstallsGiteaPlugin(t *testing.T) {
	values := DefaultValues("installing_jenkins")
	require.NotEmpty(t, values)

	controller, ok := values["controller"].(map[string]any)
	require.True(t, ok)

	admin, ok := controller["admin"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, domain.JenkinsAdminSecret, admin["existingSecret"],
		"admin 비밀번호를 values 에 평문으로 두면 OpenBao 단일 출처 원칙이 깨진다")

	// 플러그인 목록은 이미지로 옮겼다(deploy/images/jenkins/Dockerfile).
	// 런타임 설치를 켜 두면 같은 다운로드를 다시 해 준비 검사를 넘긴다.
	assert.Equal(t, false, controller["installPlugins"])
}

// 플러그인 목록의 단일 출처는 이미지 Dockerfile 이다. values 에서 옮겨 왔으므로
// 보장도 그쪽을 검사한다 — 목록이 비면 파이프라인·SSO 가 조용히 죽는다.
//
// pipeline-stage-view 는 workflow-aggregator 에 포함되지 않아 따로 필요하다.
// 없으면 화면이 단계를 "실행 정보 없음" 으로만 표시한다.
func TestJenkinsImage_BakesRequiredPlugins(t *testing.T) {
	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "..", "..",
		"deploy", "images", "jenkins", "Dockerfile"))
	require.NoError(t, err, "Jenkins 이미지 Dockerfile 을 찾지 못했다")

	joined := string(dockerfile)
	for _, want := range []string{
		"gitea", "kubernetes", "workflow-aggregator", "configuration-as-code",
		"pipeline-stage-view",
		// SSO 로그인. 이것이 빠지면 JCasC 의 oic 설정이 통째로 무시된다.
		"oic-auth",
	} {
		assert.Containsf(t, joined, want, "%s 플러그인이 이미지에 없다", want)
	}
}
