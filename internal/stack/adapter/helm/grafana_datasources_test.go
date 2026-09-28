package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치는 Prometheus·Loki·Tempo 를 세우고 OTel Collector 가 셋으로 내보내도록
// 배선하는데, 정작 그것들을 보는 Grafana 에는 데이터소스가 한 줄도 들어가지
// 않았다. 스택은 completed 이고 파드도 전부 Running 인데 Grafana 를 열면 비어 있다.
func TestGrafanaDatasourceValues_ObservabilityStack(t *testing.T) {
	cfg := &domain.StackConfig{}
	cfg.Monitoring.Collection = domain.ToolSelection{Name: "Prometheus", Enabled: true}
	cfg.Logging.Search = domain.ToolSelection{Name: "Loki", Enabled: true}
	cfg.Logging.TraceLayer = domain.ToolSelection{Name: "Tempo", Enabled: true}

	got := datasourceNamesAndURLs(t, grafanaDatasourceValues(cfg))
	assert.Equal(t, map[string]string{
		"Prometheus": "http://kube-prometheus-stack-prometheus:9090",
		"Loki":       "http://loki:3100",
		"Tempo":      "http://tempo:3100",
	}, got)
}

// 고르지 않은 도구를 걸면 Grafana 가 시작부터 연결 오류를 띄운다.
func TestGrafanaDatasourceValues_OnlyInstalledTools(t *testing.T) {
	cfg := &domain.StackConfig{}
	cfg.Monitoring.Collection = domain.ToolSelection{Name: "Prometheus", Enabled: true}
	cfg.Logging.Search = domain.ToolSelection{Name: "Loki", Enabled: false}
	cfg.Logging.TraceLayer = domain.ToolSelection{Name: "Tempo", Enabled: false}

	got := datasourceNamesAndURLs(t, grafanaDatasourceValues(cfg))
	assert.Equal(t, map[string]string{"Prometheus": "http://kube-prometheus-stack-prometheus:9090"}, got)

	// 아무것도 고르지 않았으면 values 를 내지 않는다.
	assert.Nil(t, grafanaDatasourceValues(&domain.StackConfig{}))
	assert.Nil(t, grafanaDatasourceValues(nil))
}

// 검색·추적 계층은 고른 도구에 따라 타입과 주소가 다르다.
func TestGrafanaDatasourceValues_AlternativeBackends(t *testing.T) {
	cfg := &domain.StackConfig{}
	cfg.Logging.Search = domain.ToolSelection{Name: "OpenSearch", Enabled: true}
	cfg.Logging.TraceLayer = domain.ToolSelection{Name: "Jaeger", Enabled: true}

	got := datasourceNamesAndURLs(t, grafanaDatasourceValues(cfg))
	assert.Equal(t, map[string]string{
		"OpenSearch": "http://opensearch-cluster-master:9200",
		"Jaeger":     "http://jaeger-query:16686",
	}, got)
}

// 설치 경로까지 실려 가는지 본다.
func TestMergedValuesForStep_GrafanaCarriesDatasources(t *testing.T) {
	spec, ok := DefaultChartSpecForStep("installing_grafana")
	require.True(t, ok)

	o := &Orchestrator{namespace: "nullus"}
	o.stackConfig = &domain.StackConfig{}
	o.stackConfig.Monitoring.Collection = domain.ToolSelection{Name: "Prometheus", Enabled: true}
	o.stackConfig.Logging.TraceLayer = domain.ToolSelection{Name: "Tempo", Enabled: true}

	got := datasourceNamesAndURLs(t, o.mergedValuesForStep("installing_grafana", spec))
	assert.Contains(t, got, "Prometheus")
	assert.Contains(t, got, "Tempo")
}

func datasourceNamesAndURLs(t *testing.T, values map[string]any) map[string]string {
	t.Helper()
	if values == nil {
		return nil
	}
	ds, ok := values["datasources"].(map[string]any)
	require.True(t, ok, "datasources 블록이 있어야 한다")
	file, ok := ds["datasources.yaml"].(map[string]any)
	require.True(t, ok, "datasources.yaml 이 있어야 한다")
	list, ok := file["datasources"].([]any)
	require.True(t, ok, "datasources 목록이 있어야 한다")

	out := map[string]string{}
	for _, item := range list {
		entry := item.(map[string]any)
		out[entry["name"].(string)] = entry["url"].(string)
		assert.Equal(t, "proxy", entry["access"], "Grafana 가 백엔드에 직접 붙어야 한다")
	}
	return out
}
