package helm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// SonarQube Community 는 SSO 로 SAML 만 받는다. OIDC 도구와 세 가지가 다르다.
//   - client secret 이 없다. 시크릿 평면이 만들지 않고 프로비저닝이 읽지 않는다.
//   - 디스커버리 문서가 없어 IdP 의 서명 인증서를 values 로 직접 받는다.
//   - 로그인 설정은 sonar.properties(sonarProperties)로 들어간다.

const samlTestIssuer = "https://auth.nullus.io/realms/nullus"

type samlStubProvisioner struct {
	cert      string
	certErr   error
	certCalls *int
	// provisioned 는 Provision 이 받은 client secret 을 도구별로 담는다.
	provisioned map[string]string
}

func (s samlStubProvisioner) ClientIDFor(step string) (string, bool) {
	name, ok := map[string]string{
		"installing_grafana":   "grafana",
		"installing_sonarqube": "sonarqube",
	}[step]
	if !ok {
		return "", false
	}
	return "acme-" + name, true
}

func (s samlStubProvisioner) ToolSteps() []string {
	return []string{"installing_grafana", "installing_sonarqube"}
}

func (s samlStubProvisioner) Provision(_ context.Context, spec port.SSOClientSpec) error {
	if s.provisioned != nil {
		s.provisioned[spec.StepName] = spec.ClientSecret
	}
	return nil
}

func (s samlStubProvisioner) Deprovision(context.Context, string) error { return nil }

func (s samlStubProvisioner) UsesClientSecret(step string) bool {
	return step != "installing_sonarqube"
}

func (s samlStubProvisioner) SAMLSigningCertificate(context.Context) (string, error) {
	if s.certCalls != nil {
		*s.certCalls++
	}
	return s.cert, s.certErr
}

func orchestratorWithSAML(t *testing.T, stub samlStubProvisioner) *Orchestrator {
	t.Helper()
	o := NewOrchestrator(nil, nil, "nullus-demo", WithToolOIDCIssuer(samlTestIssuer))
	o.ssoFactory = func(string, string) port.SSOProvisioner { return stub }
	o.stackConfig = &domain.StackConfig{
		AccessDomain: "nullus.local",
		Monitoring:   domain.MonitoringConfig{Visualization: domain.ToolSelection{Name: "Grafana", Enabled: true}},
		Security:     domain.SecurityConfig{SAST: domain.ToolSelection{Name: "SonarQube", Enabled: true}},
	}
	return o
}

// SAML 도구의 client secret 을 만들면 아무도 읽지 않는 값이 OpenBao 와 Secret 으로 남는다.
func TestSSOManagedSecrets_SkipsSAMLTools(t *testing.T) {
	o := orchestratorWithSAML(t, samlStubProvisioner{})

	var consumers []string
	for _, item := range o.ssoManagedSecrets() {
		consumers = append(consumers, item.Consumer)
	}
	assert.Contains(t, consumers, "installing_grafana", "OIDC 도구의 client secret 은 그대로 만든다")
	assert.NotContains(t, consumers, "installing_sonarqube", "SAML 도구는 client secret 이 없다")
}

// 프로비저닝은 OpenBao 에서 client secret 을 읽어 IdP 에 넣는다. SAML 도구에서
// 그 값을 찾으면 없는 경로를 읽다가 SSO 단계 전체가 멈춘다.
func TestProvisionSSOClients_SAMLToolNeedsNoClientSecret(t *testing.T) {
	stub := samlStubProvisioner{provisioned: map[string]string{}}
	o := orchestratorWithSAML(t, stub)

	var readPaths []string
	read := func(_ context.Context, path string) (string, error) {
		readPaths = append(readPaths, path)
		return "grafana-secret", nil
	}
	require.NoError(t, o.provisionSSOClients(context.Background(), stub, "kv/nullus/dev/org/", read))

	assert.Equal(t, []string{"kv/nullus/dev/org/" + ssoClientSecretPath("acme-grafana")}, readPaths,
		"SAML 도구의 client secret 은 읽지 않는다")
	assert.Equal(t, "grafana-secret", stub.provisioned["installing_grafana"])
	secret, registered := stub.provisioned["installing_sonarqube"]
	assert.True(t, registered, "SAML 클라이언트도 등록해야 SonarQube 가 Keycloak 으로 로그인한다")
	assert.Empty(t, secret)
}

// SonarQube 는 받은 인증서로 응답 서명을 검증하고, 속성 이름으로 사용자를 읽는다.
func TestSonarQubeSAMLValues_PointAtKeycloak(t *testing.T) {
	o := orchestratorWithSAML(t, samlStubProvisioner{cert: "MIIC-REALM-CERT"})
	require.NoError(t, o.loadSAMLSigningCertificate(context.Background()))

	values := o.oidcValuesForStep("installing_sonarqube")
	require.NotNil(t, values)
	props, _ := values["sonarProperties"].(map[string]any)

	assert.Equal(t, "true", props["sonar.auth.saml.enabled"])
	assert.Equal(t, "acme-sonarqube", props["sonar.auth.saml.applicationId"],
		"Keycloak 에 등록한 clientId(SP entity ID)와 같아야 한다")
	assert.Equal(t, samlTestIssuer, props["sonar.auth.saml.providerId"],
		"Keycloak 의 IdP entity ID 는 realm 주소(OIDC issuer)다")
	assert.Equal(t, samlTestIssuer+"/protocol/saml", props["sonar.auth.saml.loginUrl"])
	assert.Equal(t, "MIIC-REALM-CERT", props["sonar.auth.saml.certificate.secured"])
	assert.Equal(t, port.SAMLLoginAttribute, props["sonar.auth.saml.user.login"])
	assert.Equal(t, port.SAMLNameAttribute, props["sonar.auth.saml.user.name"])
	assert.Equal(t, port.SAMLEmailAttribute, props["sonar.auth.saml.user.email"])
}

// 인증서 없이 SAML 을 켜면 로그인 버튼은 있는데 누를 때마다 서명 검증이 실패한다.
// 그럴 바에는 켜지 않는다 — 관리자 계정으로라도 들어갈 수 있다.
func TestSonarQubeSAMLValues_NoCertificateNoSAML(t *testing.T) {
	o := orchestratorWithSAML(t, samlStubProvisioner{})
	assert.Nil(t, o.oidcValuesForStep("installing_sonarqube"))
}

// SAML 설정은 서버 주소(sonar.core.serverBaseURL)와 같은 sonarProperties 에 들어간다.
// 한쪽이 다른 쪽을 덮으면 ACS 주소가 사라지거나 로그인 설정이 사라진다.
func TestSonarQubeValues_SAMLKeepsServerBaseURL(t *testing.T) {
	o := orchestratorWithSAML(t, samlStubProvisioner{cert: "MIIC-REALM-CERT"})
	require.NoError(t, o.loadSAMLSigningCertificate(context.Background()))

	spec, ok := DefaultChartSpecForStep("installing_sonarqube")
	require.True(t, ok)
	props, _ := o.valuesForStep("installing_sonarqube", spec)["sonarProperties"].(map[string]any)

	assert.Equal(t, "https://sonarqube.nullus.local", props["sonar.core.serverBaseURL"],
		"SSO 를 쓰면 ACS 가 Keycloak 에 등록한 https 주소와 같아야 한다")
	assert.Equal(t, "true", props["sonar.auth.saml.enabled"])
}

// Keycloak 이 응답하지 않으면 설치를 멈춘다. SAML 없이 깔고 넘어가면 그 SonarQube 는
// 다시 깔 때까지 로컬 계정으로만 뜬다 — 초록불 뒤의 조용한 누락이다.
func TestLoadSAMLSigningCertificate_FailsWhenKeycloakFails(t *testing.T) {
	o := orchestratorWithSAML(t, samlStubProvisioner{certErr: errors.New("connection refused")})
	err := o.loadSAMLSigningCertificate(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

// SSO 를 쓰지 않는 스택은 Keycloak 에 묻지 않는다.
func TestLoadSAMLSigningCertificate_SkipsWithoutSSO(t *testing.T) {
	calls := 0
	o := orchestratorWithSAML(t, samlStubProvisioner{certCalls: &calls, certErr: errors.New("unreachable")})
	o.toolOIDCIssuer = ""

	require.NoError(t, o.loadSAMLSigningCertificate(context.Background()))
	assert.Zero(t, calls)
	assert.Nil(t, o.oidcValuesForStep("installing_sonarqube"))
}

// 배포된 values 를 편집하면 그 스냅샷이 오버라이드로 저장되고, 병합의 맨 마지막에
// 적용된다. 그 안에 옛 인증서가 얼어 있으면 렐름 키를 회전한 뒤 단계를 다시 돌려도
// 옛 인증서가 이겨 모든 SAML 로그인이 서명 검증에서 실패한다. 플랫폼이 계산하는
// 값은 오버라이드 위에 다시 못박는다 — 사용자가 더한 속성은 그대로 둔다.
func TestSonarQubeValues_FrozenOverrideCannotPinOldSAML(t *testing.T) {
	o := orchestratorWithSAML(t, samlStubProvisioner{cert: "NEW-CERT"})
	o.stackConfig.YAMLOverrides = map[string]string{"installing_sonarqube": `
sonarProperties:
  sonar.core.serverBaseURL: http://sonarqube.old.example
  sonar.auth.saml.certificate.secured: OLD-CERT
  sonar.auth.saml.providerId: https://old-idp.example/realms/nullus
  sonar.web.javaAdditionalOpts: -Xmx2G
`}
	require.NoError(t, o.loadSAMLSigningCertificate(context.Background()))

	spec, ok := DefaultChartSpecForStep("installing_sonarqube")
	require.True(t, ok)
	props, _ := o.valuesForStep("installing_sonarqube", spec)["sonarProperties"].(map[string]any)

	assert.Equal(t, "NEW-CERT", props["sonar.auth.saml.certificate.secured"])
	assert.Equal(t, samlTestIssuer, props["sonar.auth.saml.providerId"])
	assert.Equal(t, "https://sonarqube.nullus.local", props["sonar.core.serverBaseURL"],
		"ACS 가 Keycloak 에 등록한 주소에서 벗어나면 응답을 받지 못한다")
	assert.Equal(t, "-Xmx2G", props["sonar.web.javaAdditionalOpts"], "사용자가 더한 속성은 그대로 둔다")
}

// 설치 경로가 values 를 만들기 전에 인증서를 읽어야 한다. 그 호출이 빠지거나 뒤로
// 밀리면 SonarQube 는 아무 오류 없이 SAML 없이 깔린다 — 인증서를 직접 읽게 하는
// 단위 테스트로는 그 배선을 놓친다.
func TestExecuteStep_SonarQubeInstallsWithSAML(t *testing.T) {
	fakeKubectlOnPath(t)
	installer := &mockInstaller{}
	calls := 0
	o := NewOrchestrator(installer, []byte(`apiVersion: v1
kind: Config
clusters:
- name: kind
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: kind
  context: {cluster: kind, user: kind}
current-context: kind
users:
- name: kind
  user: {token: t}
`), "nullus-demo", WithToolOIDCIssuer(samlTestIssuer))
	o.ssoFactory = func(string, string) port.SSOProvisioner {
		return samlStubProvisioner{cert: "REALM-CERT", certCalls: &calls}
	}
	o.stackConfig = &domain.StackConfig{
		AccessDomain: "nullus.local",
		Security:     domain.SecurityConfig{SAST: domain.ToolSelection{Name: "SonarQube", Enabled: true}},
	}

	require.NoError(t, o.ExecuteStep(context.Background(), "", "installing_sonarqube", ""))

	assert.Equal(t, 1, calls, "설치 직전에 Keycloak 에서 인증서를 읽어야 한다")
	props, _ := installer.valuesByRelease[domain.SonarQubeReleaseName]["sonarProperties"].(map[string]any)
	assert.Equal(t, "REALM-CERT", props["sonar.auth.saml.certificate.secured"])
	assert.Equal(t, "true", props["sonar.auth.saml.enabled"])
}
