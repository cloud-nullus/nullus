package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func ghcrTarget() *port.ImageTarget {
	return &port.ImageTarget{
		Kind:        port.RegistryKindGHCR,
		Host:        "ghcr.io",
		Repository:  "ghcr.io/acme/myapp",
		UsernameVar: "GITHUB_ACTOR",
		PasswordVar: "GITHUB_TOKEN",
	}
}

func githubInput() Input {
	return Input{
		AppName:     "myapp",
		Namespace:   "myapp",
		Platform:    port.SCMPlatformGitHub,
		ImageTarget: ghcrTarget(),
	}
}

// 경로가 틀리면 파일은 커밋되지만 Actions 가 영영 돌지 않는다.
func TestRender_GitHubUsesWorkflowPathNotGitLabPath(t *testing.T) {
	files, err := Render(githubInput())
	require.NoError(t, err)

	m := fileMap(t, files)
	assert.Contains(t, m, ".github/workflows/nullus-ci.yml")
	assert.NotContains(t, m, ".gitlab-ci.yml")
}

func TestRender_GitHubWorkflowIsValidYAML(t *testing.T) {
	files, err := Render(githubInput())
	require.NoError(t, err)

	var doc map[string]any
	raw := fileMap(t, files)[".github/workflows/nullus-ci.yml"]
	require.NoError(t, yaml.Unmarshal([]byte(raw), &doc), "워크플로가 유효한 YAML 이어야 한다")

	jobs, ok := doc["jobs"].(map[string]any)
	require.True(t, ok, "jobs 가 있어야 한다")
	assert.Contains(t, jobs, "build")
	assert.Contains(t, jobs, "deploy")
}

// contents:write 가 없으면 deploy 가 매니페스트를 push 하지 못하고,
// packages:write 가 없으면 GHCR push 가 403 으로 죽는다.
func TestRender_GitHubGrantsMinimumRequiredPermissions(t *testing.T) {
	files, err := Render(githubInput())
	require.NoError(t, err)

	var doc struct {
		Permissions map[string]string `json:"permissions"`
	}
	raw := fileMap(t, files)[".github/workflows/nullus-ci.yml"]
	require.NoError(t, yaml.Unmarshal([]byte(raw), &doc))

	assert.Equal(t, "write", doc.Permissions["contents"])
	assert.Equal(t, "write", doc.Permissions["packages"])
}

// GITHUB_ACTOR 를 ${{ secrets.GITHUB_ACTOR }} 로 쓰면 빈 값이 들어가
// docker login 이 사용자명 없이 실행된다. 내장 표현식이어야 한다.
func TestRender_GitHubLoginUsesBuiltinExpressions(t *testing.T) {
	files, err := Render(githubInput())
	require.NoError(t, err)

	wf := fileMap(t, files)[".github/workflows/nullus-ci.yml"]
	assert.Contains(t, wf, "${{ github.actor }}")
	assert.Contains(t, wf, "${{ secrets.GITHUB_TOKEN }}")
	assert.NotContains(t, wf, "${{ secrets.GITHUB_ACTOR }}")
}

// GHCR 이 아닌 레지스트리를 고른 구성에서는 사용자 시크릿을 읽어야 한다.
func TestRender_GitHubFallsBackToUserSecretsForOtherRegistries(t *testing.T) {
	in := githubInput()
	in.ImageTarget = &port.ImageTarget{
		Kind:              port.RegistryKindHarbor,
		Host:              "harbor.example.com",
		Repository:        "harbor.example.com/acme/myapp",
		UsernameVar:       "HARBOR_USERNAME",
		PasswordVar:       "HARBOR_PASSWORD",
		RequiredVariables: []string{"HARBOR_USERNAME", "HARBOR_PASSWORD"},
	}

	files, err := Render(in)
	require.NoError(t, err)

	wf := fileMap(t, files)[".github/workflows/nullus-ci.yml"]
	assert.Contains(t, wf, "${{ secrets.HARBOR_USERNAME }}")
	assert.Contains(t, wf, "${{ secrets.HARBOR_PASSWORD }}")
}

// GITHUB_TOKEN 으로 만든 push 는 워크플로를 재트리거하지 않는다.
// [skip ci] 를 넣으면 커밋 메시지만 지저분해지고 얻는 것이 없다.
func TestRender_GitHubDeployCommitOmitsSkipCIMarker(t *testing.T) {
	files, err := Render(githubInput())
	require.NoError(t, err)

	wf := fileMap(t, files)[".github/workflows/nullus-ci.yml"]
	assert.Contains(t, wf, `git commit -m "chore(deploy): $IMAGE_TAG"`)
	assert.NotContains(t, wf, "[skip ci]")
}

// GitHub 호스티드 러너에는 Docker 데몬이 이미 있다. dind 서비스를 띄우면
// 존재하지 않는 docker:2375 로 붙으려다 죽는다.
func TestRender_GitHubWorkflowHasNoDindPlumbing(t *testing.T) {
	files, err := Render(githubInput())
	require.NoError(t, err)

	wf := fileMap(t, files)[".github/workflows/nullus-ci.yml"]
	assert.NotContains(t, wf, "dind")
	assert.NotContains(t, wf, "DOCKER_HOST")
	assert.NotContains(t, wf, "insecure-registry")
}

// GitHub 에는 리포 범위 토큰이 없다. 되쓰기 토큰을 요구하면 사용자가
// 불필요한 장기 PAT 를 워크플로에 노출하게 된다.
func TestRender_GitHubNeedsNoDeployToken(t *testing.T) {
	files, err := Render(githubInput())
	require.NoError(t, err)

	m := fileMap(t, files)
	assert.NotContains(t, m[".github/workflows/nullus-ci.yml"], DeployTokenVar)
	assert.NotContains(t, m["README.md"], DeployTokenVar)
	assert.Contains(t, m["README.md"], "추가 토큰이 필요 없습니다")
}

func TestRender_GitHubDeployUpdatesManifestTag(t *testing.T) {
	files, err := Render(githubInput())
	require.NoError(t, err)

	wf := fileMap(t, files)[".github/workflows/nullus-ci.yml"]
	assert.Contains(t, wf, "sed -i")
	assert.Contains(t, wf, "deploy/deployment.yaml")
	assert.Contains(t, wf, "needs: build")
}

// 배포 매니페스트·Dockerfile 은 플랫폼과 무관하게 같아야 한다.
// 여기서 갈리면 GitLab→GitHub 이전 시 배포 결과가 달라진다.
func TestRender_GitHubProducesSameDeployManifestsAsGitLab(t *testing.T) {
	gh, err := Render(githubInput())
	require.NoError(t, err)

	glInput := githubInput()
	glInput.Platform = port.SCMPlatformGitLab
	gl, err := Render(glInput)
	require.NoError(t, err)

	ghFiles, glFiles := fileMap(t, gh), fileMap(t, gl)
	for _, path := range []string{"deploy/deployment.yaml", "deploy/service.yaml", "Dockerfile"} {
		assert.Equal(t, glFiles[path], ghFiles[path], "%s 는 플랫폼과 무관해야 한다", path)
	}
}

// Platform 을 비워 둔 기존 호출자는 GitLab 동작을 유지해야 한다.
func TestRender_EmptyPlatformDefaultsToGitLab(t *testing.T) {
	in := githubInput()
	in.Platform = ""

	files, err := Render(in)
	require.NoError(t, err)

	assert.Contains(t, fileMap(t, files), ".gitlab-ci.yml")
}
