package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 도구 URL 과 OIDC redirect URI 는 코드 6곳에서 https:// 로 만들어진다
// (sso_provisioner.buildRedirectURI, oidc-values 의 grafana/argocd/minio,
// stack_handler 의 접속 주소). 그런데 게이트웨이는 HTTP:80 리스너만 열고 있었다.
//
// 그래서 Keycloak 인증이 끝난 뒤 브라우저가 https://argocd.<도메인>/auth/callback
// 으로 보내지는데 443 에 아무것도 없어 연결이 끊겼다. SSO 를 붙여도 로그인이
// 완료될 수 없는 상태였다.

func gatewayManifestWithTLS(t *testing.T, tls *domain.AccessDomainTLSConfig) string {
	t.Helper()
	cfg := domain.StackConfig{AccessDomain: "nullus.local", AccessDomainTLS: tls}
	cfg.Pipeline.CDTool = domain.ToolSelection{Name: "argocd", Enabled: true}

	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus")
	o.SetStackConfig(cfg)
	return o.defaultGatewayBundleManifest("nullus")
}

func TestGatewayTLS_HasHTTPSListenerForAccessDomain(t *testing.T) {
	manifest := gatewayManifestWithTLS(t, nil)

	require.Contains(t, manifest, "protocol: HTTPS")
	assert.Contains(t, manifest, "port: 443")
	assert.Contains(t, manifest, `hostname: "*.nullus.local"`)
	assert.Contains(t, manifest, "certificateRefs:")
}

// HTTP 리스너는 남는다. 게이트웨이 상태 확인이나 TLS 를 요구하지 않는 접근 경로가
// 여기에 의존한다(예: 에어갭 데모의 port-forward).
func TestGatewayTLS_KeepsHTTPListener(t *testing.T) {
	manifest := gatewayManifestWithTLS(t, nil)
	assert.Contains(t, manifest, "protocol: HTTP\n")
	assert.Contains(t, manifest, "port: 80")
}

// 인증서를 따로 주지 않으면 설치 파이프라인이 이미 만드는 내부 CA
// (nullus-internal-ca-issuer)로 와일드카드 인증서를 발급한다.
func TestGatewayTLS_IssuesWildcardCertFromInternalCA(t *testing.T) {
	manifest := gatewayManifestWithTLS(t, nil)

	require.Contains(t, manifest, "kind: Certificate")
	assert.Contains(t, manifest, "name: nullus-internal-ca-issuer")
	assert.Contains(t, manifest, "- \"*.nullus.local\"")
}

// 사내 인증서를 쓰는 환경(access_domain_tls.enabled)은 그 시크릿을 그대로 쓴다.
// 우리가 Certificate 를 또 만들면 cert-manager 가 사용자 인증서를 덮어쓴다.
func TestGatewayTLS_UsesProvidedSecretWithoutIssuingCert(t *testing.T) {
	manifest := gatewayManifestWithTLS(t, &domain.AccessDomainTLSConfig{
		Enabled:         true,
		SecretName:      "corp-wildcard-tls",
		SecretNamespace: "nullus",
		IssuerName:      "corp-offline-ca",
	})

	assert.Contains(t, manifest, "corp-wildcard-tls")
	assert.NotContains(t, manifest, "kind: Certificate",
		"사용자가 준 인증서가 있으면 Certificate 를 만들지 않아야 한다")
}

func TestGatewayTLS_NoAccessDomainStillYieldsNothing(t *testing.T) {
	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), "nullus")
	o.SetStackConfig(domain.StackConfig{})
	assert.Equal(t, "", strings.TrimSpace(o.defaultGatewayBundleManifest("nullus")))
}
