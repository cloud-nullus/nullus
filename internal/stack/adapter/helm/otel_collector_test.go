package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// pipelineExporters 는 렌더된 values 에서 파이프라인 하나의 exporter 목록을 꺼낸다.
func pipelineExporters(t *testing.T, values map[string]any, pipeline string) []any {
	t.Helper()

	config, ok := values["config"].(map[string]any)
	require.True(t, ok, "config 블록이 없습니다")
	service, ok := config["service"].(map[string]any)
	require.True(t, ok, "service 블록이 없습니다")
	pipelines, ok := service["pipelines"].(map[string]any)
	require.True(t, ok, "pipelines 블록이 없습니다")
	target, ok := pipelines[pipeline].(map[string]any)
	require.Truef(t, ok, "%s 파이프라인이 없습니다", pipeline)
	exporters, ok := target["exporters"].([]any)
	require.Truef(t, ok, "%s 파이프라인에 exporter 가 없습니다", pipeline)
	return exporters
}

func exporterConfig(t *testing.T, values map[string]any, name string) (map[string]any, bool) {
	t.Helper()

	config, ok := values["config"].(map[string]any)
	require.True(t, ok)
	exporters, ok := config["exporters"].(map[string]any)
	require.True(t, ok)
	raw, ok := exporters[name]
	if !ok {
		return nil, false
	}
	out, ok := raw.(map[string]any)
	require.Truef(t, ok, "exporter %s 가 매핑이 아닙니다", name)
	return out, true
}

// 수집기를 설치했는데 추적이 어디로도 가지 않으면 관측 스택이 아니다.
func TestOTelCollectorValues_SendsTracesToTempo(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			TraceLayer:    domain.ToolSelection{Name: "tempo", Enabled: true},
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	assert.Equal(t, []any{"otlp/tempo"}, pipelineExporters(t, values, "traces"))

	exporter, ok := exporterConfig(t, values, "otlp/tempo")
	require.True(t, ok, "tempo 로 보내는 exporter 가 없습니다")
	assert.Equal(t, "tempo:4317", exporter["endpoint"])
}

func TestOTelCollectorValues_SendsTracesToJaeger(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			TraceLayer:    domain.ToolSelection{Name: "jaeger", Enabled: true},
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	assert.Equal(t, []any{"otlp/jaeger"}, pipelineExporters(t, values, "traces"))

	exporter, ok := exporterConfig(t, values, "otlp/jaeger")
	require.True(t, ok)
	assert.Equal(t, "jaeger-collector:4317", exporter["endpoint"])
}

// 추적 저장소를 고르지 않았으면 없는 주소로 보내려 하면 안 된다.
// debug 는 차트 기본 config 에 이미 있으므로 따로 선언하지 않는다.
func TestOTelCollectorValues_FallsBackToDebugWithoutTraceLayer(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	assert.Equal(t, []any{"debug"}, pipelineExporters(t, values, "traces"))

	_, ok := exporterConfig(t, values, "otlp/tempo")
	assert.False(t, ok, "고르지 않은 백엔드로 보내는 exporter 를 만들면 안 됩니다")
}

func TestOTelCollectorValues_SendsLogsToLoki(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			Collection:    domain.ToolSelection{Name: "loki", Enabled: true},
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	assert.Equal(t, []any{"loki"}, pipelineExporters(t, values, "logs"))

	exporter, ok := exporterConfig(t, values, "loki")
	require.True(t, ok)
	assert.Equal(t, "http://loki:3100/loki/api/v1/push", exporter["endpoint"])
}

// 설치 마법사는 Loki 를 로그 검색 칸에서 고르게 한다. 그 경로로 고른 Loki 도
// 수집기의 로그 목적지여야 한다.
func TestOTelCollectorValues_SendsLogsToLokiChosenAsSearchStore(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			Search:        domain.ToolSelection{Name: "loki", Enabled: true},
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	assert.Equal(t, []any{"loki"}, pipelineExporters(t, values, "logs"))
}

// Loki 는 라벨로만 스트림을 가른다. 힌트가 없으면 모든 로그가 한 덩어리로
// 들어가 네임스페이스로 좁힐 수 없다.
func TestOTelCollectorValues_PromotesKubernetesLabelsForLoki(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			Search:        domain.ToolSelection{Name: "loki", Enabled: true},
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	config, ok := values["config"].(map[string]any)
	require.True(t, ok)
	processors, ok := config["processors"].(map[string]any)
	require.True(t, ok)
	resource, ok := processors["resource/loki"].(map[string]any)
	require.True(t, ok, "라벨 승격 프로세서가 없습니다")

	attrs, ok := resource["attributes"].([]any)
	require.True(t, ok)
	first, ok := attrs[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "loki.resource.labels", first["key"])
	// 파드 이름은 넣지 않는다 — 재생성마다 새 스트림이 생겨 카디널리티가 터진다.
	assert.Equal(t, "k8s.namespace.name, k8s.container.name", first["value"])

	service, ok := config["service"].(map[string]any)
	require.True(t, ok)
	pipelines, ok := service["pipelines"].(map[string]any)
	require.True(t, ok)
	logs, ok := pipelines["logs"].(map[string]any)
	require.True(t, ok)
	// 프로세서는 batch 앞에 서야 한다 — 묶인 뒤에 붙이면 배치 단위로만 적용된다.
	assert.Equal(t, []any{"memory_limiter", "resource/loki", "batch"}, logs["processors"])
}

// Loki 가 아니면 승격할 라벨도 없다. 쓰지 않을 프로세서를 선언하면 안 된다.
func TestOTelCollectorValues_OmitsLokiLabelProcessorWithoutLoki(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	config, ok := values["config"].(map[string]any)
	require.True(t, ok)
	processors, ok := config["processors"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, processors)
}

func TestOTelCollectorValues_FallsBackToDebugWhenLogStoreIsNotLoki(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			Collection:    domain.ToolSelection{Name: "fluentd", Enabled: true},
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	assert.Equal(t, []any{"debug"}, pipelineExporters(t, values, "logs"))
}

// ServiceMonitor 는 Prometheus Operator 의 CRD 다. 오퍼레이터가 없는 스택에
// 만들면 설치가 "no matches for kind ServiceMonitor" 로 통째로 실패한다.
func TestOTelCollectorValues_OmitsServiceMonitorWithoutPrometheus(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Logging: domain.LoggingConfig{
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	_, ok := values["serviceMonitor"]
	assert.False(t, ok)
}

func TestOTelCollectorValues_ScrapesItselfWhenPrometheusInstalled(t *testing.T) {
	values := otelCollectorValues(&domain.StackConfig{
		Monitoring: domain.MonitoringConfig{
			Collection: domain.ToolSelection{Name: "prometheus", Enabled: true},
		},
		Logging: domain.LoggingConfig{
			TraceExporter: domain.ToolSelection{Name: "opentelemetry-collector", Enabled: true},
		},
	})

	monitor, ok := values["serviceMonitor"].(map[string]any)
	require.True(t, ok, "Prometheus 가 있으면 수집기 자체 메트릭을 긁어야 합니다")
	assert.Equal(t, true, monitor["enabled"])

	// kube-prometheus-stack 은 자기 release 라벨이 붙은 모니터만 고른다.
	labels, ok := monitor["extraLabels"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "kube-prometheus-stack", labels["release"])
}

// 화면이 Loki 를 고를 수 있게 열어 두므로 고른 대로 깔려야 한다.
// 분기가 없어 default(OpenSearch)로 떨어지면 사용자는 고르지 않은 제품을 받는다.
func TestResolveChartSpec_InstallsLokiWhenChosenAsLogSearch(t *testing.T) {
	o := NewOrchestrator(nil, nil, "nullus")
	o.SetStackConfig(domain.StackConfig{
		Logging: domain.LoggingConfig{
			Search: domain.ToolSelection{Name: "loki", Enabled: true},
		},
	})

	spec, ok := o.chartSpecForStep("installing_log_search")
	require.True(t, ok)
	spec = o.resolveChartSpecForStep("installing_log_search", spec)

	assert.Equal(t, "loki", spec.ChartName)
	assert.Equal(t, "https://grafana.github.io/helm-charts", spec.RepoURL)
}

// Loki 는 저장소일 뿐 파드 로그를 스스로 긁어오지 않는다. 노드마다 서는
// 에이전트가 없으면 "로그 저장소는 있는데 로그가 안 들어오는" 상태가 된다.
func TestOTelAgentValues_ShipsNodeLogsToGateway(t *testing.T) {
	values := otelAgentValues("nullus-otelstack")

	assert.Equal(t, "daemonset", values["mode"], "로그 파일은 노드마다 있으므로 모든 노드에 떠야 한다")

	presets, ok := values["presets"].(map[string]any)
	require.True(t, ok)
	logsCollection, ok := presets["logsCollection"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, logsCollection["enabled"], "filelog 수신기가 없으면 읽을 것이 없다")

	// 파드/네임스페이스를 붙여야 로그가 어느 워크로드 것인지 알 수 있다.
	k8sAttrs, ok := presets["kubernetesAttributes"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, k8sAttrs["enabled"])

	// 내보내기는 게이트웨이 한 곳으로 모은다 — 저장소로 나가는 출구는 하나다.
	assert.Equal(t, []any{"otlp/gateway"}, pipelineExporters(t, values, "logs"))

	exporter, ok := exporterConfig(t, values, "otlp/gateway")
	require.True(t, ok)
	assert.Equal(t,
		"otel-collector-opentelemetry-collector.nullus-otelstack.svc.cluster.local:4317",
		exporter["endpoint"])
}

// 수신기를 명시하지 않으면 차트 기본 파이프라인이 지워진 otlp 를 계속 참조해
// 수집기가 기동하다 죽는다.
func TestOTelAgentValues_DeclaresFilelogReceiverExplicitly(t *testing.T) {
	values := otelAgentValues("nullus-otelstack")

	config, ok := values["config"].(map[string]any)
	require.True(t, ok)
	service, ok := config["service"].(map[string]any)
	require.True(t, ok)
	pipelines, ok := service["pipelines"].(map[string]any)
	require.True(t, ok)
	logs, ok := pipelines["logs"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, []any{"filelog"}, logs["receivers"])
}

// 에이전트는 받는 쪽이 아니다. 포트를 열어 두면 DaemonSet 이라 노드 포트를
// 물고 앉아 다른 것과 충돌할 여지만 남는다.
func TestOTelAgentValues_DoesNotExposeIngestPorts(t *testing.T) {
	values := otelAgentValues("nullus-otelstack")

	ports, ok := values["ports"].(map[string]any)
	require.True(t, ok)
	for _, name := range []string{"otlp", "otlp-http", "jaeger-grpc", "zipkin"} {
		port, ok := ports[name].(map[string]any)
		require.Truef(t, ok, "포트 %s 설정이 없습니다", name)
		assert.Equalf(t, false, port["enabled"], "에이전트는 %s 로 받지 않는다", name)
	}
}

// 수집기 릴리스명이 곧 파드 이름 접두사이므로 도메인 상수와 갈라지면
// 모니터링이 파드를 못 찾는다.
func TestOTelCollectorChartSpec_UsesDedicatedReleaseName(t *testing.T) {
	spec, ok := DefaultChartSpecForStep("installing_otel_collector")
	require.True(t, ok, "수집기 단계에 차트 스펙이 없습니다")

	assert.Equal(t, domain.OTelCollectorReleaseName, spec.ReleaseName)
	assert.Equal(t, "opentelemetry-collector", spec.ChartName)
	assert.Equal(t, domain.OTelCollectorChartVersion, spec.Version)

	// 추적 계층 단계와 릴리스명이 겹치면 Helm 이 소유권 충돌로 설치를 거부한다.
	traceLayer, ok := DefaultChartSpecForStep("installing_opentelemetry")
	require.True(t, ok)
	assert.NotEqual(t, spec.ReleaseName, traceLayer.ChartName)
}
