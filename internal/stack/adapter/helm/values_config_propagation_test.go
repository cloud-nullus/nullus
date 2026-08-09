package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// Harbor 의 externalURL 은 레지스트리가 광고하는 토큰 realm 이 된다.
// 클러스터 내부 DNS 이름이 남으면 노드의 containerd 가 그것을 해석하지 못해
// 배포된 앱이 ImagePullBackOff 로 멈춘다 — GitOps CD 가 끝까지 가지 못한다.
//
// YAML override 가 없는 설치(일반적인 경로)에서도 접근 도메인이 반영되어야 한다.
func TestValuesForStep_HarborExternalURLUsesAccessDomain_WithoutYAMLOverrides(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.SetStackConfig(domain.StackConfig{AccessDomain: "nullus-devsecops-stack.internal"})

	spec, ok := defaultChartSpecForStep("installing_harbor")
	require.True(t, ok)

	values := o.valuesForStep("installing_harbor", spec)

	assert.Equal(t, "http://harbor.nullus-devsecops-stack.internal", values["externalURL"])
}

// YAML override 가 있는 경로는 이미 cfg 를 넘기고 있었다 — 회귀 방지로 함께 고정한다.
func TestValuesForStep_HarborExternalURLUsesAccessDomain_WithYAMLOverrides(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.SetStackConfig(domain.StackConfig{
		AccessDomain:  "nullus-devsecops-stack.internal",
		YAMLOverrides: map[string]string{"installing_harbor": "trivy:\n  enabled: false\n"},
	})

	spec, ok := defaultChartSpecForStep("installing_harbor")
	require.True(t, ok)

	values := o.valuesForStep("installing_harbor", spec)

	assert.Equal(t, "http://harbor.nullus-devsecops-stack.internal", values["externalURL"])
}

// 접근 도메인이 없으면 클러스터 내부 주소로 되돌아간다 — 기존 동작 유지.
func TestValuesForStep_HarborExternalURLFallsBackToServiceDNS(t *testing.T) {
	o := &Orchestrator{namespace: "devsecops"}

	spec, ok := defaultChartSpecForStep("installing_harbor")
	require.True(t, ok)

	values := o.valuesForStep("installing_harbor", spec)

	assert.Equal(t, "http://harbor.devsecops.svc.cluster.local", values["externalURL"])
}

// 같은 결함이 PostgreSQL 용량에도 있었다 — YAML override 가 없으면 사용자가 고른
// 디스크 크기가 조용히 무시되고 기본값 20Gi 로 깔린다. 스토리지는 설치 후
// 늘리기 번거로우므로 사용자가 지정한 값이 그대로 가야 한다.
func TestValuesForStep_PostgresUsesConfiguredSize_WithoutYAMLOverrides(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	o.SetStackConfig(domain.StackConfig{
		Storage: &domain.StorageConfig{
			Database: domain.StorageTarget{Mode: "create", Size: 10},
		},
	})

	spec, ok := defaultChartSpecForStep("installing_postgresql")
	require.True(t, ok)

	values := o.valuesForStep("installing_postgresql", spec)

	primary, ok := values["primary"].(map[string]any)
	require.True(t, ok)
	persistence, ok := primary["persistence"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "10Gi", persistence["size"])
}
