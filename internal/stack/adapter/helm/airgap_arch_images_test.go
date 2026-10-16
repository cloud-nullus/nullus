package helm

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 공식 goharbor 이미지는 어느 버전도 arm64 를 내지 않아, arm64 클러스터에서는 설치가 노드
// 아키텍처를 읽어 멀티아키 이미지로 바꾼다(arch-images.go). 에어갭은 그 이미지를 인터넷에서
// 받을 수 없으므로 번들 목록에 있어야 한다 — 없으면 arm64 에어갭에서 같은 자리에서 멈춘다.
func TestAirgapBundle_ContainsArchAlternativeImages(t *testing.T) {
	images := airgapBundleImages(t)

	for _, p := range domain.ToolImageProfiles() {
		for _, src := range p.Sources {
			values := archImageValues(archImageComponents[p.Step], src)
			require.NotEmptyf(t, values, "%s 의 대체 이미지 values 가 비었다", p.Tool)

			got := map[string]string{}
			collectImageOverrides("", values, got)
			var missing []string
			for _, image := range got {
				if !images[normalizeBundleImage(image)] {
					missing = append(missing, image)
				}
			}
			sort.Strings(missing)
			assert.Emptyf(t, missing, "%s 대체 이미지가 에어갭 번들 목록에 없다", p.Tool)
		}
	}
}

// 29 는 예전에 arm64 Harbor 이미지를 yaml_overrides 로 따로 실었다. 설치 백엔드가 같은 일을
// 하므로 그 경로를 다시 두면 두 출처가 갈라져, 태그를 올릴 때 한쪽만 낡는다.
func TestAirgapStackInstall_DoesNotShipArchOverrides(t *testing.T) {
	script := readRepoFile(t, "airgap", "scripts", "29-install-stacks-via-api.sh")
	assert.False(t, strings.Contains(script, "stack-overrides/linux-"),
		"29 가 아키텍처별 덮어쓰기를 다시 싣는다 — 설치 백엔드(arch-images.go)가 맡는다")
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
