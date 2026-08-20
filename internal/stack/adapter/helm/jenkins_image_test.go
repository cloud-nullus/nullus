package helm

import (
	"os"
	"path/filepath"
	"regexp"
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
// CI 는 ghcr.io/<github.repository>/<이미지> 로 push 한다. 그 경로에는 저장소
// 이름이 들어가므로 리네임되면 함께 바뀐다 — cloud-nullus/draft →
// cloud-nullus/nullus 리네임 때 nullus-api·nullus-web 만 고쳐지고 Jenkins 만 옛
// 경로에 남았다. 스택은 받을 수 없는 이미지를 가리켰고 jenkins-0 이
// Init:ImagePullBackOff 로 멈췄다(2026-08-20 운영에서 실측).
//
// 그래서 여기에 경로를 다시 적지 않는다. 같은 CI 가 올리는 nullus-api 의 경로를
// 차트 기본값에서 끌어와 접두사를 맞춘다 — 플랫폼이 실제로 그 이미지로 떠 있으니
// 그 값이 옳다는 것은 증명돼 있다. 다음 리네임에서도 한쪽만 고치면 여기서 걸린다.
func TestJenkinsImage_SharesGHCRPathWithPlatformImages(t *testing.T) {
	chartValues, err := os.ReadFile(filepath.Join("..", "..", "..", "..",
		"deploy", "helm", "nullus", "values.yaml"))
	require.NoError(t, err)

	match := regexp.MustCompile(`repository:\s*(ghcr\.io/[^\s]+)/nullus-api`).FindSubmatch(chartValues)
	require.Len(t, match, 2, "차트에서 nullus-api 이미지 경로를 찾지 못했다")
	wantPrefix := string(match[1])

	controller := jenkinsValues(t)
	image := controller["image"].(map[string]any)
	assembled := image["registry"].(string) + "/" + image["repository"].(string)

	assert.Equal(t, wantPrefix+"/nullus-jenkins", assembled,
		"CI 가 push 하는 경로(%s/...)와 같아야 한다", wantPrefix)
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
