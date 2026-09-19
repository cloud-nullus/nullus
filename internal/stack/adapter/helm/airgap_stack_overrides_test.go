package helm

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// 공식 goharbor 이미지는 어느 버전도 arm64 를 내지 않아 arm64 클러스터에서 Harbor 파드가 뜨지 않는다.
// 온라인에서는 화면에서 yaml_overrides 로 멀티아키 이미지를 넣지만 에어갭 스택 설치(29)는 사람이 없다.
// 번들이 arm64 용 덮어쓰기를 싣고 29 가 그것을 요청에 실어야 하며, 덮어쓴 이미지는 번들 목록에 있어야 한다.
func TestAirgapStackOverrides_Arm64HarborImagesAreBundled(t *testing.T) {
	var values map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(readRepoFile(t, "airgap", "helm", "stack-overrides", "linux-arm64", "installing_harbor.yaml")), &values))

	got := map[string]string{}
	collectImageOverrides("", values, got)
	// installing_harbor 가 띄우는 컴포넌트 — trivy · exporter(metrics)는 설치 values 와 차트 기본값이 끈다.
	for _, path := range []string{
		"nginx.image", "portal.image", "core.image", "jobservice.image",
		"registry.registry.image", "registry.controller.image",
		"database.internal.image", "redis.internal.image",
	} {
		assert.Containsf(t, got, path, "arm64 Harbor 덮어쓰기에 %s 가 없다 — 그 파드는 공식 amd64 이미지로 뜬다", path)
	}

	images := airgapBundleImages(t)
	var missing []string
	for _, image := range got {
		if !images[normalizeBundleImage(image)] {
			missing = append(missing, image)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "arm64 Harbor 덮어쓰기 이미지가 에어갭 번들 목록에 없다")

	assert.True(t, strings.Contains(readRepoFile(t, "airgap", "scripts", "pre", "package-bundle.sh"), "helm/stack-overrides"),
		"번들이 스택 설치 덮어쓰기를 싣지 않는다")
	script := readRepoFile(t, "airgap", "scripts", "29-install-stacks-via-api.sh")
	assert.True(t, strings.Contains(script, "stack-overrides"), "29 가 번들의 스택 설치 덮어쓰기를 찾지 않는다")
	assert.True(t, strings.Contains(script, `"yaml_overrides"`), "29 가 덮어쓰기를 스택 생성 요청에 싣지 않는다")
}

// collectImageOverrides 는 repository · tag 를 함께 가진 image 맵을 "경로 → repository:tag" 로 모은다.
func collectImageOverrides(path string, node any, out map[string]string) {
	m, ok := node.(map[string]any)
	if !ok {
		return
	}
	repo, hasRepo := m["repository"].(string)
	tag, hasTag := m["tag"].(string)
	if hasRepo && hasTag {
		out[path] = repo + ":" + tag
		return
	}
	for k, v := range m {
		child := k
		if path != "" {
			child = path + "." + k
		}
		collectImageOverrides(child, v, out)
	}
}
