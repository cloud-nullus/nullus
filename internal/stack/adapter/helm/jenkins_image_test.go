package helm

import (
	"os"
	"path/filepath"
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
	assert.NotEmpty(t, image["tag"])
	assert.Equal(t, "IfNotPresent", image["pullPolicy"],
		"고정 태그인데 Always 면 노드에 적재해 둔 이미지를 두고 레지스트리를 친다")
}

// 차트는 registry 와 repository 를 이어 붙인다. repository 에 레지스트리를 또
// 넣으면 ghcr.io/ghcr.io/... 가 되어 파드가 ImagePullBackOff 로 멈춘다(실측).
//
// 빌드 스크립트가 만드는 이름과 차트가 조립하는 이름이 갈라지면, 이미지는
// 만들어졌는데 파드는 못 받는 상태가 된다. 두 출처를 여기서 묶어 둔다.
// CI 는 ghcr.io/<repo>/<이미지> 로 push 한다(cd.yml 의 nullus-api / nullus-web).
// 다른 이름을 쓰면 게시된 이미지를 아무도 못 받는다.
func TestJenkinsImage_FollowsRegistryConvention(t *testing.T) {
	controller := jenkinsValues(t)
	image := controller["image"].(map[string]any)
	assert.Equal(t, "ghcr.io", image["registry"])
	assert.Equal(t, "cloud-nullus/draft/nullus-jenkins", image["repository"],
		"CI 가 push 하는 경로와 같아야 한다")
}

// 에어갭 번들에 없으면 인터넷이 없는 설치에서 Jenkins 가 뜨지 않는다.
// images.txt 는 자동 생성물이라 생성기 쪽에 있어야 한다(직접 고치면 드리프트
// 검사가 막는다).
func TestJenkinsImage_RegisteredForAirgapBundle(t *testing.T) {
	gen, err := os.ReadFile(filepath.Join("..", "..", "..", "..",
		"airgap", "scripts", "00-generate-images.sh"))
	require.NoError(t, err)

	controller := jenkinsValues(t)
	image := controller["image"].(map[string]any)
	ref := image["registry"].(string) + "/" + image["repository"].(string) + ":" + image["tag"].(string)

	assert.Containsf(t, string(gen), ref,
		"에어갭 이미지 생성기에 %s 가 없다 — 인터넷 없는 설치에서 Jenkins 가 못 뜬다", ref)
}

func TestJenkinsImage_ReferenceMatchesBuildScript(t *testing.T) {
	controller := jenkinsValues(t)
	image := controller["image"].(map[string]any)
	assembled := image["registry"].(string) + "/" + image["repository"].(string)

	script, err := os.ReadFile(filepath.Join("..", "..", "..", "..",
		"scripts", "build-jenkins-image.sh"))
	require.NoError(t, err)

	assert.Containsf(t, string(script), "IMAGE:-"+assembled,
		"차트가 조립하는 이름(%s)이 빌드 스크립트의 기본 이미지와 다르다", assembled)
	assert.Containsf(t, string(script), "TAG:-"+image["tag"].(string),
		"태그가 빌드 스크립트와 다르다")
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
