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

// 브리지를 지나는 것은 웹 요청만이 아니다. 이미지 push 와 git push 가 같은 길로
// 간다 — ingress-nginx 기본 본문 상한은 1m 이라, 수십 MB 짜리 layer 업로드가
// 통째로 막힌다.
//
// 2026-08-21 운영에서 그랬다. docker login 과 build 는 지나가고 push 만 끝없이
// 재시도했다. 작은 요청은 통과하고 큰 본문만 막히는 것이 이 증상의 특징이다.
func TestGatewayBridgeIngress_AllowsLargeUploads(t *testing.T) {
	manifest := gatewayBridgeIngressManifest(
		"nullus-devsecops-stack", "nullus-devsecops-stack", "nullus.io", "envoy-gateway-svc")

	// 0 은 무제한이다. 고정 상한을 두면 그보다 큰 layer 에서 다시 막힌다.
	assert.Contains(t, manifest, "nginx.ingress.kubernetes.io/proxy-body-size: \"0\"")
	// 본문을 디스크에 모았다가 넘기면 큰 업로드에서 버퍼가 터지고 지연도 커진다.
	assert.Contains(t, manifest, "nginx.ingress.kubernetes.io/proxy-request-buffering: \"off\"")
	// 기본 60초로는 느린 회선의 큰 layer 가 중간에 끊긴다.
	assert.Contains(t, manifest, "nginx.ingress.kubernetes.io/proxy-read-timeout:")
	assert.Contains(t, manifest, "nginx.ingress.kubernetes.io/proxy-send-timeout:")
}
