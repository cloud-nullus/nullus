package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 게이트웨이는 스택 소유물이 아니다.
//
// 스택마다 자기 Gateway 를 만들면, 스택을 지울 때 밖에서 들어오는 현관이 함께
// 사라진다. Zadara 처럼 LoadBalancer 가 없어 ingress 로 받아 넘겨야 하는 환경에서는
// 그 배선을 스택마다 다시 해야 한다는 뜻이다. 이름과 자리를 고정한다.
func sharedGatewayManifest(t *testing.T, stackNamespace string) string {
	t.Helper()
	cfg := domain.StackConfig{AccessDomain: "nullus.io"}
	cfg.Pipeline.CDTool = domain.ToolSelection{Name: "argocd", Enabled: true}

	o := NewOrchestrator(&mockInstaller{}, []byte("kubeconfig"), stackNamespace)
	o.SetStackConfig(cfg)
	return o.defaultGatewayBundleManifest(stackNamespace)
}

func TestSharedGateway_LivesInFixedNamespaceAndName(t *testing.T) {
	manifest := sharedGatewayManifest(t, "nullus-demo")

	require.Contains(t, manifest, "kind: Gateway")
	assert.Contains(t, manifest, "name: "+domain.SharedGatewayName)
	assert.Contains(t, manifest, "namespace: "+domain.SharedGatewayNamespace)
	// 스택 이름에서 만든 옛 이름은 더 이상 나오지 않는다.
	assert.NotContains(t, manifest, "nullus-io-gateway")
}

func TestSharedGateway_CreatesItsNamespace(t *testing.T) {
	manifest := sharedGatewayManifest(t, "nullus-demo")

	assert.Contains(t, manifest, "kind: Namespace")
	assert.Contains(t, manifest, "name: "+domain.SharedGatewayNamespace)
}

// 공용 리소스에 스택 라벨이 붙으면 그 스택을 지울 때 라벨 청소가 함께 지운다 —
// 다른 스택의 현관까지 사라진다.
func TestSharedGateway_CarriesNoStackLabel(t *testing.T) {
	manifest := sharedGatewayManifest(t, "nullus-demo")

	gatewayDoc := ""
	for _, doc := range strings.Split(manifest, "\n---\n") {
		if strings.Contains(doc, "kind: Gateway") || strings.Contains(doc, "kind: Certificate") {
			gatewayDoc += doc
		}
	}
	require.NotEmpty(t, gatewayDoc)
	assert.NotContains(t, gatewayDoc, "nullus.io/stack-name")
}

// 리스너가 한 도메인에 묶이면 다른 접속 도메인을 쓰는 스택이 붙지 못한다.
// 로컬(.internal)과 운영(nullus.io)이 같은 게이트웨이를 쓴다.
func TestSharedGateway_ListenersAcceptAnyHostname(t *testing.T) {
	manifest := sharedGatewayManifest(t, "nullus-demo")

	assert.NotContains(t, manifest, `hostname: "*.nullus.io"`)
	assert.Contains(t, manifest, "protocol: HTTP\n")
	assert.Contains(t, manifest, "protocol: HTTPS")
}

// 라우트는 스택 것이다 — 스택 네임스페이스에 남고, 스택을 지우면 함께 사라진다.
func TestSharedGateway_RoutesStayInStackNamespaceAndAttachAcrossNamespaces(t *testing.T) {
	manifest := sharedGatewayManifest(t, "nullus-demo")

	routeDoc := ""
	for _, doc := range strings.Split(manifest, "\n---\n") {
		if strings.Contains(doc, "kind: HTTPRoute") {
			routeDoc = doc
			break
		}
	}
	require.NotEmpty(t, routeDoc, "HTTPRoute 가 없다")

	assert.Contains(t, routeDoc, "namespace: nullus-demo")
	assert.Contains(t, routeDoc, "nullus.io/stack-name")
	assert.Contains(t, routeDoc, "- name: "+domain.SharedGatewayName)
	assert.Contains(t, routeDoc, "namespace: "+domain.SharedGatewayNamespace,
		"다른 네임스페이스의 게이트웨이에 붙으려면 parentRefs 에 namespace 가 있어야 한다")
}
