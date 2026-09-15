package domain

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 이 테스트가 지키는 것: **런타임 매니페스트가 참조하는 이미지가 에어갭
// 번들에 전부 들어 있는가.**
//
// 번들 목록은 `helm template` 렌더 결과에서 자동 생성되므로 차트에 없는
// 이미지는 오르지 않는다. Go 코드가 만드는 Job·파드의 이미지가 정확히 그
// 사각지대이고, 실제로 5종이 빠져 있었다(2026-09-02). 폐쇄망에서
// ImagePullBackOff 로만, 그것도 설치가 한참 진행된 뒤에 드러난다.
//
// 이미지를 새로 참조하게 되면 RuntimeImages() 에 넣고 번들에도 추가해야
// 한다. 둘 중 하나만 하면 여기서 걸린다.

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}

func bundledImages(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "airgap", "images", "images.txt"))
	require.NoError(t, err, "에어갭 이미지 목록을 읽지 못했다")

	out := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = true
		// 목록은 `docker.io/` 접두사를 붙이기도 하고 생략하기도 한다.
		// 같은 이미지를 다르게 적은 것이므로 양쪽 형태를 모두 인정한다.
		out[strings.TrimPrefix(line, "docker.io/")] = true
	}
	return out
}

func TestRuntimeImages_모두_에어갭_번들에_있다(t *testing.T) {
	bundle := bundledImages(t)

	var missing []string
	for _, img := range RuntimeImages() {
		if bundle[img] || bundle[strings.TrimPrefix(img, "docker.io/")] {
			continue
		}
		missing = append(missing, img)
	}

	assert.Empty(t, missing,
		"런타임 매니페스트가 쓰는 이미지가 에어갭 번들에 없다.\n"+
			"폐쇄망에서 해당 단계가 ImagePullBackOff 로 실패한다.\n"+
			"airgap/scripts/00-generate-images.sh 의 RUNTIME_IMAGES 에 추가하고 "+
			"목록을 재생성하라: %v", missing)
}

func TestRuntimeImages_생성기와_같은_목록을_본다(t *testing.T) {
	// 번들 목록은 생성물이라, 생성기가 이 이미지들을 덧붙이지 않으면
	// 재생성하는 순간 도로 사라진다. 위 테스트는 그때야 실패한다 —
	// 생성기 쪽도 함께 본다.
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "airgap", "scripts", "00-generate-images.sh"))
	require.NoError(t, err)
	script := string(raw)

	var missing []string
	for _, img := range RuntimeImages() {
		if !strings.Contains(script, img) {
			missing = append(missing, img)
		}
	}
	assert.Empty(t, missing,
		"생성기가 이 이미지를 덧붙이지 않는다 — 목록을 재생성하면 사라진다: %v", missing)
}

// MinIO 는 Docker Hub 배포를 멈췄다 — docker.io/minio/* 는 태그와 무관하게
// "pull access denied" 로 거절된다(2026-09-15 kind 새 클러스터 실측, minio/mc:latest 도 같다).
// 노드에 캐시가 남은 클러스터에서는 드러나지 않다가, 새 클러스터에서 버킷 부트스트랩이
// ImagePullBackOff 로 설치를 실패시켰다. MinIO 가 계속 게시하는 quay.io 에서 받는다.
func TestRuntimeImages_MinIO_이미지는_quay_에서_받는다(t *testing.T) {
	for _, img := range RuntimeImages() {
		repo := strings.TrimPrefix(img, "docker.io/")
		assert.Falsef(t, strings.HasPrefix(repo, "minio/"),
			"%s 는 Docker Hub 의 MinIO 이미지다 — quay.io/minio/ 에서 받아야 한다", img)
	}
	assert.True(t, strings.HasPrefix(MinIOClientImage, "quay.io/minio/mc:"), MinIOClientImage)
}

func TestRuntimeImages_형식(t *testing.T) {
	for _, img := range RuntimeImages() {
		assert.Contains(t, img, ":", "%s 에 태그가 없다 — latest 는 재현 가능하지 않다", img)
		assert.NotContains(t, img, ":latest", "%s 가 latest 다 — 번들과 실제가 갈라진다", img)
		assert.NotEmpty(t, strings.TrimSpace(img))
	}
}

func TestRuntimeImages_중복이_없다(t *testing.T) {
	seen := map[string]bool{}
	for _, img := range RuntimeImages() {
		assert.False(t, seen[img], "%s 가 중복이다", img)
		seen[img] = true
	}
}
