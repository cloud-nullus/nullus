package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 에어갭 설치는 API 가 스택을 설치한다(29-install-stacks-via-api.sh). 그 경로는
// airgap/helm/stack-values/trivy.yaml 을 읽지 않아, 서버가 ghcr.io 에서 DB 를 받으려다
// 스캔이 전부 실패했다. 에어갭 모드면 코드가 내부 레지스트리를 가리킨다.
func TestTrivyAirgapDBValues(t *testing.T) {
	assert.Nil(t, trivyAirgapDBValues(""), "에어갭이 아니면 업스트림 기본값을 그대로 둔다")

	for _, registry := range []string{"kind-registry:5000/charts", "kind-registry:5000", " kind-registry:5000/charts/ "} {
		t.Run(registry, func(t *testing.T) {
			values := trivyAirgapDBValues(registry)
			trivy, ok := values["trivy"].(map[string]any)
			require.True(t, ok)
			// 차트 저장소 경로(/charts)가 아니라 레지스트리 루트의 반입 경로다
			// (14-push-oci-artifacts.sh 의 compute_target).
			assert.Equal(t, "kind-registry:5000/aquasecurity/trivy-db", trivy["dbRepository"])
			// 내부 레지스트리는 plain HTTP 다. insecure 가 없으면 https 로 붙어 실패한다.
			assert.Equal(t, map[string]any{"TRIVY_INSECURE": "true"}, trivy["extraEnvVars"])
		})
	}
}

func trivyValuesFromOrchestrator(t *testing.T, cfg domain.StackConfig) map[string]any {
	t.Helper()
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus-devsecops-stack")
	o.SetStackConfig(cfg)
	spec, ok := DefaultChartSpecForStep("installing_trivy")
	require.True(t, ok)
	trivy, ok := o.valuesForStep("installing_trivy", spec)["trivy"].(map[string]any)
	require.True(t, ok)
	return trivy
}

func TestTrivyValues_UseAirgapDBMirror(t *testing.T) {
	withOverride := domain.StackConfig{YAMLOverrides: map[string]string{"installing_harbor": "core:\n  replicas: 1\n"}}

	t.Run("온라인 설치는 업스트림", func(t *testing.T) {
		t.Setenv("NULLUS_HELM_OCI_REGISTRY", "")
		trivy := trivyValuesFromOrchestrator(t, domain.StackConfig{})
		assert.Equal(t, "ghcr.io/aquasecurity/trivy-db", trivy["dbRepository"])
		assert.Nil(t, trivy["extraEnvVars"])
	})

	// 다른 도구에 YAML 오버라이드가 있으면 values 조립이 다른 분기를 탄다.
	// 한쪽에만 두면 오버라이드 하나로 에어갭 DB 경로가 사라진다.
	for name, cfg := range map[string]domain.StackConfig{"오버라이드 없음": {}, "다른 도구 오버라이드": withOverride} {
		t.Run("에어갭 — "+name, func(t *testing.T) {
			t.Setenv("NULLUS_HELM_OCI_REGISTRY", "kind-registry:5000/charts")
			trivy := trivyValuesFromOrchestrator(t, cfg)
			assert.Equal(t, "kind-registry:5000/aquasecurity/trivy-db", trivy["dbRepository"])
			assert.Equal(t, map[string]any{"TRIVY_INSECURE": "true"}, trivy["extraEnvVars"])
			assert.Equal(t, false, trivy["skipDBUpdate"], "미러에서 정상 갱신 경로를 탄다")
		})
	}
}
