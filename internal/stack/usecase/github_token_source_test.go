package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

func githubConfigForTokens() domain.StackConfig {
	return domain.StackConfig{
		Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
		SourceControl:  &domain.SourceControlConfig{Owner: "acme"},
		Artifacts: domain.ArtifactsConfig{
			SourceRepository:  domain.ToolSelection{Name: "GitHub", Enabled: true, Version: "external"},
			ContainerRegistry: domain.ToolSelection{Name: "GHCR", Enabled: true, Version: "external"},
		},
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Name: "GitHub Actions", Enabled: true, Version: "external"},
			CDTool:     domain.ToolSelection{Name: "Argo CD", Enabled: true},
		},
	}
}

func githubStackWithConfig(cfg domain.StackConfig) *domain.Stack {
	return &domain.Stack{
		ID:        "stk_gh",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus-test",
		Config:    cfg,
	}
}

func githubStackForTokens() *domain.Stack {
	return githubStackWithConfig(githubConfigForTokens())
}

func findByProvider(inputs []port.TokenSourceInput, provider string) (port.TokenSourceInput, bool) {
	for _, in := range inputs {
		if in.Provider == provider {
			return in, true
		}
	}
	return port.TokenSourceInput{}, false
}

// 경로가 cicd 모듈이 읽는 곳과 같아야 한다. 어긋나면 컴파일은 통과하고
// 파이프라인 생성에서만 "등록된 PAT 가 없다" 로 드러난다.
func TestBuildStackTokenSourceInputs_GitHubPATUsesSharedPath(t *testing.T) {
	t.Parallel()

	inputs := BuildStackTokenSourceInputs(githubStackForTokens(), "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp_from_wizard"})

	gh, ok := findByProvider(inputs, "github")
	require.True(t, ok, "GitHub 항목이 있어야 한다")
	assert.Equal(t, secrets.GitHubAPITokenPath("dev", "org-1"), gh.Path)
	assert.Equal(t, "kv/nullus/dev/org-1/cicd/github/api-token", gh.Path)
	assert.Equal(t, "ghp_from_wizard", gh.TokenValue)
	assert.Equal(t, "pat", gh.TokenType)
	assert.Equal(t, "cicd", gh.Module)
}

// OpenBao 는 스택마다 배포된다. StackID 가 비면 전역 저장소에 써서
// 스택 범위로 읽는 cicd 모듈이 값을 찾지 못한다.
func TestBuildStackTokenSourceInputs_GitHubPATIsStackScoped(t *testing.T) {
	t.Parallel()

	inputs := BuildStackTokenSourceInputs(githubStackForTokens(), "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp"})

	gh, ok := findByProvider(inputs, "github")
	require.True(t, ok)
	assert.Equal(t, "stk_gh", gh.StackID)
}

// 토큰만으로는 어느 org 에 리포를 만들지 알 수 없다.
func TestBuildStackTokenSourceInputs_GitHubCarriesOwnerMetadata(t *testing.T) {
	t.Parallel()

	cfg := githubConfigForTokens()
	cfg.SourceControl.APIBaseURL = "https://ghe.acme.test/api/v3"
	stack := githubStackWithConfig(cfg)

	inputs := BuildStackTokenSourceInputs(stack, "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp"})

	gh, ok := findByProvider(inputs, "github")
	require.True(t, ok)
	assert.Equal(t, "acme", gh.Metadata["owner"])
	assert.Equal(t, "https://ghe.acme.test/api/v3", gh.Metadata["api_base_url"])
}

// 소유자를 모르면 토큰만 저장해도 리포를 만들 수 없다. 반쪽짜리 행을 남기면
// 연동이 등록된 것처럼 보여 진짜 원인을 가린다.
func TestBuildStackTokenSourceInputs_GitHubSkippedWithoutOwner(t *testing.T) {
	t.Parallel()

	cfg := githubConfigForTokens()
	cfg.SourceControl = nil
	stack := githubStackWithConfig(cfg)

	inputs := BuildStackTokenSourceInputs(stack, "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp"})

	_, ok := findByProvider(inputs, "github")
	assert.False(t, ok)
}

func TestBuildStackTokenSourceInputs_GitHubSkippedWithoutToken(t *testing.T) {
	t.Parallel()

	inputs := BuildStackTokenSourceInputs(githubStackForTokens(), "dev", SourceControlCredentials{})

	_, ok := findByProvider(inputs, "github")
	assert.False(t, ok, "PAT 가 없으면 등록할 것이 없다")
}

// GitLab 스택에 GitHub 자격증명이 딸려 와도 등록하면 안 된다.
func TestBuildStackTokenSourceInputs_GitHubSkippedForGitLabStack(t *testing.T) {
	t.Parallel()

	cfg := githubConfigForTokens()
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Name: "GitLab CE", Enabled: true}
	stack := githubStackWithConfig(cfg)

	inputs := BuildStackTokenSourceInputs(stack, "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp"})

	_, ok := findByProvider(inputs, "github")
	assert.False(t, ok)
}

// 같은 provider 로 소유자 없는 행이 하나 더 생기면, 연동 설정을 읽는 쪽이
// 둘 중 어느 것을 집을지 알 수 없다.
func TestBuildStackTokenSourceInputs_GitHubHasExactlyOneEntry(t *testing.T) {
	t.Parallel()

	inputs := BuildStackTokenSourceInputs(githubStackForTokens(), "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp"})

	count := 0
	for _, in := range inputs {
		if in.Provider == "github" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

// 마법사는 authentication.provider 를 기본값 ” 로 두고, 정규화기는 빈 값이면
// authentication 키를 통째로 뺀다. 그런데 시크릿 평면(OpenBao)은 이 값과 무관하게
// 항상 설치된다 — PostgreSQL/MinIO 가 provisioning_secrets 가 만든 Secret 을
// existingSecret 으로 참조하기 때문이다.
//
// 그래서 이 게이트로 PAT 등록까지 막으면, 사용자가 마법사에 넣은 토큰이 조용히
// 사라진다. 등록 실패가 아니라 "등록할 것이 없음" 이라 경고도 뜨지 않고, 한참 뒤
// 파이프라인 생성에서 "등록된 PAT 가 없다" 로만 드러난다.
func TestBuildStackTokenSourceInputs_GitHubPATSurvivesWithoutAuthConfig(t *testing.T) {
	t.Parallel()

	cfg := githubConfigForTokens()
	cfg.Authentication = nil
	stack := githubStackWithConfig(cfg)

	inputs := BuildStackTokenSourceInputs(stack, "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp_from_wizard"})

	gh, ok := findByProvider(inputs, "github")
	require.True(t, ok, "authentication 설정이 없어도 GitHub PAT 는 등록되어야 한다")
	assert.Equal(t, "ghp_from_wizard", gh.TokenValue)
	assert.Equal(t, secrets.GitHubAPITokenPath("dev", "org-1"), gh.Path)
	assert.Equal(t, "acme", gh.Metadata["owner"])
	assert.Equal(t, "stk_gh", gh.StackID)
	// 시크릿 평면은 항상 OpenBao 다. 비워 두면 값을 어느 저장소에 쓸지 알 수 없다.
	assert.Equal(t, "openbao", gh.SecretManager)
}

// provider 가 빈 문자열인 경우도 같다 — 프론트가 키를 빼기 전 단계의 값이고,
// 다른 경로(API 직접 호출)로는 이 형태가 그대로 올 수 있다.
func TestBuildStackTokenSourceInputs_GitHubPATSurvivesEmptyAuthProvider(t *testing.T) {
	t.Parallel()

	cfg := githubConfigForTokens()
	cfg.Authentication = &domain.AuthenticationConfig{Provider: ""}
	stack := githubStackWithConfig(cfg)

	inputs := BuildStackTokenSourceInputs(stack, "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp"})

	gh, ok := findByProvider(inputs, "github")
	require.True(t, ok, "provider 가 비어도 GitHub PAT 는 등록되어야 한다")
	assert.Equal(t, "openbao", gh.SecretManager)
}

// 반대로 회전 대상 항목은 종전 게이트를 지킨다. 이 항목들은 회전 컨트롤러가
// 나중에 값을 채우는 것을 전제한 빈 경로이므로, 범위를 넓히면 기존 스택에
// 채워지지 않는 행이 늘고 회전 실패 로그만 쌓인다.
func TestBuildStackTokenSourceInputs_ReissueEntriesStillRequireAuthProvider(t *testing.T) {
	t.Parallel()

	cfg := githubConfigForTokens()
	cfg.Authentication = nil
	stack := githubStackWithConfig(cfg)

	inputs := BuildStackTokenSourceInputs(stack, "dev",
		SourceControlCredentials{PersonalAccessToken: "ghp"})

	_, ok := findByProvider(inputs, "argo-cd")
	assert.False(t, ok, "회전 대상 항목은 authentication.provider 가 openbao 일 때만 등록한다")
}

// 등록 경로 전체가 같은 조건을 타는지 본다 — BuildStackTokenSourceInputs 만
// 고치고 호출부가 다른 게이트를 들고 있으면 값이 여전히 저장소에 닿지 않는다.
func TestRegisterStackTokenSources_StoresPATWithoutAuthConfig(t *testing.T) {
	t.Parallel()

	cfg := githubConfigForTokens()
	cfg.Authentication = nil

	registry := &mockTokenRegistry{}
	uc := &InstallStack{tokenRegistry: registry, tokenRegistryEnv: "dev"}

	require.NoError(t, uc.registerStackTokenSources(context.Background(), githubStackWithConfig(cfg),
		SourceControlCredentials{PersonalAccessToken: "ghp_from_wizard"}))

	gh, ok := findByProvider(registry.inputs, "github")
	require.True(t, ok, "authentication 없이 설치해도 PAT 가 저장소로 가야 한다")
	assert.Equal(t, "ghp_from_wizard", gh.TokenValue)
}

func TestRegisterStackTokenSources_PassesWizardCredentials(t *testing.T) {
	t.Parallel()

	registry := &mockTokenRegistry{}
	uc := &InstallStack{tokenRegistry: registry, tokenRegistryEnv: "dev"}

	require.NoError(t, uc.registerStackTokenSources(context.Background(), githubStackForTokens(),
		SourceControlCredentials{PersonalAccessToken: "ghp_from_wizard"}))

	gh, ok := findByProvider(registry.inputs, "github")
	require.True(t, ok)
	assert.Equal(t, "ghp_from_wizard", gh.TokenValue)
}
