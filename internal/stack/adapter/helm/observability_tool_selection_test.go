package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 템플릿과 카탈로그는 도구 이름을 사람이 읽는 대로 담는다("Loki", "Tempo").
// 차트를 고르는 쪽이 그 표기를 그대로 소문자 리터럴과 비교해, 고른 도구가
// 아니라 default 가 깔렸다:
//
//   - Loki 를 골랐는데 OpenSearch 가 깔렸다
//   - Tempo 를 골랐는데 OTel Collector 차트에 Tempo values 가 실려
//     스키마 검증에서 설치가 통째로 실패했다(gitlab-argocd-otel-v1 이 설치 불가)
//
// 시드가 담는 표기 그대로 고정한다.
func TestResolveChartSpecForStep_UsesCatalogToolNames(t *testing.T) {
	for _, tc := range []struct {
		name      string
		search    string
		trace     string
		wantLog   string
		wantTrace string
	}{
		{name: "시드 표기", search: "Loki", trace: "Tempo", wantLog: "loki", wantTrace: "tempo"},
		{name: "소문자", search: "loki", trace: "tempo", wantLog: "loki", wantTrace: "tempo"},
		{name: "공백 포함", search: " Loki ", trace: " Tempo ", wantLog: "loki", wantTrace: "tempo"},
		{name: "OpenSearch", search: "OpenSearch", trace: "Jaeger", wantLog: "opensearch", wantTrace: "jaeger"},
		{name: "Elasticsearch", search: "Elasticsearch", trace: "", wantLog: "elasticsearch", wantTrace: "opentelemetry-collector"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := &Orchestrator{namespace: "nullus"}
			o.stackConfig = &domain.StackConfig{}
			o.stackConfig.Logging.Search.Name = tc.search
			o.stackConfig.Logging.TraceLayer.Name = tc.trace

			logSpec, ok := DefaultChartSpecForStep("installing_log_search")
			require.True(t, ok)
			assert.Equal(t, tc.wantLog, o.resolveChartSpecForStep("installing_log_search", logSpec).ChartName)

			traceSpec, ok := DefaultChartSpecForStep("installing_opentelemetry")
			require.True(t, ok)
			assert.Equal(t, tc.wantTrace, o.resolveChartSpecForStep("installing_opentelemetry", traceSpec).ChartName)
		})
	}
}

// 차트를 고르는 쪽과 자원 기본값을 고르는 쪽이 같은 도구를 봐야 한다.
// 둘이 갈라지면 한쪽 도구의 values 가 다른 도구의 차트에 실린다 — 그것이
// gitlab-argocd-otel-v1 을 설치 불가로 만든 실패다.
func TestChartSelectionAndResourceDefaultsAgreeOnTool(t *testing.T) {
	for _, tc := range []struct {
		search, trace string
	}{
		{search: "Loki", trace: "Tempo"},
		{search: "OpenSearch", trace: "Jaeger"},
		{search: "Elasticsearch", trace: "Tempo"},
	} {
		o := &Orchestrator{namespace: "nullus"}
		o.stackConfig = &domain.StackConfig{}
		o.stackConfig.Logging.Search.Name = tc.search
		o.stackConfig.Logging.TraceLayer.Name = tc.trace

		spec, ok := DefaultChartSpecForStep("installing_opentelemetry")
		require.True(t, ok)
		chart := o.resolveChartSpecForStep("installing_opentelemetry", spec).ChartName
		key := o.resourceDefaultKeyForStep("installing_opentelemetry", o.stackConfig)
		assert.Equal(t, key, chart,
			"trace=%q: 자원 기본값은 %q 를 보는데 차트는 %q 를 깐다", tc.trace, key, chart)

		logSpec, ok := DefaultChartSpecForStep("installing_log_search")
		require.True(t, ok)
		logChart := o.resolveChartSpecForStep("installing_log_search", logSpec).ChartName
		logKey := o.resourceDefaultKeyForStep("installing_log_search", o.stackConfig)
		assert.Equal(t, logKey, logChart,
			"search=%q: 자원 기본값은 %q 를 보는데 차트는 %q 를 깐다", tc.search, logKey, logChart)
	}
}

// 로그 수집과 검색이 같은 도구면 검색 단계는 돌지 않는다 — 둘 다 같은 loki
// 릴리스를 설치하므로 뒤엣것이 앞엣것을 덮는다.
//
// 그 판단이 TrimSpace 로만 비교하면, 차트를 고르는 쪽이 정규화해서 같은 도구로
// 보는 표기("loki"/"Loki")를 여기서는 다른 도구로 보고 두 단계를 모두 돌린다.
func TestIsStepEnabled_LogSearchSkipsSameToolRegardlessOfCasing(t *testing.T) {
	for _, tc := range []struct {
		name       string
		collection string
		search     string
		want       bool
	}{
		{name: "표기만 다른 같은 도구", collection: "loki", search: "Loki", want: false},
		{name: "반대 표기", collection: "Loki", search: "loki", want: false},
		{name: "같은 표기", collection: "Loki", search: "Loki", want: false},
		{name: "다른 도구는 따로 깐다", collection: "Loki", search: "OpenSearch", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewOrchestrator(&mockInstaller{}, []byte("not-a-kubeconfig"), "nullus")
			cfg := &domain.StackConfig{}
			cfg.Logging.Collection = domain.ToolSelection{Name: tc.collection, Enabled: true}
			cfg.Logging.Search = domain.ToolSelection{Name: tc.search, Enabled: true}
			o.stackConfig = cfg

			assert.Equal(t, tc.want, o.isStepEnabled("installing_log_search"))
		})
	}
}
