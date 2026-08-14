package domain

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Jenkins 차트 버전은 자유롭게 내릴 수 없다.
//
// Gitea multibranch 스캔에 쓰는 jenkinsci/gitea 플러그인이 Jenkins 2.528.3
// 이상을 요구한다. 하한 아래로 내리면 플러그인 설치가 실패하고, 그러면 리포를
// 만들어도 Jenkins 가 Jenkinsfile 을 찾지 못해 파이프라인이 돌지 않는다.
// 실패가 설치 시점이 아니라 첫 빌드 시점에 나타나므로 원인을 찾기 어렵다.
func TestJenkinsAppVersion_SatisfiesGiteaPluginMinimum(t *testing.T) {
	got := parseJenkinsVersion(t, JenkinsAppVersion)
	min := parseJenkinsVersion(t, JenkinsGiteaPluginMinAppVersion)

	require.GreaterOrEqualf(t, compareVersions(got, min), 0,
		"Jenkins appVersion %s 이 gitea 플러그인 하한 %s 보다 낮다 — 플러그인을 설치할 수 없다",
		JenkinsAppVersion, JenkinsGiteaPluginMinAppVersion)
}

func parseJenkinsVersion(t *testing.T, v string) []int {
	t.Helper()
	parts := strings.Split(strings.TrimSpace(v), ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		require.NoErrorf(t, err, "버전 %q 를 해석할 수 없다", v)
		out = append(out, n)
	}
	return out
}

func compareVersions(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}
