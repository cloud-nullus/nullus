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

// Jenkins 는 SSO 대상 목록에 아예 없어서, 이 스택의 CI 만 자격을 따로 관리해야
// 했다. JCasC 와 installPlugins 가 이미 있으므로 그 자리에 붙인다.

type stubMultiProvisioner struct{ slug string }

func (s stubMultiProvisioner) ClientIDFor(step string) (string, bool) {
	name, ok := map[string]string{
		"installing_jenkins": "jenkins",
		"installing_gitea":   "gitea",
		"installing_harbor":  "harbor",
		"installing_argocd":  "argocd",
	}[step]
	if !ok {
		return "", false
	}
	return s.slug + "-" + name, true
}
func (s stubMultiProvisioner) ToolSteps() []string {
	return []string{"installing_jenkins", "installing_gitea", "installing_harbor", "installing_argocd"}
}
func (s stubMultiProvisioner) Provision(context.Context, port.SSOClientSpec) error { return nil }
func (s stubMultiProvisioner) Deprovision(context.Context, string) error           { return nil }

func ssoOrchestrator(t *testing.T, issuer string, ssoOn bool) *Orchestrator {
	t.Helper()
	cfg := domain.StackConfig{AccessDomain: "nullus.local"}
	cfg.Pipeline.CIPlatform = domain.ToolSelection{Name: "jenkins", Enabled: true}
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Name: "gitea", Enabled: true}

	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-sso", WithToolOIDCIssuer(issuer))
	o.SetStackConfig(cfg)
	if ssoOn {
		o.ssoFactory = func(string, string) port.SSOProvisioner {
			return stubMultiProvisioner{slug: "nullus-sso"}
		}
	}
	return o
}

func TestJenkinsOIDC_InstallsPluginAndConfiguresSecurityRealm(t *testing.T) {
	values := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true).
		oidcValuesForStep("installing_jenkins")
	require.NotNil(t, values, "Jenkins OIDC values 가 만들어지지 않았다")

	controller := values["controller"].(map[string]any)

	plugins := controller["additionalPlugins"].([]any)
	assert.Contains(t, plugins, "oic-auth", "플러그인이 없으면 JCasC 의 oic 설정이 무시된다")

	scripts := controller["JCasC"].(map[string]any)["configScripts"].(map[string]any)
	script, ok := scripts["nullus-oidc"].(string)
	require.True(t, ok, "oidc JCasC 조각이 없다")

	assert.Contains(t, script, "securityRealm")
	assert.Contains(t, script, "oic")
	assert.Contains(t, script, "nullus-sso-jenkins")

	// 디스커버리 URL 은 최상위가 아니라 serverConfiguration.wellKnown 아래다.
	// 최상위에 두면 JCasC 가 부팅을 중단시켜 Jenkins 가 통째로 못 뜬다
	// (UnknownAttributesException: wellKnownOpenIDConfigurationUrl).
	assert.Contains(t, script, "serverConfiguration:")
	assert.Contains(t, script, "wellKnown:")
	assert.Contains(t, script,
		"http://keycloak.nullus.local:8180/realms/nullus/.well-known/openid-configuration")

	// oic-auth 가 모르는 속성을 넣으면 같은 실패가 돌아온다. 최상위 scopes 는
	// manual 설정용 이름이라 wellKnown 아래에서는 부팅을 막는다.
	assert.NotContains(t, script, "\n      scopes:")
}

// 스코프를 지정하지 않으면 oic-auth 는 디스커버리 문서의 scopes_supported 를
// 통째로 요청한다(플러그인 기본값이 "request all"). 그 목록은 렐름 전체의 것이라
// 이 클라이언트에 할당되지 않은 service_account·basic·acr 까지 들어가고,
// Keycloak 은 로그인을 invalid_scope 로 거절한다:
//
//	/securityRealm/finishLogin?error=invalid_scope&error_description=Invalid+scopes:
//	openid offline_access profile email roles phone address service_account ...
//
// wellKnown 설정에서 스코프를 좁히는 속성 이름은 scopesOverride 다.
func TestJenkinsOIDC_NarrowsScopesToWhatTheClientHas(t *testing.T) {
	script := jenkinsOIDCJCasC("nullus-stack-jenkins", "sso-jenkins",
		"http://keycloak.nullus.local:8180/realms/nullus")

	assert.Contains(t, script, "scopesOverride: \"openid email profile\"")
}

// client secret 은 JCasC 본문에 평문으로 들어가면 안 된다. ESO 가 만든 Secret 을
// 마운트하고 ${...} 로 참조한다.
func TestJenkinsOIDC_ClientSecretIsMountedNotInlined(t *testing.T) {
	values := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true).
		oidcValuesForStep("installing_jenkins")
	controller := values["controller"].(map[string]any)

	secrets := controller["additionalExistingSecrets"].([]any)
	found := false
	for _, s := range secrets {
		m := s.(map[string]any)
		if m["name"] == "nullus-sso-jenkins-oidc" && m["keyName"] == "client-secret" {
			found = true
		}
	}
	assert.True(t, found, "ESO 가 만든 OIDC 시크릿이 마운트되지 않았다: %v", secrets)

	script := controller["JCasC"].(map[string]any)["configScripts"].(map[string]any)["nullus-oidc"].(string)
	assert.Contains(t, script, "${nullus-sso-jenkins-oidc-client-secret}",
		"JCasC 는 {시크릿이름}-{키이름} 으로 참조한다")
}

// 기존 Gitea credential JCasC 조각을 덮어쓰면 multibranch job 이 브랜치를 못 찾는다.
func TestJenkinsOIDC_KeepsExistingJCasCScripts(t *testing.T) {
	o := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true)
	values := o.oidcValuesForStep("installing_jenkins")
	scripts := values["controller"].(map[string]any)["JCasC"].(map[string]any)["configScripts"].(map[string]any)

	// OIDC 조각만 더한다 — 병합은 기존 values 와 합쳐질 때 이뤄진다.
	assert.NotContains(t, scripts, "nullus-gitea-credentials",
		"OIDC values 는 자기 조각만 담아야 병합에서 기존 것을 덮지 않는다")
}

func TestJenkinsOIDC_NoIssuerYieldsNoValues(t *testing.T) {
	assert.Nil(t, ssoOrchestrator(t, "", true).oidcValuesForStep("installing_jenkins"))
}

func TestJenkinsOIDC_WithoutSSOYieldsNoValues(t *testing.T) {
	assert.Nil(t, ssoOrchestrator(t, "http://kc/realms/nullus", false).oidcValuesForStep("installing_jenkins"))
}

var _ = strings.Contains

// Jenkins 도 redirect_uri 를 자기 루트 URL 에서 만든다. 그런데 jenkinsUrl 이
// 아예 설정돼 있지 않아 클러스터 내부 주소가 잡혔고, 등록된 redirect 와 달라
// 로그인이 "Invalid parameter: redirect_uri" 로 막혔다. Harbor 의 externalURL,
// Gitea 의 ROOT_URL 과 같은 실패다.
func TestJenkinsURL_MatchesRegisteredRedirect(t *testing.T) {
	o := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true)
	values := o.jenkinsURLValues()

	controller := values["controller"].(map[string]any)
	assert.Equal(t, "https://jenkins.nullus.local", controller["jenkinsUrl"],
		"등록된 redirect(https://jenkins.<도메인>/securityRealm/finishLogin)와 같은 출처여야 한다")
}

// 접속 도메인이 없으면 정할 수 없다 — 엉뚱한 주소를 박느니 차트 기본값에 맡긴다.
func TestJenkinsURL_EmptyWithoutAccessDomain(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-sso")
	o.SetStackConfig(domain.StackConfig{})
	assert.Empty(t, o.jenkinsURLValues())
}

// SSO 를 안 쓰면 http 다 — Harbor·Gitea 와 같은 판단을 공유해야 한다.
func TestJenkinsURL_SchemeFollowsSharedDecision(t *testing.T) {
	o := ssoOrchestrator(t, "http://kc/realms/nullus", false)
	controller := o.jenkinsURLValues()["controller"].(map[string]any)
	assert.Equal(t, "http://jenkins.nullus.local", controller["jenkinsUrl"])
}

// SSO 를 켜면 JCasC 의 securityRealm: oic 가 기존 보안 영역을 통째로 교체한다.
// 그래서 로컬 admin 계정으로는 더 이상 들어갈 수 없다 — IdP 가 죽으면 아무도
// 못 들어가는 상태다. ArgoCD 에서 고친 것과 같은 문제다.
//
// oic-auth 4.x 는 escapeHatch 프로퍼티로 그 수단을 준다.
func TestJenkinsOIDC_ProvidesEscapeHatch(t *testing.T) {
	values := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true).
		oidcValuesForStep("installing_jenkins")
	script := values["controller"].(map[string]any)["JCasC"].(map[string]any)["configScripts"].(map[string]any)["nullus-oidc"].(string)

	require.Contains(t, script, "escapeHatch", "IdP 가 죽으면 들어갈 수단이 없다")
	// 스키마는 username / secret / group 이다. 이름이 틀리면 JCasC 가 부팅을 막는다.
	assert.Contains(t, script, "username:")
	assert.Contains(t, script, "secret:")
}

// 비밀번호를 JCasC 본문에 평문으로 두면 ConfigMap 으로 남는다. 기존 admin
// 자격증명을 마운트해 참조한다 — 별도 비밀을 하나 더 만들지 않는다.
func TestJenkinsOIDC_EscapeHatchUsesMountedAdminCredential(t *testing.T) {
	values := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true).
		oidcValuesForStep("installing_jenkins")
	controller := values["controller"].(map[string]any)

	var mounted bool
	for _, s := range controller["additionalExistingSecrets"].([]any) {
		m := s.(map[string]any)
		if m["name"] == domain.JenkinsAdminSecret && m["keyName"] == domain.JenkinsAdminPasswordKey {
			mounted = true
		}
	}
	assert.True(t, mounted, "admin 자격증명이 마운트되지 않으면 ${...} 가 풀리지 않는다")

	script := controller["JCasC"].(map[string]any)["configScripts"].(map[string]any)["nullus-oidc"].(string)
	assert.Contains(t, script,
		"${"+domain.JenkinsAdminSecret+"-"+domain.JenkinsAdminPasswordKey+"}")
	assert.NotContains(t, script, "password: \"admin", "평문 비밀번호가 들어갔다")
}
