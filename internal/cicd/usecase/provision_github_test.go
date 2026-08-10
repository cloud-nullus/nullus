package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/scaffold"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func ghcrResolver() *fakeResolver {
	return &fakeResolver{target: &port.ImageTarget{
		Kind: port.RegistryKindGHCR, Host: "ghcr.io",
		Repository:  "ghcr.io/acme/myapp",
		UsernameVar: "GITHUB_ACTOR", PasswordVar: "GITHUB_TOKEN",
	}}
}

func githubAppInput() ProvisionAppProjectInput {
	in := appInput()
	in.Platform = port.SCMPlatformGitHub
	in.SharedAccessToken = "ghp_org_token"
	return in
}

func commitedPaths(t *testing.T, scm *fakeSCM, projectID string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, spec := range scm.commits[projectID] {
		for _, f := range spec.Files {
			files[f.Path] = f.Content
		}
	}
	return files
}

// GitHub 에는 리포 범위 토큰 API 가 없다. 발급을 시도하면 매번 오류가 나고
// 경고가 쌓여 사용자가 실제 문제를 못 본다.
func TestProvisionAppProject_GitHubIssuesNoProjectTokens(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), ghcrResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), githubAppInput())
	require.NoError(t, err)

	assert.Empty(t, pipe.issued, "토큰 발급을 시도하지 않아야 한다")
	assert.NotContains(t, pipe.vars, DeployTokenVariable, "되쓰기 토큰 변수는 필요 없다")
	assert.Empty(t, out.MissingVariables)
	assert.Empty(t, out.Warnings)
}

// Argo CD 가 private 리포를 읽을 자격증명이 없으면 동기화가
// "authentication required" 로 실패한다. 조직 PAT 를 그대로 써야 한다.
func TestProvisionAppProject_GitHubReusesOrgPATForArgo(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), ghcrResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), githubAppInput())
	require.NoError(t, err)

	assert.Equal(t, "ghp_org_token", out.ArgoReadToken)
}

func TestProvisionAppProject_GitHubWarnsWhenPATMissing(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), ghcrResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	in := githubAppInput()
	in.SharedAccessToken = ""

	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Empty(t, out.ArgoReadToken)
	require.Len(t, out.Warnings, 1)
	assert.Contains(t, out.Warnings[0], "GitHub PAT 가 전달되지 않아")
}

// 워크플로가 .github/workflows 에 없으면 파일은 커밋되지만 Actions 가 돌지 않는다.
func TestProvisionAppProject_GitHubCommitsWorkflowFile(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), ghcrResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), githubAppInput())
	require.NoError(t, err)

	files := commitedPaths(t, scm, out.Project.ID)
	assert.Contains(t, files, scaffold.GitHubWorkflowPath)
	assert.NotContains(t, files, scaffold.GitLabPipelinePath)
	assert.Contains(t, files[scaffold.GitHubWorkflowPath], "ghcr.io/acme/myapp")
}

// GHCR 은 내장 토큰으로 push 하므로 사람이 채울 변수가 없어야 한다.
// 여기에 값이 생기면 "변수를 등록하세요" 안내가 잘못 뜬다.
func TestProvisionAppProject_GitHubGHCRNeedsNoUserSecrets(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), ghcrResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), githubAppInput())
	require.NoError(t, err)

	assert.Empty(t, out.MissingVariables)
	assert.Empty(t, pipe.vars)
}

// GitHub + Harbor 처럼 자격증명이 필요한 조합에서는 여전히 등록해야 한다.
func TestProvisionAppProject_GitHubStillRegistersRequiredRegistrySecrets(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	in := githubAppInput()
	in.RegistryCredentials = map[string]string{
		"HARBOR_USERNAME": "admin",
		"HARBOR_PASSWORD": "Harbor12345",
	}

	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Empty(t, out.MissingVariables)
	assert.Contains(t, pipe.vars, "HARBOR_USERNAME")
	assert.Contains(t, pipe.vars, "HARBOR_PASSWORD")
	// GitLab 의 마스킹 요건은 GitHub 에 없다. "admin" 은 8자 미만이지만
	// Actions 는 등록된 시크릿을 항상 가리므로 경고할 이유가 없다.
	assert.True(t, pipe.vars["HARBOR_USERNAME"].Masked)
	assert.Empty(t, out.Warnings)
}

func TestProvisionAppProject_GitHubReportsMissingRegistrySecrets(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), githubAppInput())
	require.NoError(t, err)

	assert.Equal(t, []string{"HARBOR_PASSWORD", "HARBOR_USERNAME"}, out.MissingVariables)
}

func TestProvisionAppProject_GitHubSurfacesSecretRegistrationFailure(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	pipe.varErr = errors.New("422 invalid secret name")
	uc := NewProvisionAppProject(scm, pipe, res)

	in := githubAppInput()
	in.RegistryCredentials = map[string]string{
		"HARBOR_USERNAME": "admin", "HARBOR_PASSWORD": "Harbor12345",
	}

	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Len(t, out.Warnings, 2)
	assert.Equal(t, []string{"HARBOR_PASSWORD", "HARBOR_USERNAME"}, out.MissingVariables)
}

// GitLab 경로는 예전 동작을 그대로 유지해야 한다.
func TestProvisionAppProject_GitLabPathUnchanged(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)

	assert.Len(t, pipe.issued, 2, "되쓰기 토큰과 Argo 읽기 토큰을 발급해야 한다")
	assert.Contains(t, pipe.vars, DeployTokenVariable)
	assert.Equal(t, "glpat-deploy", out.ArgoReadToken)

	files := commitedPaths(t, scm, out.Project.ID)
	assert.Contains(t, files, scaffold.GitLabPipelinePath)
}

// GitLab 형식 주소를 GitHub 리포에 적어 두면 사람이 그대로 따라 하다 실패한다.
func TestProvisionCommonProject_GitHubReadmeUsesGitHubPackagesHosts(t *testing.T) {
	scm := newFakeSCM()
	uc := NewProvisionCommonProject(scm)

	out, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{
		GroupPath: "acme", Platform: port.SCMPlatformGitHub,
	})
	require.NoError(t, err)

	readme := commitedPaths(t, scm, out.Project.ID)["README.md"]
	assert.Contains(t, readme, "https://npm.pkg.github.com")
	assert.Contains(t, readme, "https://maven.pkg.github.com/acme/common")
	assert.NotContains(t, readme, "/-/packages/npm/")
}

func TestProvisionCommonProject_GitLabReadmeUnchanged(t *testing.T) {
	scm := newFakeSCM()
	uc := NewProvisionCommonProject(scm)

	out, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{GroupPath: "acme"})
	require.NoError(t, err)

	readme := commitedPaths(t, scm, out.Project.ID)["README.md"]
	assert.Contains(t, readme, "/-/packages/npm/")
	assert.NotContains(t, readme, "npm.pkg.github.com")
}

// GHCR 은 사용자명으로 GitHub 계정을 기대한다. GitLab 쪽 토큰 이름을 넣으면
// pull 이 인증 오류로 실패하고, 오류가 이미지 부재처럼 보인다.
func TestPullSecretUsername_UsesOwnerForGitHub(t *testing.T) {
	assert.Equal(t, "acme", pullSecretUsername(&port.SCMBundle{
		Platform: port.SCMPlatformGitHub, GroupPath: "acme",
	}))
	assert.Equal(t, argoReadTokenName, pullSecretUsername(&port.SCMBundle{
		Platform: port.SCMPlatformGitLab, GroupPath: "nullus",
	}))
	// owner 를 모르면 기존 기본값으로 떨어진다 — 빈 사용자명은 Secret 생성을 깨뜨린다.
	assert.Equal(t, argoReadTokenName, pullSecretUsername(&port.SCMBundle{
		Platform: port.SCMPlatformGitHub, GroupPath: "  ",
	}))
}
