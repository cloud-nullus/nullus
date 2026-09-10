package helm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 취약점 DB 미러 경로가 세 곳에 흩어져 있다:
//
//  1. airgap/images/oci-artifacts.txt          — 무엇을 반입하나
//  2. airgap/scripts/14-push-oci-artifacts.sh  — 어디로 올리나
//  3. airgap/helm/stack-values/trivy.yaml      — 서버가 어디서 찾나
//
// 셋이 갈라지면 서버는 정상으로 뜨는데 없는 경로에서 DB 를 찾는다. 스캔이 전부
// 실패하고, 오류가 스캐너 장애처럼 보여 원인을 찾기 어렵다.
//
// 반입(1)과 조회(3)가 같은 곳을 가리키는지 고정한다.
func TestAirgapTrivyDBMirror_MatchesStackValues(t *testing.T) {
	const registryHost = "localhost:5001"

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

	// 14-push-oci-artifacts.sh 의 compute_target 규칙:
	// 레지스트리 호스트를 벗겨 내부 레지스트리 경로로 바꾼다.
	// trivy 는 dbRepository 에 태그를 붙이지 않는다 — 스키마 버전 태그는 스스로 붙인다.
	path := upstream[strings.Index(upstream, "/")+1:]
	wantRepository := registryHost + "/" + path[:strings.LastIndex(path, ":")]

	values := readRepoFile(t, "airgap", "helm", "stack-values", "trivy.yaml")
	assert.Contains(t, values, "dbRepository: "+wantRepository,
		"stack-values 의 dbRepository 가 반입 경로와 다르면 서버가 없는 곳에서 DB 를 찾는다")
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
