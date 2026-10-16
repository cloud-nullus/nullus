package helm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 취약점 DB 미러 경로가 네 곳에 흩어져 있다:
//
//  1. airgap/images/oci-artifacts.txt          — 무엇을 반입하나
//  2. airgap/scripts/14-push-oci-artifacts.sh  — 어디로 올리나 (호스트에서 localhost:5001)
//  3. airgap/helm/values-airgap.yaml           — 클러스터 안에서 그 레지스트리를 부르는 이름
//  4. 서버가 어디서 찾나 — API 설치는 trivyAirgapDBValues, helm 직접 설치는 stack-values/trivy.yaml
//
// 셋이 갈라지면 서버는 정상으로 뜨는데 없는 경로에서 DB 를 찾는다. 스캔이 전부
// 실패하고, 오류가 스캐너 장애처럼 보여 원인을 찾기 어렵다.
//
// 반입 경로를 **클러스터 안의 주소**로 옮긴 것이 조회 경로여야 한다. 예전 계약은
// 호스트 주소(localhost:5001)와 같은지만 봤다 — 파드 안의 localhost 는 파드 자신이라
// 그 값으로는 DB 를 받을 수 없었다.
func TestAirgapTrivyDBMirror_MatchesStackValues(t *testing.T) {
	artifacts := readRepoFile(t, "airgap", "images", "oci-artifacts.txt")

	// oci-artifacts.txt 의 trivy-db 항목을 찾는다.
	var upstream string
	for _, line := range strings.Split(artifacts, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "/trivy-db:") {
			upstream = line
			break
		}
	}
	require.NotEmpty(t, upstream,
		"oci-artifacts.txt 에 trivy-db 가 없으면 에어갭에서 DB 를 반입할 방법이 없다")

	// 클러스터 안에서 내부 레지스트리를 부르는 이름은 values-airgap.yaml 의 ociRegistry 다.
	var ociRegistry string
	for _, line := range strings.Split(readRepoFile(t, "airgap", "helm", "values-airgap.yaml"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ociRegistry:") {
			ociRegistry = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "ociRegistry:")), `"`)
		}
	}
	require.NotEmpty(t, ociRegistry)

	// 14-push-oci-artifacts.sh 의 compute_target 규칙: 레지스트리 호스트를 벗겨
	// 내부 레지스트리 루트 아래로 옮긴다. trivy 는 dbRepository 에 태그를 붙이지 않는다.
	path := upstream[strings.Index(upstream, "/")+1:]
	inClusterHost := strings.SplitN(ociRegistry, "/", 2)[0]
	wantRepository := inClusterHost + "/" + path[:strings.LastIndex(path, ":")]

	// API 설치 경로
	trivy, ok := trivyAirgapDBValues(ociRegistry)["trivy"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, wantRepository, trivy["dbRepository"],
		"API 설치의 dbRepository 가 반입 경로와 다르면 서버가 없는 곳에서 DB 를 찾는다")

	// helm 직접 설치 경로
	values := stripYAMLComments(readRepoFile(t, "airgap", "helm", "stack-values", "trivy.yaml"))
	assert.Contains(t, values, "dbRepository: "+wantRepository,
		"stack-values 의 dbRepository 가 반입 경로와 다르면 서버가 없는 곳에서 DB 를 찾는다")
	assert.NotContains(t, values, "localhost:5001/aquasecurity",
		"파드 안의 localhost 는 파드 자신이다 — 레지스트리에 닿지 못한다")
	assert.Contains(t, values, `TRIVY_INSECURE: "true"`, "내부 레지스트리는 plain HTTP 다")
}

// 에어갭 번들 이미지 목록에 Trivy 이미지가 있어야 한다.
//
// 스택 Trivy 서버(차트)와 CI 스캔 잡(client)이 같은 aquasec/trivy 이미지를 쓴다. 목록은
// 00-generate-images.sh 가 카탈로그 차트를 렌더해 만드는데, Trivy 카탈로그를 넣은 뒤
// 재생성하지 않아 목록에서 빠져 있었다 — 폐쇄망 설치에서 서버가 ImagePullBackOff 로 뜨지
// 않는다. 버전은 설치가 쓰는 앱 버전과 같아야 한다.
func TestAirgapImages_IncludeTrivy(t *testing.T) {
	want := "aquasec/trivy:" + domain.TrivyAppVersion
	var found bool
	for _, line := range strings.Split(readRepoFile(t, "airgap", "images", "images.txt"), "\n") {
		if strings.TrimPrefix(strings.TrimSpace(line), "docker.io/") == want {
			found = true
		}
	}
	assert.True(t, found, "airgap/images/images.txt 에 %s 가 없다 — 00-generate-images.sh 로 재생성하라", want)
}

// 에어갭 번들 목록에 받을 수 없는 이미지가 있으면 번들 생성이 멈춘다.
//
// 01-pull-images.sh 는 하나라도 못 받으면 exit 1 이다. GitLab 차트에 딸린 MinIO
// 서브차트(minio/minio:RELEASE.2017…, minio/mc:RELEASE.2018…)가 목록에 들어 있었는데,
// MinIO 가 Docker Hub 배포를 멈춰 익명 pull 이 거절된다 — 어느 머신에서도 번들을 만들
// 수 없었다. 실제 설치는 global.minio.enabled=false 로 그 서브차트를 끄므로(values.go),
// 목록을 만드는 카탈로그 values 도 같게 두고 목록에 Docker Hub 의 minio/* 가 없어야 한다.
func TestAirgapImages_NoDockerHubMinIO(t *testing.T) {
	for _, line := range strings.Split(readRepoFile(t, "airgap", "images", "images.txt"), "\n") {
		repo := strings.TrimPrefix(strings.TrimSpace(line), "docker.io/")
		assert.Falsef(t, strings.HasPrefix(repo, "minio/"),
			"%s 는 Docker Hub 의 MinIO 이미지다 — 받을 수 없어 번들 생성이 멈춘다", line)
	}

	values := stripYAMLComments(readRepoFile(t, "airgap", "helm", "charts-catalog-values", "gitlab.yaml"))
	assert.Regexp(t, `(?m)^\s*minio:\s*\n\s*enabled:\s*false`, values,
		"카탈로그 values 가 GitLab 내장 MinIO 를 끄지 않으면 재생성할 때 목록에 다시 들어온다")
}

// 카탈로그용 values 는 업스트림을 가리켜야 한다.
//
// rewrite_upstream(00-generate-images.sh)은 cloud-nullus / dasomel / bitnami
// 접두사만 되돌린다. aquasec 를 localhost:5001 로 적으면 그 경로가 그대로
// images.txt 에 박히고, pull 단계가 아직 존재하지 않는 내부 레지스트리에서
// 이미지를 받으려 한다.
func TestAirgapTrivyCatalogValues_PointUpstream(t *testing.T) {
	values := readRepoFile(t, "airgap", "helm", "charts-catalog-values", "trivy.yaml")

	// 주석은 뺀다 — 이 파일의 주석이 바로 "localhost:5001 을 쓰지 말라" 는
	// 설명이라, 통째로 검사하면 규칙을 적어둔 것이 규칙 위반으로 잡힌다.
	assert.NotContains(t, stripYAMLComments(values), "localhost:5001",
		"카탈로그 values 는 이미지 추출용이라 업스트림 경로여야 한다")
	assert.Contains(t, values, "registry: docker.io")
}

func stripYAMLComments(raw string) string {
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))

	raw, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	require.NoError(t, err, "파일을 읽지 못했다: %v", parts)
	return string(raw)
}
