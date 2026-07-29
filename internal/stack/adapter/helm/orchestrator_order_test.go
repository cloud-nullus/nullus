package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// canonicalInstallOrder 는 설치가 실행해야 하는 정식 순서다.
// usecase 패키지의 installDAG 와 반드시 일치해야 한다 — ensureOrder 가
// orderedStep 순서를 강제하므로, 둘이 어긋나면 설치가 멈춘다.
// (usecase 쪽 대칭 테스트: TestInstallDAG_MatchesCanonicalOrder)
var canonicalInstallOrder = []string{
	"installing_cert_manager",
	"installing_metrics_server",
	"installing_openbao",
	"installing_external_secrets",
	"provisioning_secrets",
	"installing_postgresql",
	"installing_minio",
	"installing_object_storage_secret",
	"installing_object_storage_buckets",
	"installing_database_connection_check",
	"installing_gitlab",
	"installing_argocd",
	"installing_runner",
	"installing_prometheus",
	"installing_grafana",
	"installing_logging",
	"installing_log_search",
	"installing_opentelemetry",
	"installing_gateway",
	"integration_check",
}

func newOrderTestOrchestrator() *Orchestrator {
	return NewOrchestrator(nil, nil, "nullus")
}

func TestOrderedStep_MatchesCanonicalInstallOrder(t *testing.T) {
	o := newOrderTestOrchestrator()
	assert.Equal(t, canonicalInstallOrder, o.orderedStep)
}

// stepOrder 는 orderedStep 의 인덱스와 정확히 일치해야 한다.
// ensureOrder 가 두 자료구조를 함께 쓰므로 어긋나면 순서 검증이 깨진다.
func TestStepOrder_MatchesOrderedStepIndexes(t *testing.T) {
	o := newOrderTestOrchestrator()

	require.Len(t, o.stepOrder, len(o.orderedStep))
	for idx, step := range o.orderedStep {
		order, ok := o.stepOrder[step]
		require.Truef(t, ok, "step %s missing from stepOrder", step)
		assert.Equalf(t, idx, order, "stepOrder[%s] must match orderedStep index", step)
	}
}

// 시크릿 평면이 스토리지 차트보다 먼저 와야 한다.
func TestOrderedStep_SecretProvisioningPrecedesStorage(t *testing.T) {
	o := newOrderTestOrchestrator()

	provisioning := o.stepOrder["provisioning_secrets"]
	for _, consumer := range []string{"installing_postgresql", "installing_minio"} {
		assert.Lessf(t, provisioning, o.stepOrder[consumer],
			"provisioning_secrets must precede %s", consumer)
	}
}
