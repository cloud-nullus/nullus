package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Gitea 는 OAuth 소스를 Helm values 로 받지 않는다. CLI 로만 등록할 수 있고 CLI 는
// app.ini 로 DB 를 찾으므로, 별도 Job 이 아니라 기동된 파드에 exec 한다.
//
// 소스 이름은 Keycloak 에 등록한 콜백 경로(/user/oauth2/keycloak/callback)의
// "keycloak" 과 같아야 한다 — 갈라지면 Gitea 가 콜백을 자기 소스로 못 찾는다.

func TestGiteaOAuthScript_RegistersSourceWithDiscovery(t *testing.T) {
	script := giteaOAuthScript("nullus-sso-gitea", "http://keycloak.nullus.local:8180/realms/nullus")

	assert.Contains(t, script, "add-oauth")
	assert.Contains(t, script, `"keycloak"`, "소스 이름이 콜백 경로와 같아야 한다")
	assert.Contains(t, script, `"nullus-sso-gitea"`, "스택 네임스페이스가 붙은 client ID")
	assert.Contains(t, script,
		"http://keycloak.nullus.local:8180/realms/nullus/.well-known/openid-configuration")
}

// 비밀값이 인자로 들어가면 파드의 프로세스 목록과 감사 로그에 남는다.
func TestGiteaOAuthScript_ReadsSecretFromStdin(t *testing.T) {
	script := giteaOAuthScript("nullus-sso-gitea", "http://kc/realms/nullus")

	assert.Contains(t, script, "read -r OIDC_SECRET", "비밀값은 표준입력으로 받아야 한다")
	assert.Contains(t, script, `--secret "$OIDC_SECRET"`)
}

// 스택을 다시 배포하면 이 단계가 또 돈다. 이미 있는 소스를 다시 add 하면
// Gitea 가 실패하므로 update 로 물러나야 한다.
func TestGiteaOAuthScript_IsIdempotent(t *testing.T) {
	script := giteaOAuthScript("nullus-sso-gitea", "http://kc/realms/nullus")
	assert.Contains(t, script, "update-oauth",
		"재배포 시 기존 소스를 갱신하지 않으면 설치가 깨진다")
}

// issuer 끝 슬래시가 있어도 discovery URL 이 겹치지 않아야 한다.
func TestGiteaOAuthScript_TrimsTrailingSlash(t *testing.T) {
	script := giteaOAuthScript("c", "http://kc/realms/nullus/")
	assert.NotContains(t, script, "nullus//.well-known")
}

func TestGiteaOIDCSettings_OffWithoutSSOOrIssuer(t *testing.T) {
	_, _, ok := ssoOrchestrator(t, "http://kc/realms/nullus", false).giteaOIDCSettings()
	assert.False(t, ok, "SSO 를 안 쓰면 켜지 않는다")

	_, _, ok = ssoOrchestrator(t, "", true).giteaOIDCSettings()
	assert.False(t, ok, "issuer 가 없으면 켜지 않는다")
}

func TestGiteaOIDCSettings_OnWithBoth(t *testing.T) {
	clientID, issuer, ok := ssoOrchestrator(t, "http://kc/realms/nullus", true).giteaOIDCSettings()
	require.True(t, ok)
	assert.Equal(t, "nullus-sso-gitea", clientID)
	assert.Equal(t, "http://kc/realms/nullus", issuer)
}

var _ = strings.Contains
