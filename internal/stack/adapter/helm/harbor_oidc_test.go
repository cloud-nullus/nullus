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

// Harbor 는 절반만 배선돼 있었다. Keycloak 에 클라이언트가 만들어지고
// <스택>-harbor-oidc 시크릿까지 생기는데, 정작 Harbor 에 알려주는 곳이 없어
// authMode 가 db_auth 로 남았다 — 클라이언트만 뜬 상태라 오히려 오해를 부른다.
//
// Harbor 는 OIDC 를 Helm values 가 아니라 자기 API 로 받는다. 그래서
// oidc-values 의 switch 가 아니라 프로비저닝 Job 에 붙인다.

func harborOrchestrator(t *testing.T, issuer string, ssoOn bool) *Orchestrator {
	t.Helper()
	cfg := domain.StackConfig{AccessDomain: "nullus.local"}
	cfg.Artifacts.ContainerRegistry = domain.ToolSelection{Name: "Harbor", Enabled: true}

	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "ssolive", WithToolOIDCIssuer(issuer))
	o.SetStackConfig(cfg)
	if ssoOn {
		o.ssoFactory = func(string, string) port.SSOProvisioner {
			return stubHarborProvisioner{slug: "ssolive"}
		}
	}
	return o
}

type stubHarborProvisioner struct{ slug string }

func (s stubHarborProvisioner) ClientIDFor(step string) (string, bool) {
	if step != "installing_harbor" {
		return "", false
	}
	return s.slug + "-harbor", true
}
func (s stubHarborProvisioner) ToolSteps() []string { return []string{"installing_harbor"} }
func (s stubHarborProvisioner) Provision(ctx context.Context, spec port.SSOClientSpec) error {
	return nil
}
func (s stubHarborProvisioner) Deprovision(ctx context.Context, step string) error { return nil }

func TestHarborProvision_ConfiguresOIDCWhenSSOEnabled(t *testing.T) {
	manifest := harborOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true).
		harborProvisionManifest("ssolive")

	require.Contains(t, manifest, "oidc_auth", "authMode 를 oidc_auth 로 바꾸지 않으면 db_auth 로 남는다")
	assert.Contains(t, manifest, "ssolive-harbor", "스택 네임스페이스가 붙은 client ID 여야 한다")
	assert.Contains(t, manifest, "http://keycloak.nullus.local:8180/realms/nullus")
	assert.Contains(t, manifest, "/api/v2.0/configurations")
}

// client secret 은 매니페스트에 평문으로 실리면 안 된다. 관리자 비밀번호와 같은
// 이유로 Secret 참조여야 한다(회전돼도 Job 이 최신 값을 본다).
func TestHarborProvision_OIDCSecretIsReferencedNotInlined(t *testing.T) {
	manifest := harborOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true).
		harborProvisionManifest("ssolive")

	assert.Contains(t, manifest, "ssolive-harbor-oidc", "ESO 가 만든 시크릿을 참조해야 한다")
	assert.Contains(t, manifest, "client-secret")
	assert.NotContains(t, manifest, "HARBOR_OIDC_SECRET\n          value:",
		"client secret 이 매니페스트에 평문으로 들어갔다")
}

// SSO 를 쓰지 않는 설치는 예전 그대로여야 한다 — 프로젝트만 만든다.
func TestHarborProvision_WithoutSSOKeepsProjectOnly(t *testing.T) {
	manifest := harborOrchestrator(t, "", false).harborProvisionManifest("ssolive")

	assert.Contains(t, manifest, "/api/v2.0/projects")
	assert.NotContains(t, manifest, "oidc_auth")
	assert.NotContains(t, manifest, "/api/v2.0/configurations")
}

// issuer 가 없으면 절반만 설정하지 않는다. endpoint 없는 oidc_auth 는 아무도
// 로그인할 수 없는 Harbor 를 만든다 — db_auth 로 두는 편이 낫다.
func TestHarborProvision_NoIssuerLeavesAuthModeAlone(t *testing.T) {
	manifest := harborOrchestrator(t, "", true).harborProvisionManifest("ssolive")
	assert.NotContains(t, manifest, "oidc_auth")
}

// 스택을 다시 배포하면 이 단계가 또 돈다. 이미 oidc_auth 인 Harbor 에 다시 PUT
// 해도 실패하지 않아야 한다.
func TestHarborProvision_OIDCScriptTolerantOfRerun(t *testing.T) {
	manifest := harborOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true).
		harborProvisionManifest("ssolive")

	// 성공 코드를 명시적으로 다루는지 본다(200 계열을 성공으로).
	assert.True(t, strings.Contains(manifest, "200"),
		"재실행 시 Harbor 응답 코드를 다루지 않으면 설치가 깨진다")
}
