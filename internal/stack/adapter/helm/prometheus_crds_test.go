package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// Prometheus 를 고른 스택은 각 도구의 ServiceMonitor 를 켠다
// (service-monitors.go). 그런데 그 리소스들이 쓰는 CRD 는 kube-prometheus-stack
// 이 가져오고, 그 설치는 순서상 한참 뒤다. 그래서 MinIO 에서 이렇게 죽었다:
//
//	install step installing_minio: no matches for kind "Probe" in version
//	"monitoring.coreos.com/v1" — ensure CRDs are installed first
//
// Prometheus 가 없는 템플릿에서는 ServiceMonitor 를 안 켜서 드러나지 않았다.
// CRD 를 앞에서 먼저 깐다.

func stepIndex(t *testing.T, steps []string, name string) int {
	t.Helper()
	for i, s := range steps {
		if s == name {
			return i
		}
	}
	t.Fatalf("%s 단계가 없다", name)
	return -1
}

func TestPrometheusCRDs_InstalledBeforeAnyServiceMonitorConsumer(t *testing.T) {
	steps := NewOrchestrator(nil, nil, "nullus").orderedStep

	crds := stepIndex(t, steps, stepInstallingPrometheusCRDs)
	// ServiceMonitor / Probe 를 만드는 단계들. 하나라도 CRD 보다 앞서면 설치가 깨진다.
	for _, consumer := range []string{"installing_minio", "installing_argocd", "installing_prometheus"} {
		assert.Lessf(t, crds, stepIndex(t, steps, consumer),
			"%s 가 CRD 설치보다 앞서면 그 단계에서 설치가 실패한다", consumer)
	}
}

// CRD 설치는 Prometheus 를 고른 스택에서만 필요하다.
func TestPrometheusCRDs_OnlyWhenMonitoringSelected(t *testing.T) {
	o := NewOrchestrator(nil, nil, "nullus")

	on := domain.StackConfig{}
	on.Monitoring.Collection = domain.ToolSelection{Name: "prometheus", Enabled: true}
	o.SetStackConfig(on)
	assert.True(t, o.IsStepEnabled(stepInstallingPrometheusCRDs))

	off := domain.StackConfig{}
	o.SetStackConfig(off)
	assert.False(t, o.IsStepEnabled(stepInstallingPrometheusCRDs),
		"모니터링을 안 쓰는 스택에 CRD 를 깔 이유가 없다")
}

func TestPrometheusCRDs_ChartMatchesOperatorVersion(t *testing.T) {
	spec, ok := NewOrchestrator(nil, nil, "nullus").chartSpecForStep(stepInstallingPrometheusCRDs)
	require.True(t, ok, "CRD 차트 정의가 없다")
	assert.Equal(t, "prometheus-operator-crds", spec.ChartName)
	assert.NotEmpty(t, spec.Version)
}

// kube-prometheus-stack 도 같은 CRD 를 만든다. 그대로 두면 이미 있는 리소스의
// 소유권 충돌로 설치가 실패한다.
func TestPrometheus_DoesNotReinstallCRDs(t *testing.T) {
	values := DefaultValues("installing_prometheus")
	crds, ok := values["crds"].(map[string]any)
	require.True(t, ok, "crds 설정이 없으면 kube-prometheus-stack 이 CRD 를 또 만든다")
	assert.Equal(t, false, crds["enabled"])
}
