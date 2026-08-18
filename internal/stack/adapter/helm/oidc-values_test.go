package helm

import (
	"context"
	"strings"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type stubSSOProvisioner struct{ slug string }

func (s stubSSOProvisioner) ClientIDFor(step string) (string, bool) {
	name, ok := map[string]string{
		"installing_grafana": "grafana",
		"installing_argocd":  "argocd",
		"installing_minio":   "minio",
	}[step]
	if !ok {
		return "", false
	}
	return s.slug + "-" + name, true
}

func (s stubSSOProvisioner) ToolSteps() []string {
	return []string{"installing_grafana", "installing_argocd", "installing_minio"}
}

func (s stubSSOProvisioner) Provision(context.Context, port.SSOClientSpec) error { return nil }

func (s stubSSOProvisioner) Deprovision(context.Context, string) error { return nil }

func orchestratorWithSSO(t *testing.T, issuer, accessDomain string) *Orchestrator {
	t.Helper()
	o := NewOrchestrator(nil, nil, "ns", WithToolOIDCIssuer(issuer))
	o.ssoFactory = func(string, string) port.SSOProvisioner { return stubSSOProvisioner{slug: "acme"} }
	o.stackConfig = &domain.StackConfig{AccessDomain: accessDomain}
	return o
}

// issuer 는 설정에서 온다. access_domain 에서 "keycloak.<도메인>" 으로 만들어 내던
// 예전 방식은 플랫폼 Keycloak 이 하나뿐이라는 사실과 어긋났다 — 스택이 둘이면
// 둘 중 하나는 반드시 틀린 주소를 받았다.
func TestOIDCValues_IssuerComesFromConfigNotAccessDomain(t *testing.T) {
	o := orchestratorWithSSO(t, "https://auth.nullus.io/realms/nullus", "team-a.nullus.local")

	values := o.oidcValuesForStep("installing_argocd")
	if values == nil {
		t.Fatal("expected argocd OIDC values")
	}
	cm := values["configs"].(map[string]any)["cm"].(map[string]any)
	oidcConfig := cm["oidc.config"].(string)

	if !strings.Contains(oidcConfig, "issuer: https://auth.nullus.io/realms/nullus") {
		t.Fatalf("expected the configured issuer, got:\n%s", oidcConfig)
	}
	if strings.Contains(oidcConfig, "keycloak.team-a.nullus.local") {
		t.Fatalf("issuer must not be derived from access_domain, got:\n%s", oidcConfig)
	}
}

// 도구 자신의 주소는 계속 access_domain 에서 온다 — 도구는 실제로 거기 있다.
func TestOIDCValues_ToolURLsStillUseAccessDomain(t *testing.T) {
	o := orchestratorWithSSO(t, "https://auth.nullus.io/realms/nullus", "team-a.nullus.local")

	values := o.oidcValuesForStep("installing_argocd")
	cm := values["configs"].(map[string]any)["cm"].(map[string]any)
	if got := cm["url"]; got != "https://argocd.team-a.nullus.local" {
		t.Fatalf("expected the tool URL to follow access_domain, got %v", got)
	}
}

func TestOIDCValues_GrafanaEndpointsDeriveFromIssuer(t *testing.T) {
	o := orchestratorWithSSO(t, "http://localhost:8180/realms/nullus", "nullus.local")

	values := o.oidcValuesForStep("installing_grafana")
	if values == nil {
		t.Fatal("expected grafana OIDC values")
	}
	oauth := values["grafana.ini"].(map[string]any)["auth.generic_oauth"].(map[string]any)
	if got := oauth["auth_url"]; got != "http://localhost:8180/realms/nullus/protocol/openid-connect/auth" {
		t.Fatalf("unexpected auth_url: %v", got)
	}
	if got := oauth["client_id"]; got != "acme-grafana" {
		t.Fatalf("expected the namespaced client id, got %v", got)
	}
}

// issuer 가 없으면 값을 절반만 넣지 않는다. 반쯤 설정된 채로 뜨면 도구가 깨진
// 로그인 화면을 보여 주는데, 아예 없으면 로컬 계정으로라도 들어갈 수 있다.
func TestOIDCValues_NoIssuerYieldsNoValues(t *testing.T) {
	o := orchestratorWithSSO(t, "", "team-a.nullus.local")
	if got := o.oidcValuesForStep("installing_argocd"); got != nil {
		t.Fatalf("expected no OIDC values without an issuer, got %v", got)
	}
}
