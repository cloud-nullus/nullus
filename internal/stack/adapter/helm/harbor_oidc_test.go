package helm

import (
	"context"
	"encoding/json"
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

// 생성된 스크립트의 JSON 이 실제로 파싱되는지 본다.
//
// 문자열만 검사하면 따옴표가 깨져도 통과한다 — 실제로 그렇게 나갔고 Harbor 가
// 422 로 거부했다:
//
//	parsing configurations body ... invalid character 'a' looking for
//	beginning of object key string
//
// 셸이 -d "{"auth_mode":...}" 를 받으면 바깥 따옴표가 첫 안쪽 따옴표에서 끝나
// 따옴표 없는 쓰레기가 전달된다.
func TestHarborProvision_OIDCPayloadIsValidJSON(t *testing.T) {
	script := harborOIDCScript("ssoharbor-harbor", "http://keycloak.nullus.local:8180/realms/nullus")

	// curl 의 -w '%{http_code}' 에도 중괄호가 있으므로 본문 시작을 키로 찾는다.
	start := strings.Index(script, `{"auth_mode"`)
	require.GreaterOrEqual(t, start, 0, "JSON 본문을 찾지 못했다:\n%s", script)
	end := strings.Index(script[start:], "}")
	require.Greater(t, end, 0, "JSON 끝을 찾지 못했다:\n%s", script)

	// 셸의 따옴표 교차와 변수는 런타임에 풀린다 — 파싱을 위해 자리만 채운다.
	payload := strings.ReplaceAll(script[start:start+end+1], `"'"$HARBOR_OIDC_SECRET"'"`, `"dummy-secret"`)

	var parsed map[string]any
	require.NoErrorf(t, json.Unmarshal([]byte(payload), &parsed),
		"Harbor 에 보내는 본문이 JSON 이 아니다:\n%s", payload)

	assert.Equal(t, "oidc_auth", parsed["auth_mode"])
	assert.Equal(t, "ssoharbor-harbor", parsed["oidc_client_id"])
	assert.Equal(t, "http://keycloak.nullus.local:8180/realms/nullus", parsed["oidc_endpoint"])
	assert.Equal(t, "dummy-secret", parsed["oidc_client_secret"])
	assert.Equal(t, true, parsed["oidc_auto_onboard"])
}

// Harbor 는 redirect_uri 를 externalURL 에서 만든다. 그런데 externalURL 은 http
// 였고 Keycloak 에 등록된 redirect 는 https 라 로그인이 막혔다:
//
//	We are sorry... Invalid parameter: redirect_uri
//
// 스킴을 두 곳에서 따로 정한 탓이다. SSO 를 켠 설치에서는 등록된 redirect 와
// 같은 스킴이어야 한다.
func TestHarborExternalURL_MatchesRegisteredRedirectSchemeWhenSSOEnabled(t *testing.T) {
	o := harborOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true)
	cfg := domain.StackConfig{AccessDomain: "nullus.local"}

	values := o.harborExternalURLValues(&cfg)
	assert.Equal(t, "https://harbor.nullus.local", values["externalURL"],
		"등록된 redirect(https://harbor.<도메인>/c/oidc/callback)와 스킴이 같아야 한다")
}

// SSO 를 쓰지 않는 설치는 예전 그대로다. externalURL 은 docker login/push 의 토큰
// realm 이기도 해서, 스킴을 바꾸면 클라이언트가 CA 를 신뢰해야 한다.
func TestHarborExternalURL_StaysHTTPWithoutSSO(t *testing.T) {
	o := harborOrchestrator(t, "", false)
	cfg := domain.StackConfig{AccessDomain: "nullus.local"}

	values := o.harborExternalURLValues(&cfg)
	assert.Equal(t, "http://harbor.nullus.local", values["externalURL"])
}
