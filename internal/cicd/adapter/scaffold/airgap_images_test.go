package scaffold

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 이 테스트가 지키는 것: **클러스터 안에서 도는 파이프라인 잡의 이미지가 에어갭 번들에
// 전부 들어 있는가.**
//
// 이미지 이름을 내부 레지스트리로 바꾸는 코드는 없다. 폐쇄망 노드의 containerd 가
// docker.io·quay.io 등을 내부 레지스트리로 미러링하므로(airgap/kind/kind-airgap.yaml)
// 번들 목록(airgap/images/images.txt)에 오른 이미지만 받힌다. 목록에 없는 잡 이미지는
// 설치가 아니라 첫 파이프라인 실행에서야 ImagePullBackOff 로 드러난다 — GitLab 의
// build·deploy 와 SAST 스캐너가 실제로 그렇게 빠져 있었다.
//
// GitHub Actions 는 GitHub 호스티드 러너에서 돌아 에어갭 번들과 무관하다.
func TestRenderPipeline_InClusterJobImagesAreInAirgapBundle(t *testing.T) {
	bundle := airgapBundleImages(t)

	tests := []struct {
		ci      port.CIPlatform
		extract func(t *testing.T, content string) []string
	}{
		{port.CIPlatformGitLabCI, gitLabJobImages},
		{port.CIPlatformJenkins, jenkinsPodImages},
	}
	for _, tc := range tests {
		t.Run(string(tc.ci), func(t *testing.T) {
			// 스택이 고를 수 있는 단계를 모두 켠다 — 꺼진 단계의 이미지는 렌더되지 않아 검사를 빠져나간다.
			_, content := renderPipelineFor(sastInput(tc.ci, testSASTEndpoint, testScannerEndpoint))
			images := tc.extract(t, content)
			require.NotEmpty(t, images, "렌더 결과에서 잡 이미지를 찾지 못했다 — 추출 규칙이 렌더러와 어긋났다")

			var missing []string
			for _, img := range images {
				if !bundle[normalizeImageRef(img)] {
					missing = append(missing, img)
				}
			}
			assert.Empty(t, missing,
				"파이프라인 잡 이미지가 에어갭 번들에 없다 — 폐쇄망에서 첫 실행이 ImagePullBackOff 로 멈춘다.\n"+
					"internal/shared/domain/runtime_images.go 와 airgap/scripts/00-generate-images.sh 의 "+
					"RUNTIME_IMAGES 에 넣고 목록을 재생성하라: %v", missing)
		})
	}
}

// 스캐너 이미지는 GitLab 잡에서 변수로 참조된다. 변수 기본값까지 따라가지 않으면 위 검사가
// 스캐너를 통째로 놓친다.
func TestGitLabJobImages_ResolvesImageVariables(t *testing.T) {
	_, content := renderPipelineFor(sastInput(port.CIPlatformGitLabCI, testSASTEndpoint, testScannerEndpoint))

	images := gitLabJobImages(t, content)

	assert.Contains(t, images, defaultSASTScannerImage)
	assert.Contains(t, images, defaultScannerImage)
	for _, img := range images {
		assert.NotContains(t, img, "$", "변수를 풀지 못한 이미지 참조가 남았다: %s", img)
	}
}

func airgapBundleImages(t *testing.T) map[string]bool {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "airgap", "images", "images.txt"))
	require.NoError(t, err, "에어갭 이미지 목록을 읽지 못했다")

	out := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[normalizeImageRef(line)] = true
	}
	return out
}

// normalizeImageRef 는 같은 이미지를 다르게 적은 표기를 하나로 맞춘다.
// 목록은 docker.io/ 를 붙이기도 하고 생략하기도 한다.
func normalizeImageRef(ref string) string {
	ref = strings.TrimPrefix(ref, "docker.io/")
	return strings.TrimPrefix(ref, "library/")
}

// gitLabJobImages 는 .gitlab-ci.yml 의 잡 이미지와 서비스 이미지를 모은다.
// `$VAR` 참조는 최상위 variables 의 기본값으로 푼다.
func gitLabJobImages(t *testing.T, content string) []string {
	t.Helper()
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(content), &doc))

	vars := map[string]string{}
	if v, ok := doc["variables"].(map[string]any); ok {
		for k, val := range v {
			if s, ok := val.(string); ok {
				vars[k] = s
			}
		}
	}
	resolve := func(ref string) string {
		if name, ok := strings.CutPrefix(ref, "$"); ok {
			if val, ok := vars[strings.Trim(name, "{}")]; ok {
				return val
			}
		}
		return ref
	}
	imageName := func(v any) string {
		switch x := v.(type) {
		case string:
			return x
		case map[string]any:
			name, _ := x["name"].(string)
			return name
		}
		return ""
	}

	var out []string
	for key, val := range doc {
		job, ok := val.(map[string]any)
		if !ok || key == "variables" {
			continue
		}
		if img := imageName(job["image"]); img != "" {
			out = append(out, resolve(img))
		}
		if svcs, ok := job["services"].([]any); ok {
			for _, s := range svcs {
				if img := imageName(s); img != "" {
					out = append(out, resolve(img))
				}
			}
		}
	}
	return out
}

var jenkinsPodImageLine = regexp.MustCompile(`(?m)^\s+image:\s+(\S+)\s*$`)

// jenkinsPodImages 는 Jenkinsfile 에 박힌 에이전트 파드 YAML 의 컨테이너 이미지를 모은다.
func jenkinsPodImages(t *testing.T, content string) []string {
	t.Helper()
	var out []string
	for _, m := range jenkinsPodImageLine.FindAllStringSubmatch(content, -1) {
		out = append(out, m[1])
	}
	return out
}
