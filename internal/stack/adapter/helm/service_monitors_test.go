package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func monitoringEnabledConfig() *domain.StackConfig {
	return &domain.StackConfig{
		Monitoring: domain.MonitoringConfig{
			Collection: domain.ToolSelection{Name: "prometheus", Enabled: true},
		},
	}
}

// ServiceMonitor 를 만들어도 kube-prometheus-stack 의 release 라벨이 없으면
// 스크랩되지 않는다 — 리소스는 있는데 메트릭은 없는 상태가 된다.
func TestServiceMonitorValues_CarryPrometheusReleaseLabel(t *testing.T) {
	cases := []struct {
		step  string
		path  []string
		label []string
	}{
		{step: "installing_grafana", path: []string{"serviceMonitor"}, label: []string{"labels"}},
		{step: "installing_minio", path: []string{"metrics", "serviceMonitor"}, label: []string{"additionalLabels"}},
		{step: "installing_logging", path: []string{"serviceMonitor"}, label: []string{"additionalLabels"}},
	}

	for _, tc := range cases {
		t.Run(tc.step, func(t *testing.T) {
			values := serviceMonitorValuesForStep(tc.step, monitoringEnabledConfig())
			require.NotNil(t, values, "메트릭 노출이 켜지지 않았습니다")

			monitor, found := lookupValue(values, tc.path)
			require.True(t, found)
			block, ok := monitor.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, true, block["enabled"])

			labels, ok := block[tc.label[0]].(map[string]any)
			require.Truef(t, ok, "%s 가 없습니다", tc.label[0])
			assert.Equal(t, "kube-prometheus-stack", labels["release"])
		})
	}
}

// minio 차트의 템플릿 조건은 `and .enabled .includeNode` 다. enabled 만 켜면
// values 에는 실리는데 ServiceMonitor 는 만들어지지 않는다.
func TestServiceMonitorValues_MinIONeedsIncludeNode(t *testing.T) {
	values := serviceMonitorValuesForStep("installing_minio", monitoringEnabledConfig())
	require.NotNil(t, values)

	includeNode, found := lookupValue(values, []string{"metrics", "serviceMonitor", "includeNode"})
	require.True(t, found, "includeNode 가 없으면 차트가 ServiceMonitor 를 만들지 않는다")
	assert.Equal(t, true, includeNode)
}

// Argo CD 는 컴포넌트마다 metrics Service 가 따로다. 컨트롤러만 켜면
// 저장소 서버·서버 지표가 빠진다.
func TestServiceMonitorValues_ArgoCDCoversEveryComponent(t *testing.T) {
	values := serviceMonitorValuesForStep("installing_argocd", monitoringEnabledConfig())
	require.NotNil(t, values)

	for _, component := range []string{"controller", "server", "repoServer", "applicationSet", "notifications"} {
		enabled, found := lookupValue(values, []string{component, "metrics", "enabled"})
		require.Truef(t, found, "%s 의 metrics 설정이 없습니다", component)
		assert.Equalf(t, true, enabled, "%s 메트릭이 꺼져 있습니다", component)

		label, found := lookupValue(values, []string{component, "metrics", "serviceMonitor", "additionalLabels", "release"})
		require.Truef(t, found, "%s 의 ServiceMonitor 라벨이 없습니다", component)
		assert.Equal(t, "kube-prometheus-stack", label)
	}
}

// ServiceMonitor 는 Prometheus Operator 의 CRD 다. 오퍼레이터 없는 스택에
// 만들면 "no matches for kind ServiceMonitor" 로 설치가 통째로 멈춘다.
func TestServiceMonitorValues_OmittedWithoutPrometheus(t *testing.T) {
	for _, step := range []string{"installing_argocd", "installing_grafana", "installing_minio", "installing_logging"} {
		assert.Nilf(t, serviceMonitorValuesForStep(step, &domain.StackConfig{}), "step %s", step)
	}
}

// 로그 검색으로 OpenSearch 를 고른 스택에 Loki 용 키를 넣으면, 켜지지도 않은
// 것을 켰다고 오해하게 된다.
func TestServiceMonitorValues_LogSearchOnlyWhenLoki(t *testing.T) {
	cfg := monitoringEnabledConfig()
	cfg.Logging.Search = domain.ToolSelection{Name: "opensearch", Enabled: true}
	assert.Nil(t, serviceMonitorValuesForStep("installing_log_search", cfg))

	cfg.Logging.Search = domain.ToolSelection{Name: "loki", Enabled: true}
	assert.NotNil(t, serviceMonitorValuesForStep("installing_log_search", cfg))
}

// Jaeger 차트에는 이 키가 없다. 넣어도 무시되지만 켰다고 오해하게 된다.
func TestServiceMonitorValues_TraceLayerOnlyForTempo(t *testing.T) {
	cfg := monitoringEnabledConfig()
	cfg.Logging.TraceLayer = domain.ToolSelection{Name: "jaeger", Enabled: true}
	assert.Nil(t, serviceMonitorValuesForStep("installing_opentelemetry", cfg))

	cfg.Logging.TraceLayer = domain.ToolSelection{Name: "tempo", Enabled: true}
	assert.NotNil(t, serviceMonitorValuesForStep("installing_opentelemetry", cfg))
}

// 실제 설치 values 에 실려야 의미가 있다. 함수만 있고 배선이 빠지면
// 테스트는 통과하는데 클러스터에는 아무 일도 일어나지 않는다.
func TestValuesForStep_AppliesServiceMonitorToGrafana(t *testing.T) {
	o := NewOrchestrator(nil, nil, "nullus")
	o.SetStackConfig(*monitoringEnabledConfig())

	spec, ok := o.chartSpecForStep("installing_grafana")
	require.True(t, ok)
	values := o.valuesForStep("installing_grafana", spec)

	enabled, found := lookupValue(values, []string{"serviceMonitor", "enabled"})
	require.True(t, found, "ServiceMonitor 설정이 실제 values 에 실리지 않았습니다")
	assert.Equal(t, true, enabled)
}
