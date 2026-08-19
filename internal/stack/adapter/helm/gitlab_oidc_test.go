package helm

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// GitLab 은 프로비저너 스펙은 있는데 도구 쪽 설정이 없어, Keycloak 에 클라이언트만
// 만들어지고 GitLab 은 그것을 모르는 상태였다. Harbor 가 그랬던 것과 같다.
//
// GitLab 은 omniauth provider 를 client secret 하나가 아니라 "설정 전체를 담은
// Secret" 으로 받는다. ESO 의 target.template 으로 그 블록을 만든다.

type stubGitLabProvisioner struct{ slug string }

func (s stubGitLabProvisioner) ClientIDFor(step string) (string, bool) {
	if step != "installing_gitlab" {
		return "", false
	}
	return s.slug + "-gitlab", true
}
func (s stubGitLabProvisioner) ToolSteps() []string                                 { return []string{"installing_gitlab"} }
func (s stubGitLabProvisioner) Provision(context.Context, port.SSOClientSpec) error { return nil }
func (s stubGitLabProvisioner) Deprovision(context.Context, string) error           { return nil }

func gitlabOrchestrator(t *testing.T, issuer string, ssoOn bool) *Orchestrator {
	t.Helper()
	cfg := domain.StackConfig{AccessDomain: "nullus.local"}
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Name: "gitlab", Enabled: true}

	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-sso", WithToolOIDCIssuer(issuer))
	o.SetStackConfig(cfg)
	if ssoOn {
		o.ssoFactory = func(string, string) port.SSOProvisioner { return stubGitLabProvisioner{slug: "nullus-sso"} }
	}
	return o
}

func gitlabOIDCSecret(t *testing.T, o *Orchestrator) (ManagedSecret, bool) {
	t.Helper()
	for _, item := range o.ssoManagedSecrets() {
		if item.TargetSecret == GitLabOIDCSecretName {
			return item, true
		}
	}
	return ManagedSecret{}, false
}

func TestGitLabOIDC_BuildsOmniauthProviderSecret(t *testing.T) {
	item, ok := gitlabOIDCSecret(t, gitlabOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true))
	require.True(t, ok, "GitLab omniauth Secret 관리 항목이 없다")

	provider := item.TemplateData["provider"]
	require.NotEmpty(t, provider, "provider 블록이 없으면 GitLab 이 Keycloak 을 모른다")

	assert.Contains(t, provider, "openid_connect")
	assert.Contains(t, provider, "nullus-sso-gitlab")
	assert.Contains(t, provider, "http://keycloak.nullus.local:8180/realms/nullus")
	// 이름은 콜백 경로와 짝이다: /users/auth/<name>/callback
	assert.Contains(t, provider, "keycloak")
}

// client secret 은 템플릿 안에서 참조해야 한다 — 값이 매니페스트에 박히면
// ExternalSecret 정의에 평문이 남는다.
func TestGitLabOIDC_SecretIsReferencedInTemplate(t *testing.T) {
	item, _ := gitlabOIDCSecret(t, gitlabOrchestrator(t, "http://kc/realms/nullus", true))

	assert.Contains(t, item.TemplateData["provider"], "{{ .clientSecret }}",
		"client secret 은 템플릿 참조여야 한다")

	var hasEntry bool
	for _, e := range item.Entries {
		if e.TargetKey == "clientSecret" {
			hasEntry = true
		}
	}
	assert.True(t, hasEntry, "참조할 엔트리가 없으면 템플릿이 빈 값으로 렌더된다")
}

func TestGitLabOIDC_AbsentWithoutSSOOrIssuer(t *testing.T) {
	_, ok := gitlabOIDCSecret(t, gitlabOrchestrator(t, "http://kc/realms/nullus", false))
	assert.False(t, ok, "SSO 를 안 쓰면 만들지 않는다")

	_, ok = gitlabOIDCSecret(t, gitlabOrchestrator(t, "", true))
	assert.False(t, ok, "issuer 가 없으면 만들지 않는다")
}

// 차트가 그 Secret 을 실제로 읽게 해야 한다. 만들어만 두면 아무 일도 일어나지 않는다.
func TestGitLabOIDC_ValuesEnableOmniauthWithSecret(t *testing.T) {
	values := gitlabOrchestrator(t, "http://kc/realms/nullus", true).oidcValuesForStep("installing_gitlab")
	require.NotNil(t, values, "GitLab OIDC values 가 없다")

	omniauth := values["global"].(map[string]any)["appConfig"].(map[string]any)["omniauth"].(map[string]any)
	assert.Equal(t, true, omniauth["enabled"])

	providers := omniauth["providers"].([]any)
	require.Len(t, providers, 1)
	assert.Equal(t, GitLabOIDCSecretName, providers[0].(map[string]any)["secret"])
}

var _ = strings.Contains

// GitLab 은 자기 외부 주소를 global.hosts 에서 만든다. https 를 켜지 않으면
// redirect_uri 가 http 로 나가 등록된 https redirect 와 어긋난다 —
// Harbor·Gitea·Jenkins 에서 세 번 반복된 실패다.
func TestGitLabHosts_SchemeFollowsSharedDecision(t *testing.T) {
	o := gitlabOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true)
	hosts := o.gitlabSharedServiceValues()["global"].(map[string]any)["hosts"].(map[string]any)

	assert.Equal(t, "nullus.local", hosts["domain"])
	assert.Equal(t, true, hosts["https"],
		"등록된 redirect 가 https 인데 hosts.https 가 false 면 로그인이 막힌다")
}

func TestGitLabHosts_StaysHTTPWithoutSSO(t *testing.T) {
	o := gitlabOrchestrator(t, "http://kc/realms/nullus", false)
	hosts := o.gitlabSharedServiceValues()["global"].(map[string]any)["hosts"].(map[string]any)
	assert.Equal(t, false, hosts["https"])
}

// provider 블록의 redirect_uri 도 같은 판단을 써야 한다.
func TestGitLabOIDC_ProviderRedirectUsesSharedScheme(t *testing.T) {
	o := gitlabOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true)
	item, _ := gitlabOIDCSecret(t, o)
	assert.Contains(t, item.TemplateData["provider"],
		o.toolURLScheme()+"://gitlab.nullus.local/users/auth/openid_connect/callback")
}

// GitLab 로그인이 이렇게 깨졌다:
//
//	Could not authenticate you from OpenIDConnect because
//	"Ssl connect returned=1 ... peeraddr=192.168.65.254:8180 state=error: wrong version number"
//
// omniauth-openid_connect 의 client_options 는 scheme=https / port=443 이 기본값이라
// issuer 의 스킴·포트를 따라가지 않는다. 평문 포트에 TLS 로 말해서 나는 오류다.
// issuer 를 분해해 넣어야 한다.
func TestGitLabOIDC_ClientOptionsCarryIssuerSchemeHostPort(t *testing.T) {
	item, _ := gitlabOIDCSecret(t, gitlabOrchestrator(t,
		"http://keycloak.nullus.local:8180/realms/nullus", true))
	provider := item.TemplateData["provider"]

	assert.Contains(t, provider, `scheme: "http"`, "평문 issuer 에 TLS 로 붙으면 wrong version number 가 난다")
	assert.Contains(t, provider, `host: "keycloak.nullus.local"`)
	assert.Contains(t, provider, "port: 8180")
}

// https issuer 는 기본 포트를 쓴다.
func TestGitLabOIDC_ClientOptionsForHTTPSIssuer(t *testing.T) {
	item, _ := gitlabOIDCSecret(t, gitlabOrchestrator(t,
		"https://auth.nullus.io/realms/nullus", true))
	provider := item.TemplateData["provider"]

	assert.Contains(t, provider, `scheme: "https"`)
	assert.Contains(t, provider, `host: "auth.nullus.io"`)
	assert.Contains(t, provider, "port: 443")
}

// openid_connect 젬은 discovery URL 을 항상 HTTPS 로 만든다.
//
//	# swd.rb
//	def self.url_builder
//	  @@url_builder ||= URI::HTTPS
//
// 그래서 issuer 가 http 면 discovery 단계에서 평문 포트에 TLS 로 붙어 깨진다
// (Ssl connect ... wrong version number). client_options 를 고쳐도 소용없다 —
// discovery 가 그보다 먼저 돈다.
//
// http issuer 에서는 discovery 를 끄고 엔드포인트를 직접 준다.
func TestGitLabOIDC_HTTPIssuerSkipsDiscovery(t *testing.T) {
	item, _ := gitlabOIDCSecret(t, gitlabOrchestrator(t,
		"http://keycloak.nullus.local:8180/realms/nullus", true))
	provider := item.TemplateData["provider"]

	assert.Contains(t, provider, "discovery: false",
		"http issuer 에서 discovery 를 켜면 젬이 https 로 붙어 깨진다")
	// 엔드포인트는 issuer 의 경로에서 파생된다. client_options 안에서는 경로다.
	assert.Contains(t, provider, `authorization_endpoint: "/realms/nullus/protocol/openid-connect/auth"`)
	assert.Contains(t, provider, `token_endpoint: "/realms/nullus/protocol/openid-connect/token"`)
	assert.Contains(t, provider, `userinfo_endpoint: "/realms/nullus/protocol/openid-connect/userinfo"`)
	// jwks_uri 만 예외다. 젬이 이 값을 scheme/host/port 와 합치지 않고 그대로
	// HTTP 요청 대상으로 쓴다(http_client.get(client_options.jwks_uri)).
	// 경로만 주면 호스트가 비어 ":80" 으로 붙는다:
	//   Failed to open tcp connection to :80 (... for nil port 80)
	assert.Contains(t, provider,
		`jwks_uri: "http://keycloak.nullus.local:8180/realms/nullus/protocol/openid-connect/certs"`)
}

// https issuer 는 discovery 가 정상 동작한다. 엔드포인트가 바뀌어도 따라가므로
// 그쪽이 더 견고하다.
func TestGitLabOIDC_HTTPSIssuerUsesDiscovery(t *testing.T) {
	item, _ := gitlabOIDCSecret(t, gitlabOrchestrator(t,
		"https://auth.nullus.io/realms/nullus", true))
	provider := item.TemplateData["provider"]

	assert.Contains(t, provider, "discovery: true")
	assert.NotContains(t, provider, "authorization_endpoint")
}

// Keycloak 클라이언트를 PKCE(S256) 요구로 등록하는데 GitLab 이 안 보내면
// 콜백이 이렇게 깨진다:
//
//	Could not authenticate you from OpenIDConnect because
//	"Missing parameter: code challenge method"
//
// omniauth_openid_connect 0.8 은 pkce 를 지원하고(option :pkce, false),
// 기본 pkce_options 의 code_challenge_method 가 이미 S256 이다. 끄는 것보다
// 켜는 쪽이 맞다 — Harbor 는 젬 자체가 안 보내서 뺀 것이고, 여기는 다르다.
func TestGitLabOIDC_EnablesPKCE(t *testing.T) {
	item, _ := gitlabOIDCSecret(t, gitlabOrchestrator(t,
		"http://keycloak.nullus.local:8180/realms/nullus", true))

	assert.Contains(t, item.TemplateData["provider"], "pkce: true",
		"클라이언트가 PKCE 를 요구하는데 안 보내면 콜백이 깨진다")
}
