package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 기본 차트는 파드가 뜰 때마다 updates.jenkins.io 에서 플러그인을 받는다.
// SSO 용 oic-auth 를 더하자 의존성 해석에 대용량 메타데이터가 필요해졌고,
// 준비 검사 600초를 넘겨 설치가 통째로 실패했다:
//
//   runtime readiness failed for jenkins: timed out waiting for the condition
//   (init 컨테이너가 11분째 "Cache miss for: plugin-versions" 에 머물렀다)
//
// 에어갭에서는 애초에 나갈 수 없다. 플러그인은 이미지에 굽고 런타임 설치는 끈다.

func jenkinsValues(t *testing.T) map[string]any {
	t.Helper()
	values := DefaultValues("installing_jenkins")
	require.NotNil(t, values)
	controller, ok := values["controller"].(map[string]any)
	require.True(t, ok, "controller 블록이 없다")
	return controller
}

func TestJenkinsValues_UsesPrebuiltImage(t *testing.T) {
	controller := jenkinsValues(t)

	image, ok := controller["image"].(map[string]any)
	require.True(t, ok, "이미지를 지정하지 않으면 기본 이미지가 뜨고 플러그인이 없다")
	assert.Equal(t, "ghcr.io/cloud-nullus/nullus-jenkins", image["repository"])
	assert.NotEmpty(t, image["tag"])
}

// 런타임 설치가 켜져 있으면 이미지에 구워 둔 의미가 없다 — 같은 다운로드를
// 다시 하고 같은 실패가 돌아온다.
func TestJenkinsValues_DisablesRuntimePluginInstall(t *testing.T) {
	controller := jenkinsValues(t)
	assert.Equal(t, false, controller["installPlugins"],
		"installPlugins 를 끄지 않으면 init 컨테이너가 다시 플러그인을 받는다")
}

// 이미지 태그와 Jenkins 버전이 갈라지면 JCasC 스키마가 안 맞을 수 있다.
func TestJenkinsValues_ImageTagMatchesChartAppVersion(t *testing.T) {
	controller := jenkinsValues(t)
	image := controller["image"].(map[string]any)
	assert.Equal(t, jenkinsImageTag, image["tag"])
}
