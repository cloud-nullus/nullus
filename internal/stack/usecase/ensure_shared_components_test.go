package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// sharedStepExecutor 는 설치기 대역이다. 어떤 단계를 어떤 스택 ID 로 돌렸는지 남긴다.
type sharedStepExecutor struct {
	calls     []string
	stackIDs  []string
	namespace string
	fail      map[string]error
}

func (e *sharedStepExecutor) ExecuteStep(_ context.Context, stackID, step, _ string) error {
	e.calls = append(e.calls, step)
	e.stackIDs = append(e.stackIDs, stackID)
	if err, ok := e.fail[step]; ok {
		return err
	}
	return nil
}

func (e *sharedStepExecutor) SetNamespace(ns string) { e.namespace = ns }

var _ port.StepExecutor = (*sharedStepExecutor)(nil)

// 클러스터의 마지막 스택을 지우면 cert-manager·metrics-server 가 회수된다. 복구는 설치기의
// 같은 단계를 스택 없이 다시 돌려 컨트롤러를 세운다 — 있으면 재사용, 없으면 설치.
func TestEnsureSharedComponents_설치기의_공용_단계를_스택_없이_돌린다(t *testing.T) {
	exec := &sharedStepExecutor{}
	uc := NewEnsureSharedComponents(func([]byte) port.StepExecutor { return exec })

	steps, warnings, err := uc.EnsureSharedComponents(context.Background(), []byte("apiVersion: v1\nkind: Config\n"), "nullus-app")
	require.NoError(t, err)

	assert.Equal(t, []string{"installing_cert_manager", "installing_metrics_server"}, exec.calls,
		"cert-manager 가 먼저다 — metrics-server 는 그것에 기대지 않지만 설치 순서와 같게 둔다")
	assert.Equal(t, []string{"", ""}, exec.stackIDs, "스택 ID 가 비어야 순서 검사를 타지 않는다")
	assert.Equal(t, "nullus-app", exec.namespace, "설치기가 스택 네임스페이스를 안다(cert-manager 후보 네임스페이스)")
	assert.Equal(t, []string{"installing_cert_manager", "installing_metrics_server"}, steps)
	assert.Empty(t, warnings)
}

// cert-manager 없이는 ClusterIssuer·Certificate 를 되살릴 수 없다 — 실패는 오류다.
func TestEnsureSharedComponents_cert_manager_실패는_오류다(t *testing.T) {
	exec := &sharedStepExecutor{fail: map[string]error{"installing_cert_manager": errors.New("chart pull failed")}}
	uc := NewEnsureSharedComponents(func([]byte) port.StepExecutor { return exec })

	_, _, err := uc.EnsureSharedComponents(context.Background(), []byte("kubeconfig"), "nullus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cert-manager")
	assert.Equal(t, []string{"installing_cert_manager"}, exec.calls, "필수 단계가 죽으면 다음으로 가지 않는다")
}

// metrics-server 는 HPA·자원 지표에만 쓰인다. 못 세워도 복구를 멈추지 않고 알린다.
func TestEnsureSharedComponents_metrics_server_실패는_경고다(t *testing.T) {
	exec := &sharedStepExecutor{fail: map[string]error{"installing_metrics_server": errors.New("timeout")}}
	uc := NewEnsureSharedComponents(func([]byte) port.StepExecutor { return exec })

	steps, warnings, err := uc.EnsureSharedComponents(context.Background(), []byte("kubeconfig"), "nullus")
	require.NoError(t, err)
	assert.Equal(t, []string{"installing_cert_manager"}, steps)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "metrics-server")
}

func TestEnsureSharedComponents_입력_검증(t *testing.T) {
	uc := NewEnsureSharedComponents(func([]byte) port.StepExecutor { return &sharedStepExecutor{} })
	_, _, err := uc.EnsureSharedComponents(context.Background(), nil, "nullus")
	require.Error(t, err, "kubeconfig 없이는 어느 클러스터인지 모른다")

	none := NewEnsureSharedComponents(nil)
	_, _, err = none.EnsureSharedComponents(context.Background(), []byte("kubeconfig"), "nullus")
	require.Error(t, err, "설치기 팩토리가 없으면 할 수 있는 것이 없다")
}
