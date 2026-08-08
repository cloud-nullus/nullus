package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// root token 은 부트스트랩 성공 뒤 폐기되고 Secret 에서도 제거된다(설계대로).
// 그래서 두 번째 실행부터는 BAO_TOKEN 이 비어 있는 상태로 들어온다.
//
// 이때 "이미 부트스트랩되었는가" 를 확인하는 경로가 인증을 요구하면 확인 자체가
// 불가능해 항상 exit 1 로 떨어진다. 실제로 재배포가 여기서 막혔다.
// 확인은 Kubernetes Auth 로그인으로 해야 한다 — 로그인에 성공한다는 것은
// auth 활성화 + role + policy 가 모두 갖춰졌다는 뜻이기 때문이다.

func TestOpenBaoBootstrapScript_ChecksBootstrapViaKubernetesAuthLogin(t *testing.T) {
	script := openBaoBootstrapScript

	require.Contains(t, script, "auth/kubernetes/login",
		"root token 이 없을 때는 Kubernetes Auth 로그인으로 부트스트랩 여부를 확인해야 한다")
	assert.Contains(t, script, OpenBaoControllerRole,
		"컨트롤러 role 로 로그인해야 한다")
	assert.Contains(t, script, "/var/run/secrets/kubernetes.io/serviceaccount/token",
		"파드에 마운트된 SA 토큰을 JWT 로 써야 한다")
}

func TestOpenBaoBootstrapScript_NoTokenPathDoesNotRelyOnAuthenticatedList(t *testing.T) {
	// 토큰이 없는 분기에서 인증이 필요한 list 를 그대로 호출하면 원래 버그가 재발한다.
	noTokenBlock := extractNoTokenBranch(t, openBaoBootstrapScript)

	assert.NotContains(t, noTokenBlock, "bao list auth/kubernetes/role",
		"토큰 없이 인증이 필요한 list 를 호출하면 항상 실패한다")
}

func TestOpenBaoBootstrapManifest_RunsAsControllerServiceAccount(t *testing.T) {
	manifest := openBaoBootstrapManifest("devsecops", false)

	// Kubernetes Auth 로그인은 role 에 바인딩된 SA 로 떠야만 통과한다.
	// serviceAccountName 이 없으면 default SA 로 떠서 로그인이 거부된다.
	assert.Contains(t, manifest, "serviceAccountName: "+OpenBaoControllerServiceAccount)
}

// wait 의 컨텍스트 타임아웃이 kubectl --timeout 과 같으면 컨텍스트가 먼저(또는
// 동시에) 만료돼 프로세스가 kill 되고, 원인 대신 "signal: killed" 만 남는다.
func TestOpenBaoBootstrapWaitTimeout_HasHeadroomOverKubectlTimeout(t *testing.T) {
	assert.Greater(t, openBaoBootstrapWaitContextTimeout, openBaoBootstrapTimeout,
		"컨텍스트가 kubectl 보다 먼저 끊기면 kubectl 의 실제 오류 메시지를 잃는다")
}

// extractNoTokenBranch 는 "root token 을 쓸 수 없음" 분기 본문을 뽑는다.
func extractNoTokenBranch(t *testing.T, script string) string {
	t.Helper()
	const marker = "root token 을 쓸 수 없음"
	idx := strings.Index(script, marker)
	require.GreaterOrEqual(t, idx, 0, "토큰 부재 분기를 찾지 못했다")

	rest := script[idx:]
	end := strings.Index(rest, "== KV v2 엔진 활성화 ==")
	require.Greater(t, end, 0, "분기 끝을 찾지 못했다")
	return rest[:end]
}
