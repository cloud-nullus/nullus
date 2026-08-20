// 스택 도구로 들어오는 요청을 공용 게이트웨이까지 데려다주는 배선을 검사한다.
//
// LoadBalancer 가 없는 환경에서는 게이트웨이가 외부 IP 를 받지 못한다. 그래서
// gitlab.<도메인> 요청은 밖에서 유일하게 열려 있는 ingress-nginx 에 도착하는데,
// 거기에 규칙이 없으면 404 로 끝난다 — 스택은 정상인데 아무도 못 들어가는 상태다.
package helm_test

import (
	"strings"
	"testing"
)

const bridgeHost = "*.nullus.io"

func bridgeArgs(extra ...string) []string {
	return append([]string{
		"--set", "gatewayBridge.enabled=true",
		"--set", "gatewayBridge.hosts[0]=" + bridgeHost,
		"--set", "ingress.className=nginx",
	}, extra...)
}

func TestGatewayBridge_DisabledByDefault(t *testing.T) {
	rendered := renderChart(t)

	if strings.Contains(rendered, "gateway-bridge") {
		t.Fatal("브리지는 기본으로 꺼져 있어야 한다 — 환경마다 필요 여부가 다르다")
	}
}

func TestGatewayBridge_SendsWildcardHostToSharedGateway(t *testing.T) {
	rendered := renderChart(t, bridgeArgs()...)

	for _, want := range []string{
		"nullus-gateway-bridge",
		"host: \"" + bridgeHost + "\"",
		// 다른 네임스페이스의 게이트웨이는 ExternalName 으로 건너간다.
		"externalName: nullus-gateway-proxy.nullus-gateway.svc.cluster.local",
		"name: nullus-gateway-upstream",
		"ingressClassName: nginx",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("브리지 렌더에 %q 가 없다", want)
		}
	}
}

// 게이트웨이의 HTTPRoute 는 Host 로 도구를 갈라낸다. ingress 가 호스트를 바꿔
// 보내면 어느 라우트에도 걸리지 않아 404 가 그대로 남는다.
func TestGatewayBridge_PreservesOriginalHost(t *testing.T) {
	rendered := renderChart(t, bridgeArgs()...)

	if !strings.Contains(rendered, "nginx.ingress.kubernetes.io/upstream-vhost: $host") {
		t.Fatal("원래 Host 를 보존하는 어노테이션이 없다")
	}
}

func TestGatewayBridge_TerminatesTLSWhenSecretGiven(t *testing.T) {
	rendered := renderChart(t, bridgeArgs("--set", "gatewayBridge.tlsSecretName=nullus-wildcard-tls")...)

	if !strings.Contains(rendered, "secretName: nullus-wildcard-tls") {
		t.Fatal("TLS 시크릿을 주면 ingress 가 종단해야 한다")
	}
}

// 운영 값은 브리지를 켜 둔다. 끄면 스택 도구가 밖에서 열리지 않는다.
func TestGatewayBridge_EnabledInZadaraValues(t *testing.T) {
	rendered := renderChart(t, "-f", "../csp/zadara/values-zadara.yaml")

	if !strings.Contains(rendered, "nullus-gateway-bridge") {
		t.Fatal("values-zadara 에서 브리지가 켜져 있어야 한다")
	}
	if !strings.Contains(rendered, "host: \""+bridgeHost+"\"") {
		t.Fatalf("values-zadara 의 브리지가 %s 를 받지 않는다", bridgeHost)
	}
}
