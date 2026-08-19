package helm

import (
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"

	"github.com/stretchr/testify/assert"
)

// 도구는 저마다 자기 기본 주소 설정에서 redirect_uri 를 만든다
// (Harbor externalURL, Gitea ROOT_URL). 그 스킴이 Keycloak 에 등록된 redirect 와
// 다르면 로그인이 "Invalid parameter: redirect_uri" 로 막힌다.
//
// 실제로 Harbor 와 Gitea 에서 같은 실패가 났다. 스킴을 도구마다 따로 정하면
// 도구를 추가할 때마다 같은 실수가 반복되므로 한 곳에서 정한다.

func TestToolURLScheme_HTTPSWhenSSOEnabled(t *testing.T) {
	o := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true)
	assert.Equal(t, "https", o.toolURLScheme(),
		"등록된 redirect 가 https 이므로 도구 주소도 https 여야 한다")
}

// SSO 를 쓰지 않으면 http 그대로다. 이 주소들은 클론·이미지 push 의 출처이기도
// 해서, 스킴을 바꾸면 클라이언트가 게이트웨이 인증서의 CA 를 신뢰해야 한다.
func TestToolURLScheme_HTTPWithoutSSO(t *testing.T) {
	assert.Equal(t, "http", ssoOrchestrator(t, "http://kc/realms/nullus", false).toolURLScheme())
	assert.Equal(t, "http", ssoOrchestrator(t, "", true).toolURLScheme())
}

// 도구는 저마다 자기 기본 주소 설정에서 redirect_uri 를 만든다. 셋이 같은 판단을
// 써야 한다 — 갈라지면 그 도구만 로그인이 깨지고, 원인은 인증이 아니라 주소
// 설정에 있어 찾기 어렵다. 실제로 Harbor·Gitea·Jenkins 에서 차례로 같은 실패가 났다:
//
//	We are sorry... Invalid parameter: redirect_uri
func TestToolURLScheme_SharedByAllToolsThatBuildRedirectURI(t *testing.T) {
	o := ssoOrchestrator(t, "http://keycloak.nullus.local:8180/realms/nullus", true)
	scheme := o.toolURLScheme()

	harbor := o.harborExternalURLValues(&domainStackConfigWithAccessDomain)
	assert.Equal(t, scheme+"://harbor.nullus.local", harbor["externalURL"], "Harbor externalURL")

	gitea := o.giteaSharedServiceValues()
	server := gitea["gitea"].(map[string]any)["config"].(map[string]any)["server"].(map[string]any)
	assert.Equal(t, scheme+"://gitea.nullus.local/", server["ROOT_URL"], "Gitea ROOT_URL")

	jenkins := o.jenkinsURLValues()["controller"].(map[string]any)
	assert.Equal(t, scheme+"://jenkins.nullus.local", jenkins["jenkinsUrl"], "Jenkins jenkinsUrl")

	gitlab := o.gitlabSharedServiceValues()["global"].(map[string]any)["hosts"].(map[string]any)
	assert.Equal(t, scheme == "https", gitlab["https"], "GitLab global.hosts.https")
}

var domainStackConfigWithAccessDomain = domain.StackConfig{AccessDomain: "nullus.local"}
