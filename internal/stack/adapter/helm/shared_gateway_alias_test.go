package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

const envoyServiceListJSON = `{
  "items": [
    {
      "metadata": {"name": "envoy-nullus-gateway-nullus-gateway-3197e0f2"},
      "spec": {
        "selector": {
          "gateway.envoyproxy.io/owning-gateway-name": "nullus-gateway",
          "app.kubernetes.io/component": "proxy"
        },
        "ports": [
          {"name": "http-80", "port": 80, "protocol": "TCP", "targetPort": 10080},
          {"name": "https-443", "port": 443, "protocol": "TCP", "targetPort": 10443}
        ]
      }
    }
  ]
}`

// Envoy Gateway 가 만드는 Service 이름에는 해시가 붙는다. 차트나 ingress 규칙이
// 미리 적을 수 없어 사람이 조회해 옮겨 적어야 했다. 고정 이름 별칭을 둔다.
func TestSharedGatewayProxyAlias_CopiesSelectorAndPorts(t *testing.T) {
	manifest, err := sharedGatewayProxyAliasManifest([]byte(envoyServiceListJSON))
	require.NoError(t, err)

	assert.Contains(t, manifest, "name: "+sharedGatewayProxyServiceName)
	assert.Contains(t, manifest, "namespace: "+domain.SharedGatewayNamespace)
	assert.Contains(t, manifest, "gateway.envoyproxy.io/owning-gateway-name: nullus-gateway")
	assert.Contains(t, manifest, "app.kubernetes.io/component: proxy")

	// 포트 번호를 짐작하지 않고 실제 Service 에서 복사한다 — Envoy Gateway 가
	// 권한 포트를 10000 더해 매핑하는 규칙을 바꿔도 따라간다.
	assert.Contains(t, manifest, "port: 80")
	assert.Contains(t, manifest, "targetPort: 10080")
	assert.Contains(t, manifest, "port: 443")
	assert.Contains(t, manifest, "targetPort: 10443")
	assert.Contains(t, manifest, "nullus.io/alias-of: envoy-nullus-gateway-nullus-gateway-3197e0f2")
}

func TestSharedGatewayProxyAlias_FailsWhenDataPlaneMissing(t *testing.T) {
	_, err := sharedGatewayProxyAliasManifest([]byte(`{"items": []}`))
	require.Error(t, err)
}

func TestSharedGatewayProxyAlias_FailsWithoutSelector(t *testing.T) {
	_, err := sharedGatewayProxyAliasManifest([]byte(`{"items":[{"metadata":{"name":"envoy-x"},"spec":{"ports":[]}}]}`))
	require.Error(t, err)
}
