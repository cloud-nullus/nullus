package helm

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type catalogChart struct {
	name    string
	repoURL string
	version string
}

// airgapChartCatalog 는 airgap/scripts/pre/pull-charts-catalog.sh 의 CATALOG 항목이다.
// 형식: name|repo-name|repo-url|chart-ref|version
func airgapChartCatalog(t *testing.T) map[string]catalogChart {
	t.Helper()
	entry := regexp.MustCompile(`^\s*"([^|"]+)\|([^|"]*)\|([^|"]*)\|([^|"]*)\|([^|"]+)"`)
	out := map[string]catalogChart{}
	for _, line := range strings.Split(readRepoFile(t, "airgap", "scripts", "pre", "pull-charts-catalog.sh"), "\n") {
		m := entry.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ref := m[4]
		base := ref[strings.LastIndex(ref, "/")+1:]
		c := catalogChart{name: base, repoURL: m[3], version: strings.TrimPrefix(m[5], "v")}
		out[c.name+"@"+c.version] = c
	}
	require.NotEmpty(t, out, "pull-charts-catalog.sh 의 CATALOG 를 읽지 못했다")
	return out
}

// 에어갭 설치는 차트를 인터넷이 아니라 내부 OCI 레지스트리에서 받는다
// (installAirgapOCI — oci://<NULLUS_HELM_OCI_REGISTRY>/<차트 이름>). 그 레지스트리에
// 올라가는 차트는 pull-charts-catalog.sh 가 받은 것뿐이다.
//
// 설치 코드가 쓰는 차트가 카탈로그에 없으면 에어갭 스택 설치가 그 단계에서 멈춘다 —
// openbao · external-secrets · postgresql 이 빠져 있어 어떤 템플릿이든 시크릿 평면
// 첫 단계에서 멈췄다(nullus-plan#82). 설치 코드의 차트 · 버전 · 저장소를 카탈로그가
// 그대로 따르는지 고정한다.
func TestAirgapChartCatalog_CoversEveryInstallerChart(t *testing.T) {
	catalog := airgapChartCatalog(t)

	var missing, repoMismatch []string
	seen := map[string]bool{}
	for _, step := range domain.InstallStepOrder {
		spec, ok := DefaultChartSpecForStep(step)
		if !ok {
			continue // 차트가 아닌 단계(검증 · 프로비저닝 등)
		}
		name := chartBaseName(spec.ChartName)
		version := strings.TrimPrefix(spec.Version, "v")
		key := name + "@" + version
		if seen[key] {
			continue
		}
		seen[key] = true

		got, found := catalog[key]
		if !found {
			missing = append(missing, step+" → "+key)
			continue
		}
		if spec.RepoURL != "" && strings.TrimSuffix(got.repoURL, "/") != strings.TrimSuffix(spec.RepoURL, "/") {
			repoMismatch = append(repoMismatch, key+": 설치 "+spec.RepoURL+" / 카탈로그 "+got.repoURL)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "에어갭 카탈로그(pull-charts-catalog.sh)에 없는 설치 차트 — 에어갭 설치가 이 단계에서 멈춘다")
	assert.Empty(t, repoMismatch, "설치와 카탈로그가 다른 저장소에서 차트를 받는다")
}
