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
	}[step]
	if !ok {
		return "", false
	}
	return s.slug + "-" + name, true
}
func (s stubMultiProvisioner) ToolSteps() []string {
	return []string{"installing_jenkins", "installing_gitea", "installing_harbor"}
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
	assert.Contains(t, script, "http://keycloak.nullus.local:8180/realms/nullus")
	assert.Contains(t, script, "nullus-sso-jenkins")
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
