package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// multibranch job 이 private Gitea 리포를 스캔하려면 credential 객체가 필요하다.
// agent 파드의 env 로는 안 된다 — 스캔은 컨트롤러가 하고 Jenkins 는 실제
// credential 을 요구한다.
func TestJenkinsValues_DeclaresGiteaCredentialViaJCasC(t *testing.T) {
	values := DefaultValues("installing_jenkins")

	controller, ok := values["controller"].(map[string]any)
	require.True(t, ok)

	// 차트는 마운트된 Secret 을 {name}-{keyName} 으로 노출한다. 이 목록에
	// 없으면 JCasC 의 보간이 빈 문자열이 되고 인증이 조용히 실패한다.
	mounted, ok := controller["additionalExistingSecrets"].([]any)
	require.True(t, ok, "Secret 을 마운트하지 않으면 JCasC 가 보간할 값이 없다")
	require.Len(t, mounted, 2)

	jcasc, ok := controller["JCasC"].(map[string]any)
	require.True(t, ok)
	scripts, ok := jcasc["configScripts"].(map[string]any)
	require.True(t, ok)
	script, ok := scripts["nullus-gitea-credentials"].(string)
	require.True(t, ok)

	assert.Contains(t, script, `id: "`+JenkinsGiteaCredentialID+`"`)
	// 보간 표현식이 마운트한 Secret 이름/키와 정확히 맞아야 한다.
	assert.Contains(t, script, "${"+domain.GiteaAdminSecret+"-"+domain.GiteaAdminUserKey+"}")
	assert.Contains(t, script, "${"+domain.GiteaAdminSecret+"-"+domain.GiteaAdminPasswordKey+"}")
}

// 서버를 등록하지 않으면 플러그인이 job 의 SCM 소스가 가리키는 주소를
// 모르는 서버로 보고 스캔을 거부한다.
func TestJenkinsGiteaServerValues_RegistersInClusterAddress(t *testing.T) {
	values := jenkinsGiteaServerValues("nullus-demo")

	controller := values["controller"].(map[string]any)
	jcasc := controller["JCasC"].(map[string]any)
	scripts := jcasc["configScripts"].(map[string]any)
	script := scripts["nullus-gitea-server"].(string)

	assert.Contains(t, script, "http://gitea-http.nullus-demo.svc:3000",
		"주소는 네임스페이스에 달려 있어 DefaultValues 에 넣을 수 없다")
	assert.Contains(t, script, `credentialsId: "`+JenkinsGiteaCredentialID+`"`)
}

func TestJenkinsGiteaServerValues_FallsBackToDefaultNamespace(t *testing.T) {
	values := jenkinsGiteaServerValues("")

	script := values["controller"].(map[string]any)["JCasC"].(map[string]any)["configScripts"].(map[string]any)["nullus-gitea-server"].(string)
	assert.Contains(t, script, defaultStackNamespace)
}

// 모듈 간 직접 import 가 금지돼 stack 과 cicd 가 같은 값을 각자 들고 있다.
// 갈라지면 job 은 만들어지되 브랜치를 하나도 찾지 못한다 — 실패가 설치 시점이
// 아니라 첫 스캔 시점에 나타나 원인을 찾기 어렵다.
//
// cicd 쪽 값은 internal/cicd/usecase 의 giteaCredentialID 다. 문자열을 여기
// 적어 두어 한쪽만 바뀌면 이 테스트가 걸리게 한다.
func TestJenkinsGiteaCredentialID_MatchesCICDContract(t *testing.T) {
	const cicdSideValue = "nullus-gitea"

	assert.Equal(t, cicdSideValue, JenkinsGiteaCredentialID,
		"internal/cicd/usecase 의 giteaCredentialID 와 같아야 한다")
	assert.False(t, strings.Contains(JenkinsGiteaCredentialID, " "),
		"credential id 에 공백이 들어가면 JCasC 참조가 깨진다")
}
