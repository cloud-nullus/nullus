package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LoadBalancer 가 없는 클러스터에서는 Gateway 의 Service 가 외부 IP 를 받지 못한다.
// 밖에서 온 요청은 유일하게 열려 있는 ingress-nginx 에 도착하는데, 거기에 이 호스트
// 규칙이 없으면 404 로 끝난다 — 스택은 정상인데 아무도 못 들어가는 상태다.
func TestGatewayBridgeIngress_SendsWildcardHostToStackGateway(t *testing.T) {
	manifest := gatewayBridgeIngressManifest("nullus-demo", "demo-stack", "nullus.io", "envoy-demo-abc123")

	assert.Contains(t, manifest, "kind: Ingress")
	assert.Contains(t, manifest, "namespace: nullus-demo")
	assert.Contains(t, manifest, `- host: "*.nullus.io"`)
	assert.Contains(t, manifest, "name: envoy-demo-abc123")
	assert.Contains(t, manifest, "ingressClassName: nginx")
}

// Envoy 의 HTTPRoute 가 Host 로 도구를 갈라낸다. ingress 가 호스트를 바꿔 보내면
// 어느 라우트에도 걸리지 않아 404 가 그대로 남는다.
func TestGatewayBridgeIngress_PreservesOriginalHost(t *testing.T) {
	manifest := gatewayBridgeIngressManifest("nullus-demo", "demo-stack", "nullus.io", "envoy-demo-abc123")

	assert.Contains(t, manifest, "nginx.ingress.kubernetes.io/upstream-vhost: $host")
}

// 스택을 지우면 함께 사라져야 한다. 라벨이 없으면 Ingress 가 남는다.
func TestGatewayBridgeIngress_CarriesStackLabel(t *testing.T) {
	manifest := gatewayBridgeIngressManifest("nullus-demo", "demo-stack", "nullus.io", "envoy-demo-abc123")

	assert.Contains(t, manifest, "nullus.io/stack-name: demo-stack")
}

// TLS 는 ingress-nginx 의 기본 인증서가 처리한다(setup-tls.sh 의 ingress-https).
// 여기서 tls 를 선언하면 스택 네임스페이스에 그 시크릿을 복사해 둬야 한다.
func TestGatewayBridgeIngress_LeavesTLSToDefaultCertificate(t *testing.T) {
	manifest := gatewayBridgeIngressManifest("nullus-demo", "demo-stack", "nullus.io", "envoy-demo-abc123")

	assert.NotContains(t, manifest, "secretName:")
	assert.NotContains(t, manifest, "\n  tls:")
}

// 로컬(.internal)은 ingress 컨트롤러가 없고 포트포워드로 붙는다. 규칙을 만들어도
// 아무도 읽지 않으므로 만들지 않는다.
func TestShouldCreateGatewayBridge_SkipsLocalDomains(t *testing.T) {
	assert.False(t, shouldCreateGatewayBridge("nullus-devsecops-stack.internal"))
	assert.False(t, shouldCreateGatewayBridge(""))
	assert.False(t, shouldCreateGatewayBridge("   "))

	require.True(t, shouldCreateGatewayBridge("nullus.io"))
	require.True(t, shouldCreateGatewayBridge("stack.example.com"))
}
