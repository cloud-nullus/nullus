package helm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 게이트웨이가 선 뒤 러너를 다시 적용하는 경로는 이미 지나간 단계를 부른다.
// 그때 stackID 를 함께 넘기면 순서 검사가 거부하고, 그 오류가 게이트웨이 단계를
// 실패로 뒤집어 설치 전체가 멈춘다 — 이름 해석을 넣으려던 것이 설치를 깨뜨린다.
func TestReapplyStep_DoesNotTripStepOrdering(t *testing.T) {
	installer := &mockInstaller{}
	o := NewOrchestrator(installer, []byte("not-a-kubeconfig"), "nullus")

	gatewayOrder := o.stepOrder["installing_gateway"]
	runnerOrder := o.stepOrder[stepInstallingRunner]
	require.Less(t, runnerOrder, gatewayOrder, "러너는 게이트웨이보다 앞 단계여야 이 회귀가 성립한다")

	// 게이트웨이 단계가 도는 중의 진행도 — ensureOrder 가 진입 시 세우는 값이다.
	o.progress["stk_reapply"] = gatewayOrder - 1

	// 회귀 고정: 지나간 단계를 stackID 와 함께 부르면 거부된다.
	err := o.ExecuteStep(context.Background(), "stk_reapply", stepInstallingRunner, "C")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of order step")

	// 재적용 경로는 장부를 건드리지 않으므로 거부되지 않고, 실제로 다시 설치한다.
	require.NoError(t, o.reapplyStep(context.Background(), stepInstallingRunner, "C"))
	assert.Contains(t, installer.installed, "gitlab-runner")

	// 게이트웨이 단계의 진행도가 뒤로 밀리지 않는다 — 밀리면 그 뒤 단계들이
	// 지나간 단계를 다시 요구하며 순서 검사에서 멈춘다.
	assert.Equal(t, gatewayOrder-1, o.progress["stk_reapply"])
}

// 게이트웨이 주소를 읽지 못하면 설치를 실패로 뒤집지 않는다.
func TestReconcileGatewayHostAliases_NoAccessDomainIsNoop(t *testing.T) {
	installer := &mockInstaller{}
	o := NewOrchestrator(installer, []byte("not-a-kubeconfig"), "nullus")
	o.stackConfig = &domain.StackConfig{}

	require.NoError(t, o.reconcileGatewayHostAliases(context.Background(), "stk_noop", "nullus", "C"))
	assert.Empty(t, installer.installed, "접속 도메인이 없으면 아무것도 다시 적용하지 않는다")
}
