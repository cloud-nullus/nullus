package domain

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// 오브젝트 스토리지 이미지는 받을 수 있는 출처에서만 받는다.
//
// MinIO 는 Docker Hub 배포를 멈췄고(docker.io/minio/* 는 태그와 무관하게
// "pull access denied"), 그 다음 quay.io 저장소도 익명 pull 을 막았다 —
// 2026-10-17 실측: quay.io/minio/minio · quay.io/minio/mc 가 `latest` 든 고정
// RELEASE 태그든 401 unauthorized 다. 두 번 다 노드 캐시가 있는 클러스터에서는
// 드러나지 않다가 새 클러스터에서 ImagePullBackOff 로 설치를 실패시켰다.
//
// 그래서 포크(pgsty/silo · pgsty/mc)에서 받는다. 둘 중 하나로 되돌리면 이 테스트가 막는다.
func TestRuntimeImages_오브젝트스토리지_이미지는_받을수있는_출처에서_받는다(t *testing.T) {
	unreachable := []string{"minio/", "quay.io/minio/"}
	for _, img := range append(RuntimeImages(), MinIOServerImage) {
		repo := strings.TrimPrefix(img, "docker.io/")
		for _, prefix := range unreachable {
			assert.Falsef(t, strings.HasPrefix(repo, prefix),
				"%s 는 익명 pull 이 막힌 출처다(%s) — pgsty/ 포크에서 받아야 한다", img, prefix)
		}
	}
	assert.True(t, strings.HasPrefix(MinIOClientImage, "docker.io/pgsty/mc:"), MinIOClientImage)
	assert.True(t, strings.HasPrefix(MinIOServerImage, "docker.io/pgsty/silo:"), MinIOServerImage)
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

// 공식 sonar-scanner-cli 이미지는 amd64 만 낸다(Docker Hub 전 태그 실측, 2026-10). arm64
// 노드에서는 에뮬레이션 없이는 SAST 단계가 서지 않고, 에어갭 arm64 번들은 그 이미지를
// 아예 건너뛴다. 스캐너 zip 은 플랫폼 독립이라 멀티아치 JRE 위에 우리가 굽는다 — Jenkins
// 이미지와 같은 자리(ghcr)·같은 CD 잡 모양이다. 태그는 Dockerfile 의 스캐너 버전이
// 단일 출처다.
func TestSonarScannerImage_우리가_굽는_멀티아치_이미지를_쓴다(t *testing.T) {
	const repository = "ghcr.io/cloud-nullus/nullus/nullus-sonar-scanner"
	assert.True(t, strings.HasPrefix(SonarScannerImage, repository+":"), SonarScannerImage)

	dockerfile, err := os.ReadFile(filepath.Join(repoRoot(t), "deploy", "images", "sonar-scanner", "Dockerfile"))
	require.NoError(t, err)
	version := regexp.MustCompile(`(?m)^ARG SONAR_SCANNER_VERSION=(\S+)$`).FindSubmatch(dockerfile)
	require.Len(t, version, 2, "Dockerfile 에 ARG SONAR_SCANNER_VERSION 이 있어야 한다")
	revision := regexp.MustCompile(`(?m)^ARG NULLUS_IMAGE_REVISION=(\d+)$`).FindSubmatch(dockerfile)
	require.Len(t, revision, 2, "Dockerfile 에 ARG NULLUS_IMAGE_REVISION 이 있어야 한다")
	assert.Equal(t, repository+":"+string(version[1])+"-"+string(revision[1]), SonarScannerImage,
		"태그는 Dockerfile 의 스캐너 버전-이미지 리비전이다")

	// 태그를 계산하는 곳은 스크립트 하나다. CD · 이미지 스캔 · 로컬 빌드가 각자 grep 하면 규칙이 갈린다.
	tag, err := exec.Command(filepath.Join(repoRoot(t), "scripts", "sonar-scanner-image-tag.sh")).Output()
	require.NoError(t, err)
	assert.Equal(t, repository+":"+strings.TrimSpace(string(tag)), SonarScannerImage)

	for _, path := range []string{
		filepath.Join(".github", "workflows", "cd.yml"),
		filepath.Join(".github", "workflows", "image-scan.yml"),
		filepath.Join("scripts", "build-sonar-scanner-image.sh"),
	} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), path))
		require.NoError(t, err)
		assert.Contains(t, string(raw), "sonar-scanner-image-tag.sh", "%s 는 태그를 스크립트로 계산해야 한다", path)
	}
	cd, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "cd.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(cd), "deploy/images/sonar-scanner/Dockerfile", "CD 가 이 이미지를 굽는다")
	assert.Contains(t, string(cd), "nullus-sonar-scanner:", "CD 가 같은 이름으로 민다")
}

// 같은 스캐너 버전에서 이미지 내용이 바뀌면 리비전을 올려 태그를 바꾼다. 태그가 그대로면 CD 가 같은 태그로
// 다시 밀어도 이미 받아 둔 노드는 옛 이미지를 계속 쓴다(쿠버네티스 기본 pull 정책 IfNotPresent) — Node 를
// 넣은 이미지가 그 노드에는 영영 닿지 않는다. 리비전 1 은 접미사 없는 첫 태그(8.1.0.6389)였다.
func TestSonarScannerImage_내용이_바뀐_이미지는_새_태그다(t *testing.T) {
	assert.NotEqual(t, "ghcr.io/cloud-nullus/nullus/nullus-sonar-scanner:8.1.0.6389", SonarScannerImage,
		"Node 를 넣은 이미지가 옛 태그를 다시 쓰면 안 된다")
}

// SonarQube 의 JS/TS/CSS 분석기는 Node 런타임을 플랫폼별로 내장하는데 LINUX_ARM64 용은 없어 PATH 의
// node 를 찾는다. 스캐너 이미지에 node 가 없으면 arm64 노드에서는 .js·.css 가 하나만 있어도 분석 전체가
// EXECUTION FAILURE 로 끝나고, SAST 를 기다리는 deploy 가 돌지 않는다(2026-10-19 kind arm64 ·
// SonarQube 26.9 · 전자정부 표준프레임워크 샘플의 정적 CSS/JS 로 실측).
func TestSonarScannerImage_JS_분석용_Node_가_있다(t *testing.T) {
	dockerfile, err := os.ReadFile(filepath.Join(repoRoot(t), "deploy", "images", "sonar-scanner", "Dockerfile"))
	require.NoError(t, err)

	stages := strings.Split(string(dockerfile), "\nFROM ")
	runtime := stages[len(stages)-1]
	assert.Regexp(t, `dnf install -y [^\n]*\bnodejs\d+\b`, runtime, "실행 단계가 Node.js 를 설치해야 한다")
	assert.Contains(t, runtime, "node --version", "빌드가 node 가 PATH 에 잡히는지 확인해야 한다")
}

// Trivy CLI 이미지는 CI 잡이 쓴다. 정책 변수와 함께 밀려야 하므로(port.ScanPolicyVariables)
// 스캐폴드 어댑터가 아니라 여기 있어야 한다. 에어갭 번들은 Trivy 차트가 같은 태그를 이미 싣는다.
func TestTrivyCLIImage_에어갭_번들에_있다(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "airgap", "images", "images.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), TrivyCLIImage,
		"CI 잡의 Trivy 이미지가 에어갭 번들에 없다 — 폐쇄망에서 이미지 스캔 단계가 ImagePullBackOff 다")
}
