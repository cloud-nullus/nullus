package helm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 2026-08-21 운영에서 Argo CD 가 "Client not found" 로 막혔다.
//
// 도구에는 OIDC 설정이 들어갔는데 그 클라이언트를 아무도 만들지 않았다. 두 판단이
// 서로 다른 근거를 봤기 때문이다 — 도구 쪽 값(oidcValuesForStep)은 플랫폼이
// Keycloak 을 가리키는지만 보고, 클라이언트 등록 단계는 스택 설정의
// authentication.provider 가 openbao 인지를 봤다.
type ssoGateProvisioner struct{}

func (ssoGateProvisioner) ClientIDFor(step string) (string, bool) { return "demo-" + step, true }
func (ssoGateProvisioner) ToolSteps() []string                    { return []string{"installing_argocd"} }
func (ssoGateProvisioner) Provision(context.Context, port.SSOClientSpec) error {
	return nil
}
func (ssoGateProvisioner) Deprovision(context.Context, string) error { return nil }

func ssoGateOrchestrator(t *testing.T, cfg domain.StackConfig, issuer string, withFactory bool) *Orchestrator {
	t.Helper()
	opts := []OrchestratorOption{}
	if issuer != "" {
		opts = append(opts, WithToolOIDCIssuer(issuer))
	}
	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus-demo", opts...)
	o.SetStackConfig(cfg)
	if withFactory {
		o.SetSSOProvisionerFactory(func(string, string) port.SSOProvisioner { return ssoGateProvisioner{} })
	}
	return o
}

// 스택이 인증 공급자를 고르지 않아도, 플랫폼이 클라이언트를 만들 수 있으면 만든다.
// 그러지 않으면 도구만 SSO 로 설정되고 클라이언트는 없는 상태가 된다.
func TestProvisioningSSOEnabled_WhenPlatformCanRegisterClients(t *testing.T) {
	cfg := domain.StackConfig{AccessDomain: "nullus.io"}
	o := ssoGateOrchestrator(t, cfg, "https://auth.nullus.io/realms/nullus", true)

	assert.True(t, o.isStepEnabled("provisioning_sso"))
}

// 예전 조건. 이제 인증 공급자 선택은 이 판단과 무관하다.
func TestProvisioningSSOEnabled_DoesNotDependOnAuthenticationProvider(t *testing.T) {
	withBao := domain.StackConfig{AccessDomain: "nullus.io", Authentication: &domain.AuthenticationConfig{Provider: "openbao"}}
	without := domain.StackConfig{AccessDomain: "nullus.io"}

	issuer := "https://auth.nullus.io/realms/nullus"
	assert.Equal(t,
		ssoGateOrchestrator(t, withBao, issuer, true).isStepEnabled("provisioning_sso"),
		ssoGateOrchestrator(t, without, issuer, true).isStepEnabled("provisioning_sso"),
	)
}

// 플랫폼이 Keycloak 을 모르면 도구에도 OIDC 를 넣지 않으므로 등록할 것도 없다.
func TestProvisioningSSODisabled_WithoutIssuerOrFactory(t *testing.T) {
	cfg := domain.StackConfig{AccessDomain: "nullus.io"}

	assert.False(t, ssoGateOrchestrator(t, cfg, "", true).isStepEnabled("provisioning_sso"),
		"issuer 가 없으면 도구에 OIDC 를 넣지 않는다")
	assert.False(t, ssoGateOrchestrator(t, cfg, "https://auth.nullus.io/realms/nullus", false).isStepEnabled("provisioning_sso"),
		"등록할 수단이 없으면 켜지 않는다")
}

// 핵심 불변식: 도구에 OIDC 값이 들어가면 그 클라이언트를 만드는 단계도 켜져 있어야
// 한다. 이 둘이 갈린 것이 이번 장애였다.
func TestToolOIDCValues_ImplyClientRegistrationStep(t *testing.T) {
	cfg := domain.StackConfig{AccessDomain: "nullus.io"}
	cfg.Pipeline.CDTool = domain.ToolSelection{Name: "argocd", Enabled: true}
	o := ssoGateOrchestrator(t, cfg, "https://auth.nullus.io/realms/nullus", true)

	values := o.oidcValuesForStep("installing_argocd")
	if len(values) == 0 {
		t.Skip("이 도구는 OIDC values 를 만들지 않는다")
	}
	assert.True(t, o.isStepEnabled("provisioning_sso"),
		"도구에 OIDC 를 넣으면서 클라이언트 등록을 건너뛰면 Client not found 가 난다")
}
