package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SSO 로 로그인은 되는데 화면이 비어 있었다. Argo CD 는 정책이 없는 사용자에게
// 아무것도 보여 주지 않는다 — "No applications available to you just yet".
//
// kubectl 로는 Application 이 Synced/Healthy 로 보였다. 즉 없는 것이 아니라
// 안 보이는 것이었고, 그 차이가 화면 문구에만 있어서(available "to you")
// 없는 것으로 읽혔다.
//
// 로그인을 열어 주었으면 볼 권한도 함께 주어야 한다. 둘을 따로 두면 SSO 를 켠
// 것이 오히려 아무것도 못 보는 상태를 만든다.
func TestArgoCDOIDC_GrantsDefaultPolicyToAuthenticatedUsers(t *testing.T) {
	values := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true).
		oidcValuesForStep("installing_argocd")
	require.NotNil(t, values, "Argo CD OIDC values 가 만들어지지 않았다")

	configs, ok := values["configs"].(map[string]any)
	require.True(t, ok)
	rbac, ok := configs["rbac"].(map[string]any)
	require.True(t, ok, "RBAC 정책이 없으면 로그인해도 아무것도 보이지 않는다")

	// 읽기까지만 연다. 동기화는 CI 가 되커밋한 매니페스트를 보고 자동으로
	// 일어나므로, 보는 데 쓰기 권한이 필요하지 않다.
	assert.Equal(t, "role:readonly", rbac["policy.default"])
}
