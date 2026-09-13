package usecase

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type countingBundleFactory struct {
	mu      sync.Mutex
	calls   map[string]int
	bundles map[string]*port.SCMBundle
}

func (f *countingBundleFactory) For(_ context.Context, stackID string) (*port.SCMBundle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[stackID]++
	if b, ok := f.bundles[stackID]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("%w: stack %s (state=%q)", port.ErrStackToolsUnavailable, stackID, "installing")
}

type staticSyncablePipelines []*domain.Pipeline

func (s staticSyncablePipelines) ListWithStack(context.Context) ([]*domain.Pipeline, error) {
	return s, nil
}

// 화면을 열지 않아도 실행 기록과 스캔 결과가 들어와야 한다. 번들 조립에는 SCM 인증
// 확인이 따르므로 스택마다 한 번만 만든다.
func TestSyncPipelineRuns_SyncAll_BuildsBundleOncePerStack(t *testing.T) {
	factory := &countingBundleFactory{calls: map[string]int{}, bundles: map[string]*port.SCMBundle{
		"stk_a": {CIBuilds: &stubBuildReader{builds: []port.CIBuild{{Number: 1, Result: "SUCCESS"}}}},
	}}
	deployments := newMemDeployments()
	uc := NewSyncPipelineRuns(nil, deployments).
		WithBundleFactory(factory, nil).
		WithSyncablePipelines(staticSyncablePipelines{
			{ID: "pip_1", Name: "app1", StackID: "stk_a"},
			{ID: "pip_2", Name: "app2", StackID: "stk_a"},
			// 설치 중인 스택은 건너뛴다 — 실패가 아니라 지금은 할 수 없는 일이다.
			{ID: "pip_3", Name: "app3", StackID: "stk_b"},
		})

	synced, err := uc.SyncAll(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, synced)
	assert.Equal(t, 1, factory.calls["stk_a"])
	assert.Equal(t, 1, factory.calls["stk_b"])
	for _, id := range []string{"dep_ci_pip_1_1", "dep_ci_pip_2_1"} {
		got, err := deployments.GetByID(context.Background(), id)
		require.NoError(t, err)
		assert.NotNil(t, got, id)
	}
}

func TestSyncPipelineRuns_SyncAll_NotWired(t *testing.T) {
	synced, err := NewSyncPipelineRuns(nil, newMemDeployments()).SyncAll(context.Background())
	require.NoError(t, err)
	assert.Zero(t, synced)
}
