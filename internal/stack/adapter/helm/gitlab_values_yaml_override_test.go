package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// YAML override 가 하나라도 있으면 valuesForStep 은 다른 분기를 탄다. 그 분기에
// GitLab 공용 값(global.hosts.https, registry.authEndpoint)이 빠져 있어, 다른
// 도구 하나만 손봐도 GitLab 레지스트리 realm 이 http 로 돌아갔다 — 이미지 스캔이
// "realm scheme "http" not allowed for a secure registry" 로 다시 실패한다.
//
// Harbor 이미지만 바꾼 스택(arm64 kind)에서 코드를 따라가다 드러났다.
func TestValuesForStep_GitLabKeepsSharedServiceValuesWithYAMLOverrides(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.SetStackConfig(domain.StackConfig{
		AccessDomain:  "nullus.local",
		YAMLOverrides: map[string]string{"installing_harbor": "core:\n  replicas: 1\n"},
	})

	spec, ok := defaultChartSpecForStep("installing_gitlab")
	require.True(t, ok)

	values := o.valuesForStep("installing_gitlab", spec)

	registry, _ := values["registry"].(map[string]any)
	assert.Equal(t, "https://gitlab.nullus.local", registry["authEndpoint"],
		"다른 도구의 override 가 GitLab realm 을 http 로 되돌리면 안 된다")

	global, _ := values["global"].(map[string]any)
	hosts, _ := global["hosts"].(map[string]any)
	assert.Equal(t, "nullus.local", hosts["domain"])
	assert.Equal(t, false, hosts["https"])
}

// GitLab 의 초기 root 비밀번호는 설치가 금고에 만들어 ESO 로 프로비저닝한 Secret 이 출처여야 한다.
// 차트가 그것을 참조하지 않으면 차트가 자기 Secret(gitlab-gitlab-initial-root-password)에 임의
// 비밀번호를 만들고, 금고의 값·연결 정보 안내·백업은 전부 가짜를 가리킨다 — 실측: 두 Secret 의
// 값이 달랐고 로그인되는 것은 차트 것이었다. 값의 단일 출처는 domain 이다.
func TestValuesForStep_GitLab_초기_root_비밀번호는_프로비저닝된_Secret_을_쓴다(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.SetStackConfig(domain.StackConfig{AccessDomain: "nullus.local"})
	spec, ok := defaultChartSpecForStep("installing_gitlab")
	require.True(t, ok)

	values := o.valuesForStep("installing_gitlab", spec)
	global, _ := values["global"].(map[string]any)
	root, _ := global["initialRootPassword"].(map[string]any)
	require.NotNil(t, root, "global.initialRootPassword 가 없으면 차트가 임의 비밀번호를 만든다")
	assert.Equal(t, ProvisionedGitLabRootSecret, root["secret"])
	assert.Equal(t, domain.GitLabRootPassKey, root["key"])
	assert.Equal(t, domain.GitLabRootSecret, ProvisionedGitLabRootSecret,
		"설치 경로와 연결 정보 안내가 같은 Secret 이름을 봐야 한다")
}
