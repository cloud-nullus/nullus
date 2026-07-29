package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치가 만들어 내는 릴리스 이름은 전부 domain.InstalledHelmReleaseNames 에
// 등록되어 있어야 한다. 등록되지 않은 릴리스는 DeleteStack 이 uninstall 하지
// 않아 cluster-scoped 리소스가 고아로 남고, 다음 설치를 Helm ownership 충돌로
// 막는다. 차트를 새로 추가하면 이 테스트가 먼저 실패한다.
func TestChartSpecReleaseNames_AreRegisteredForDeletion(t *testing.T) {
	registered := map[string]bool{}
	for _, name := range domain.InstalledHelmReleaseNames {
		registered[name] = true
	}

	o := NewOrchestrator(nil, nil, "nullus")

	for _, step := range o.orderedStep {
		spec, ok := o.chartSpecForStep(step)
		if !ok {
			continue // 차트가 없는 단계 (provisioning_secrets 등)
		}
		name := o.releaseNameForSpec(spec)
		require.NotEmptyf(t, name, "step %s 의 릴리스 이름이 비어 있다", step)
		assert.Truef(t, registered[name],
			"step %s 의 릴리스 %q 가 domain.InstalledHelmReleaseNames 에 없다 — 삭제되지 않는다", step, name)
	}
}

// 로깅/트레이스 단계는 설정에 따라 차트가 갈린다. 변형도 전부 등록되어야 한다.
func TestResolvedChartVariants_AreRegisteredForDeletion(t *testing.T) {
	registered := map[string]bool{}
	for _, name := range domain.InstalledHelmReleaseNames {
		registered[name] = true
	}

	cases := []struct {
		step   string
		config domain.StackConfig
	}{
		{step: "installing_log_search", config: domain.StackConfig{
			Logging: domain.LoggingConfig{Search: domain.ToolSelection{Name: "opensearch"}}}},
		{step: "installing_log_search", config: domain.StackConfig{
			Logging: domain.LoggingConfig{Search: domain.ToolSelection{Name: "elasticsearch"}}}},
		{step: "installing_opentelemetry", config: domain.StackConfig{
			Logging: domain.LoggingConfig{TraceLayer: domain.ToolSelection{Name: "tempo"}}}},
		{step: "installing_opentelemetry", config: domain.StackConfig{
			Logging: domain.LoggingConfig{TraceLayer: domain.ToolSelection{Name: "jaeger"}}}},
		{step: "installing_opentelemetry", config: domain.StackConfig{
			Logging: domain.LoggingConfig{TraceLayer: domain.ToolSelection{Name: ""}}}},
	}

	for _, tc := range cases {
		o := NewOrchestrator(nil, nil, "nullus")
		o.SetStackConfig(tc.config)

		spec, ok := o.chartSpecForStep(tc.step)
		require.Truef(t, ok, "step %s 의 chart spec 이 없다", tc.step)
		spec = o.resolveChartSpecForStep(tc.step, spec)

		name := o.releaseNameForSpec(spec)
		assert.Truef(t, registered[name],
			"step %s 의 변형 릴리스 %q 가 삭제 대상에 없다", tc.step, name)
	}
}
