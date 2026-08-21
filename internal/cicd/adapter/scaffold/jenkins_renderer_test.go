package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func jenkinsTarget() *port.ImageTarget {
	return &port.ImageTarget{
		Kind:        port.RegistryKindHarbor,
		Host:        "registry.otel.internal",
		Repository:  "registry.otel.internal/nullus/api",
		UsernameVar: "HARBOR_USERNAME",
		PasswordVar: "HARBOR_PASSWORD",
	}
}

// SCM 과 CI 는 별개의 축이다. Gitea 는 소스 저장소만 담당하므로 파이프라인
// 정의 형식은 CI 플랫폼(Jenkins)이 정한다.
//
// 축을 하나로 묶어 두면 Gitea 스택에 .gitlab-ci.yml 이 깔리고, Jenkins 는
// 읽을 Jenkinsfile 이 없어 파이프라인이 영영 돌지 않는다.
func TestRenderPipelineFor_GiteaWithJenkins_EmitsJenkinsfile(t *testing.T) {
	path, content := renderPipelineFor(port.SCMPlatformGitea, port.CIPlatformJenkins, "api", jenkinsTarget())

	assert.Equal(t, JenkinsfilePath, path)
	assert.Contains(t, content, "pipeline {")
	assert.Contains(t, content, "registry.otel.internal/nullus/api")
}

// 기존 경로 무회귀 — 축 분리가 GitLab/GitHub 결과를 바꾸면 안 된다.
func TestRenderPipelineFor_GitLabUnchanged(t *testing.T) {
	path, content := renderPipelineFor(port.SCMPlatformGitLab, "", "api", jenkinsTarget())

	assert.Equal(t, GitLabPipelinePath, path)
	assert.Contains(t, content, "stages:")
	assert.NotContains(t, content, "pipeline {")
}

func TestRenderPipelineFor_GitHubUnchanged(t *testing.T) {
	path, content := renderPipelineFor(port.SCMPlatformGitHub, "", "api", jenkinsTarget())

	assert.Equal(t, GitHubWorkflowPath, path)
	assert.Contains(t, content, "runs-on: ubuntu-latest")
	assert.NotContains(t, content, "pipeline {")
}

// CI 를 명시하지 않으면 SCM 의 기본 CI 를 쓴다 — 기존 호출부가 그대로 동작한다.
func TestRenderPipelineFor_EmptyCIFallsBackToPlatformDefault(t *testing.T) {
	gitlabPath, _ := renderPipelineFor(port.SCMPlatformGitLab, "", "api", jenkinsTarget())
	assert.Equal(t, GitLabPipelinePath, gitlabPath)

	// Gitea 는 자체 CI 를 쓰지 않는다. CI 가 비면 Jenkins 로 본다 —
	// Gitea 스택에 .gitlab-ci.yml 을 깔면 아무것도 읽지 않는다.
	giteaPath, _ := renderPipelineFor(port.SCMPlatformGitea, "", "api", jenkinsTarget())
	assert.Equal(t, JenkinsfilePath, giteaPath)
}

// Jenkinsfile 은 배포하지 않는다. GitLab 판과 같은 GitOps 패턴을 따라야 한다 —
// 이미지 태그를 매니페스트에 되쓰고 Argo CD 가 그 커밋을 동기화한다.
func TestJenkinsfile_RewritesTagAndPushesBackInsteadOfDeploying(t *testing.T) {
	content := renderJenkinsfile("api", jenkinsTarget())

	assert.Contains(t, content, "deploy/deployment.yaml",
		"매니페스트 태그를 갱신하지 않으면 Argo CD 가 배포할 새 커밋이 없다")
	assert.Contains(t, content, "git push")
	assert.NotContains(t, content, "kubectl apply",
		"Jenkins 가 직접 배포하면 Argo CD 와 소유권이 충돌한다")
}

// 되커밋이 다시 빌드를 트리거하면 무한 루프가 된다.
// GitLab 은 [skip ci] 로 끊는데 Jenkins multibranch 는 이를 자동 인식하지 않으므로
// 커밋 메시지 규약과 별개로 브랜치 조건을 함께 건다.
func TestJenkinsfile_GuardsAgainstBuildLoop(t *testing.T) {
	content := renderJenkinsfile("api", jenkinsTarget())

	assert.Contains(t, content, "[skip ci]")
	assert.Contains(t, content, "changeset",
		"되커밋이 매니페스트만 바꾸므로 그 변경만으로는 빌드하지 않게 막는다")
}

// 자격증명은 K8s Secret 에서 env 로 들어온다(§3.4). Jenkins Credentials 를
// 1차 저장소로 쓰면 OpenBao 단일 출처가 깨진다.
func TestJenkinsfile_ReadsRegistryCredentialsFromEnv(t *testing.T) {
	content := renderJenkinsfile("api", jenkinsTarget())

	assert.Contains(t, content, "HARBOR_USERNAME")
	assert.Contains(t, content, "HARBOR_PASSWORD")
	assert.NotContains(t, content, "withCredentials",
		"자격증명은 ESO 가 만든 Secret 에서 env 로 들어온다 — Jenkins Credentials 사본을 만들지 않는다")
}

// Render 전체가 Gitea+Jenkins 조합에서 올바른 파일 집합을 낸다.
func TestRender_GiteaJenkins_EmitsJenkinsfileNotGitLabCI(t *testing.T) {
	files, err := Render(Input{
		AppName:     "api",
		Namespace:   "apps",
		Platform:    port.SCMPlatformGitea,
		CIPlatform:  port.CIPlatformJenkins,
		ImageTarget: jenkinsTarget(),
	})
	require.NoError(t, err)

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	joined := strings.Join(paths, " ")

	assert.Contains(t, joined, JenkinsfilePath)
	assert.NotContains(t, joined, GitLabPipelinePath)
	assert.Contains(t, joined, "deploy/deployment.yaml")
	assert.Contains(t, joined, "Dockerfile")
}

// Jenkinsfile 의 envFrom 이 참조하는 Secret 이름은 그 Secret 을 만드는 쪽
// (gitea.CISecretName)과 같아야 한다. 레이어 방향 때문에 서로 import 하지
// 않으므로 값을 양쪽에 고정한다 — 갈라지면 agent 파드가 없는 Secret 을
// 참조해 기동하지 못하고, 실패가 첫 빌드 시점에야 나타난다.
func TestCISecretName_MatchesCredentialPlaneContract(t *testing.T) {
	assert.Equal(t, "nullus-ci-api", ciSecretName("api"))
}

// 파이프라인이 읽는 자격증명 변수 이름은 그것을 채우는 쪽과 같아야 한다.
func TestJenkinsfile_UsesSharedGitCredentialVarNames(t *testing.T) {
	content := renderJenkinsfile("api", jenkinsTarget())

	assert.Equal(t, "GIT_USERNAME", GitUsernameVar)
	assert.Equal(t, "GIT_PASSWORD", GitPasswordVar)
	assert.Contains(t, content, "$"+GitUsernameVar)
	assert.Contains(t, content, "$"+GitPasswordVar)
}

// 워크스페이스를 만든 사용자와 빌드 컨테이너의 사용자가 다르다.
//
// 체크아웃은 jnlp 컨테이너(uid 1000)가 하고 빌드는 docker:27-cli(root)가 하는데,
// git 2.35.2+ 는 다른 사용자 소유의 저장소를 거부한다. 그러면 deploy 단계의
// git config 가 "fatal: not in a git directory" 로 죽는다 — 이미지는 올라갔는데
// 매니페스트 되커밋만 실패해 Argo CD 가 배포할 새 커밋이 영영 없다.
func TestJenkinsfile_MarksWorkspaceAsSafeDirectory(t *testing.T) {
	content := renderJenkinsfile("api", jenkinsTarget())

	assert.Contains(t, content, "safe.directory",
		"소유권 검사를 풀지 않으면 되커밋이 not in a git directory 로 죽는다")
	// 되커밋보다 앞서야 한다.
	assert.Less(t, strings.Index(content, "safe.directory"), strings.Index(content, "git config user.email"))
}

// 자격증명이 빌드 로그에 남으면 안 된다.
//
// GitLab 은 CI 변수를 masked 로 등록해 로그에서 가려 주지만, Jenkins 는 K8s
// Secret 에서 env 로 주입된 값을 마스킹하지 않는다. URL 에 토큰을 박아 push 하면
// 셸 트레이스에 그대로 찍혀, 빌드 로그를 볼 수 있는 사람이 저장소 쓰기 토큰을
// 얻는다.
func TestJenkinsfile_DoesNotLeakCredentialsIntoBuildLog(t *testing.T) {
	content := renderJenkinsfile("api", jenkinsTarget())
	idx := strings.Index(content, "stage('Deploy')")
	require.Positive(t, idx, "deploy 단계를 찾지 못했다")
	deploy := content[idx:]

	// 보호는 두 겹이다: 트레이스를 끄고, 명령 출력도 버린다.
	traceOff := strings.Index(deploy, "set +x")
	push := strings.Index(deploy, "git push")
	require.Positive(t, traceOff, "자격증명을 다루는 구간에서는 셸 트레이스를 꺼야 한다")
	assert.Less(t, traceOff, push, "트레이스를 끄기 전에 push 하면 토큰이 이미 찍힌다")

	line := deploy[push:]
	if end := strings.Index(line, "\\n"); end > 0 {
		line = line[:end]
	}
	assert.Contains(t, line, ">/dev/null 2>&1",
		"push 출력에도 원격 주소가 섞여 나올 수 있다")
}

// Jenkins 는 sh -xe 로 실행한다. 모든 명령이 그대로 로그에 찍히므로, 비밀값이
// 인자나 echo 로 지나가면 빌드 로그를 볼 수 있는 사람이 그것을 그대로 얻는다.
//
// 2026-08-21 운영 빌드 로그에 Harbor 관리자 비밀번호가 평문으로 남았다:
//
//   - echo <레지스트리 관리자 비밀번호가 그대로>
//
// Deploy 단계는 git 자격증명 앞에서 이미 트레이스를 끄고 있었다. Build 단계만
// 빠져 있었다.
func TestJenkinsfile_DoesNotTraceRegistryPassword(t *testing.T) {
	pipeline := renderJenkinsfile("orders-api", &port.ImageTarget{
		Host:        "harbor.nullus.io",
		Repository:  "harbor.nullus.io/nullus/orders-api",
		UsernameVar: "HARBOR_USERNAME",
		PasswordVar: "HARBOR_PASSWORD",
	})

	loginLine := "echo \"$HARBOR_PASSWORD\" | docker login"
	require.Contains(t, pipeline, loginLine)

	// 로그인 줄 앞에서 트레이스가 꺼져 있어야 한다.
	before := pipeline[:strings.Index(pipeline, loginLine)]
	lastOff := strings.LastIndex(before, "set +x")
	require.NotEqual(t, -1, lastOff, "로그인 앞에서 셸 트레이스를 꺼야 한다")

	// 그리고 그 사이에 다시 켜지지 않아야 한다.
	assert.NotContains(t, before[lastOff:], "set -x")

	// 로그인 뒤에는 다시 켠다 — 빌드·push 는 로그로 보여야 진단이 된다.
	after := pipeline[strings.Index(pipeline, loginLine)+len(loginLine):]
	assert.Contains(t, after, "set -x")
}
