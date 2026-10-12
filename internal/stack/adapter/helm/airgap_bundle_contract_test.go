package helm

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// airgapBundleImages 는 airgap/images/images.txt 의 이미지를 docker.io/ · library/ 를 뗀 모양으로 돌려준다.
func airgapBundleImages(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, line := range strings.Split(readRepoFile(t, "airgap", "images", "images.txt"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[normalizeBundleImage(line)] = true
	}
	require.NotEmpty(t, out)
	return out
}

func normalizeBundleImage(ref string) string {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "docker.io/")
	return strings.TrimPrefix(ref, "library/")
}

// 설치 코드는 차트 기본 이미지를 values 로 덮어쓴다. 번들 목록은 차트 기본값을 렌더해
// 만들므로, 덮어쓴 이미지는 목록에 따로 있어야 한다 — 없으면 에어갭에서 그 파드가
// ImagePullBackOff 로 뜨지 않는다. 스택 PostgreSQL(bitnamilegacy 17.6.0)이 실제로 빠져 있었다.
func TestAirgapImages_IncludeInstallerImageOverrides(t *testing.T) {
	images := airgapBundleImages(t)
	for _, want := range []string{
		stackPostgresImageRegistry + "/" + stackPostgresImageRepository + ":" + stackPostgresImageTag,
		jenkinsImageRegistry + "/" + jenkinsImageRepository + ":" + jenkinsImageTag,
		OpenBaoImageRepository + ":" + OpenBaoImageTag,
	} {
		assert.Truef(t, images[normalizeBundleImage(want)],
			"설치 코드가 쓰는 %s 가 에어갭 번들 목록에 없다", want)
	}
}

// 에어갭 kind 노드는 kind-airgap.yaml 의 containerd 미러가 있는 레지스트리만 내부
// 레지스트리로 돌린다. 그 밖의 호스트 이미지는 폐쇄망에서 받을 수 없다 — 목록에
// oci.external-secrets.io(차트 저장소 주소를 이미지로 잘못 적은 것)가 있었다.
func TestAirgapImages_UseMirroredRegistries(t *testing.T) {
	mirror := regexp.MustCompile(`registry\.mirrors\."([^"]+)"`)
	mirrored := map[string]bool{}
	for _, m := range mirror.FindAllStringSubmatch(readRepoFile(t, "airgap", "kind", "kind-airgap.yaml"), -1) {
		mirrored[m[1]] = true
	}
	require.NotEmpty(t, mirrored)

	var unmirrored []string
	for image := range airgapBundleImages(t) {
		first := strings.SplitN(image, "/", 2)[0]
		if !strings.Contains(image, "/") || !(strings.ContainsAny(first, ".:") || first == "localhost") {
			continue // 레지스트리 생략 = docker.io
		}
		if !mirrored[first] {
			unmirrored = append(unmirrored, image)
		}
	}
	assert.Empty(t, unmirrored, "에어갭 kind 에 미러가 없는 레지스트리의 이미지다")
}

// 목록 생성기는 helm template 으로 차트를 렌더한다. helm v4 의 기본 kubeVersion 은 v1.20 이라
// argo-cd(≥1.25) · cert-manager(≥1.22) 렌더가 실패해 그 이미지가 목록에서 빠졌다. 에어갭 kind
// 노드의 쿠버네티스 버전으로 렌더해야 한다. OpenBao 는 차트가 아닌 설치 코드 상수를 따른다.
func TestAirgapImageGenerator_RendersForKindNodeAndInstallerOpenBao(t *testing.T) {
	script := readRepoFile(t, "airgap", "scripts", "00-generate-images.sh")
	assert.True(t, strings.Contains(script, "--kube-version"), "helm template 에 kubeVersion 을 주지 않으면 v1.20 으로 렌더한다")
	assert.True(t, strings.Contains(script, "kind-airgap.yaml"), "kubeVersion 은 에어갭 kind 노드 이미지에서 온다")

	code := stripYAMLComments(script)
	assert.False(t, strings.Contains(code, "openbao/openbao:latest"), "OpenBao 를 latest 로 받는다")
	assert.True(t, strings.Contains(code, `"`+OpenBaoImageRepository+":"+OpenBaoImageTag+`"`), "OpenBao 태그가 설치 상수와 다르다")
}

// 에어갭 스택 설치는 오프라인에서 차트(28) · Trivy DB(14)를 내부 레지스트리에 올리고
// API 로 설치한다(29). 번들이 이 스크립트를 싣지 않으면 install.sh 가 없는 파일을 부르고
// 경고로 넘어간다. Trivy DB 는 install.sh 가 올려야 서버가 받는다.
func TestAirgapBundle_ShipsStackInstallScripts(t *testing.T) {
	pkg := readRepoFile(t, "airgap", "scripts", "pre", "package-bundle.sh")
	for _, s := range []string{"14-push-oci-artifacts.sh", "28-push-charts-oci.sh", "29-install-stacks-via-api.sh"} {
		assert.Truef(t, strings.Contains(pkg, s), "번들에 %s 가 없다", s)
	}
	assert.True(t, strings.Contains(readRepoFile(t, "airgap", "install.sh"), "14-push-oci-artifacts.sh"),
		"install.sh 가 Trivy DB(OCI 아티팩트)를 내부 레지스트리에 올리지 않는다")
	assert.True(t, strings.Contains(readRepoFile(t, "airgap", "scripts", "pre", "pull-binaries.sh"), "oras"),
		"14-push-oci-artifacts.sh 는 oras 가 필요한데 번들 바이너리에 없다")
}

// 29-install-stacks-via-api.sh 는 nullus-bootstrap 으로 무인 설치 토큰을 받는다. 번들에 그
// 바이너리가 없으면 오프라인에서는 받을 곳이 없어 스택 설치가 토큰 단계에서 멈췄다.
// install.sh 가 PATH 에 넣은 번들 bin 은 install.sh 가 끝나면 사라지므로 29 가 직접 찾아야 한다.
func TestAirgapBundle_ShipsBootstrapCLI(t *testing.T) {
	assert.True(t, strings.Contains(readRepoFile(t, "airgap", "scripts", "pre", "pull-binaries.sh"), "./cmd/nullus-bootstrap"),
		"번들 바이너리에 nullus-bootstrap 이 없다")
	assert.True(t, strings.Contains(readRepoFile(t, "airgap", "scripts", "29-install-stacks-via-api.sh"), "bin/${PLATFORM}/nullus-bootstrap"),
		"29-install-stacks-via-api.sh 가 번들 bin 의 nullus-bootstrap 을 찾지 않는다")
}

// 공식 goharbor · nexus3 처럼 arm64 이미지를 제공하지 않는 이미지가 목록에 있으면 arm64 에서
// 번들 생성이 01-pull-images 에서 멈췄다. 01 이 그 이미지를 건너뛰어 기록하고, 번들 저장(02)과
// 오프라인 push(12)가 같은 기록으로 그 이미지를 뺀다 — 한 곳이라도 빠지면 그 단계에서 멈춘다.
func TestAirgapBundle_PlatformSkippedImagesShareOneRecord(t *testing.T) {
	for _, script := range []string{"01-pull-images.sh", "02-save-bundle.sh", "12-push-to-registry.sh"} {
		assert.Truef(t, strings.Contains(readRepoFile(t, "airgap", "scripts", script), "images.skipped-platform.txt"),
			"%s 가 대상 플랫폼 이미지가 없는 이미지 기록을 쓰지 않는다", script)
	}
}

// 에어갭에서 Nullus 는 nullus 네임스페이스에 산다. 29 의 기본 스택 네임스페이스가 그 자리면
// 스택 생성 API 가 플랫폼 네임스페이스라며 거부해 기본값으로는 스택 설치가 되지 않았다.
// 비워 두면 API 가 스택 이름으로 nullus-<이름> 을 만든다.
func TestAirgapStackInstallScript_DefaultNamespaceIsNotPlatform(t *testing.T) {
	m := regexp.MustCompile(`STACK_NAMESPACE="\$\{STACK_NAMESPACE:-([^}]*)\}"`).
		FindStringSubmatch(readRepoFile(t, "airgap", "scripts", "29-install-stacks-via-api.sh"))
	require.NotNil(t, m, "29-install-stacks-via-api.sh 에 STACK_NAMESPACE 기본값이 없다")
	assert.Truef(t, m[1] == "" || strings.HasPrefix(m[1], "nullus-"),
		"기본 스택 네임스페이스 %q 는 플랫폼 네임스페이스와 겹칠 수 있다", m[1])
}
