package helm

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

var updateAirgapValues = flag.Bool("update-airgap-values", false,
	"airgap/helm/charts-catalog-values/installer 를 설치 코드의 values 로 다시 쓴다")

func repoPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
	return filepath.Join(append([]string{root}, parts...)...)
}

// airgapInstallerValues 는 에어갭 모드에서 설치 단계마다 실제로 쓰는 차트 values 다.
//
// 번들 이미지 목록은 차트를 렌더해 만드는데, 차트 기본값으로 렌더하면 설치가 values 로
// 덮어쓰는 이미지(bitnamilegacy postgresql · redis, Jenkins 자체 이미지, OpenBao 2.5.5,
// OTel 수집기 등)가 목록에서 빠진다. 생성기(00-generate-images.sh)가 이 파일로도 렌더한다.
func airgapInstallerValues(t *testing.T) map[string]string {
	t.Helper()
	t.Setenv("NULLUS_HELM_OCI_REGISTRY", "kind-registry:5000/charts")

	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
	o.SetStackConfig(domain.StackConfig{AccessDomain: "nullus.internal"})

	files := map[string]string{}
	for _, step := range domain.InstallStepOrder {
		spec, ok := DefaultChartSpecForStep(step)
		if !ok {
			continue
		}
		body, err := yaml.Marshal(redactGeneratedSecrets(o.valuesForStep(step, spec)))
		require.NoError(t, err, step)
		files[step+".yaml"] = fmt.Sprintf(
			"# AUTO-GENERATED — go test ./internal/stack/adapter/helm -run TestAirgapInstallerValues -update-airgap-values\n"+
				"# step: %s\n# chart: %s %s\n%s",
			step, chartBaseName(spec.ChartName), strings.TrimPrefix(spec.Version, "v"), body)
	}
	return files
}

// generatedSecretKeys 는 설치가 매번 새로 만드는 비밀값이다. 파일에 그대로 쓰면 values 가
// 실행마다 달라 계약이 성립하지 않고, 비밀값이 저장소에 남는다. 이미지 렌더에는 영향이 없다.
var generatedSecretKeys = map[string]bool{
	"server.secretkey": true, // Argo CD 세션 서명 키(installing_argocd)
}

func redactGeneratedSecrets(v any) any {
	switch m := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			if _, isString := val.(string); isString && generatedSecretKeys[k] {
				out[k] = "<generated-at-install>"
				continue
			}
			out[k] = redactGeneratedSecrets(val)
		}
		return out
	case []any:
		out := make([]any, len(m))
		for i, val := range m {
			out[i] = redactGeneratedSecrets(val)
		}
		return out
	}
	return v
}

func TestAirgapInstallerValues(t *testing.T) {
	want := airgapInstallerValues(t)
	dir := repoPath(t, "airgap", "helm", "charts-catalog-values", "installer")

	if *updateAirgapValues {
		require.NoError(t, os.MkdirAll(dir, 0o755))
		old, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
		for _, f := range old {
			require.NoError(t, os.Remove(f))
		}
		for name, body := range want {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
		}
		return
	}

	got := map[string]string{}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		require.NoError(t, err)
		got[filepath.Base(p)] = string(raw)
	}
	names := func(m map[string]string) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	require.Equal(t, names(want), names(got),
		"설치 단계가 바뀌었다 — -update-airgap-values 로 다시 쓰고 00-generate-images.sh 로 images.txt 를 재생성하라")
	for name := range want {
		assert.Equalf(t, want[name], got[name],
			"%s: 설치 values 가 바뀌었다 — -update-airgap-values 로 다시 쓰고 images.txt 를 재생성하라", name)
	}
}

// airgapInstallerValuesHash 는 설치 values 파일들의 해시다(파일 이름 정렬 순서로 내용을 이어 붙임).
// 00-generate-images.sh 가 같은 규칙으로 계산해 images.txt 헤더에 남긴다.
func airgapInstallerValuesHash(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob(repoPath(t, "airgap", "helm", "charts-catalog-values", "installer", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "설치 values 파일이 없다 — -update-airgap-values 로 만들어라")
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		require.NoError(t, err)
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// 설치 values 가 바뀌었는데 images.txt 를 재생성하지 않으면, 번들에 설치가 쓰지 않는
// 이미지가 들어가거나 쓰는 이미지가 빠진다. 렌더에는 차트와 네트워크가 필요해 CI 에서
// 목록을 다시 만들 수는 없으므로, 목록이 어떤 values 로 만들어졌는지를 해시로 고정한다.
func TestAirgapImages_GeneratedFromCurrentInstallerValues(t *testing.T) {
	want := airgapInstallerValuesHash(t)
	m := regexp.MustCompile(`(?m)^# installer-values-sha256: ([0-9a-f]{64})$`).
		FindStringSubmatch(readRepoFile(t, "airgap", "images", "images.txt"))
	require.NotNil(t, m, "images.txt 헤더에 installer-values-sha256 가 없다 — 00-generate-images.sh 로 재생성하라")
	assert.Equal(t, want, m[1], "images.txt 가 지금의 설치 values 로 만들어지지 않았다 — 00-generate-images.sh 로 재생성하라")
}
