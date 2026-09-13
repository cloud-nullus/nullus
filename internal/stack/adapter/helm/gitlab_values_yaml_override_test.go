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
