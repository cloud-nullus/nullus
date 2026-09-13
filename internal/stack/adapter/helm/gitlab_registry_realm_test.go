package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 게이트웨이는 설정과 무관하게 HTTPS 리스너를 연다. GitLab 이 http 로 설정되면
// 레지스트리 인증 realm 이 http:// 로 나가고, https 로 접속한 엄격한 클라이언트
// (Trivy·crane 등 go-containerregistry 기반)는 "realm scheme http not allowed for a
// secure registry" 로 거부한다 — kind 스택에서 이미지 스캔 잡이 매번 실패했다.
//
// realm 만 https 로 둔다. GitLab 외부 주소 전체를 https 로 바꾸면 러너 clone 과
// deploy 잡의 push 가 내부 CA 신뢰 문제에 걸린다.
func TestGitLabRegistryRealm_IsAlwaysHTTPS(t *testing.T) {
	for _, sso := range []bool{false, true} {
		o := gitlabOrchestrator(t, "http://kc/realms/nullus", sso)
		values := o.gitlabSharedServiceValues()

		registry, ok := values["registry"].(map[string]any)
		require.True(t, ok, "registry 블록이 없다 (sso=%v)", sso)
		assert.Equal(t, "https://gitlab.nullus.local", registry["authEndpoint"],
			"차트가 /jwt/auth 를 붙인다 — 스킴과 호스트만 준다 (sso=%v)", sso)
	}
}
