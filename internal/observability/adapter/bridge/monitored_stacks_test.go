package bridge

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
)

type fakeCompleted struct{ stacks []*stackdomain.Stack }

func (f fakeCompleted) ListCompleted(context.Context) ([]*stackdomain.Stack, error) {
	return f.stacks, nil
}

func cfg(collection bool) map[string]any {
	return map[string]any{"monitoring": map[string]any{"collection": map[string]any{"name": "Prometheus", "enabled": collection}}}
}

// 모니터링 수집(Prometheus)을 고른 완료 스택만 — Alertmanager 는 그 차트(kube-prometheus-stack)에 함께 선다.
func TestMonitoredStacks_OnlyCompletedStacksWithCollection(t *testing.T) {
	lister := NewMonitoredStacks(fakeCompleted{stacks: []*stackdomain.Stack{
		{ID: "stk_a", Name: "gitlab", OrgID: "org-1", ClusterID: "c1", Namespace: "nullus-gitlab", State: stackdomain.StateCompleted, Config: cfg(true)},
		{ID: "stk_b", Name: "lite", OrgID: "org-1", ClusterID: "c1", Namespace: "nullus-lite", State: stackdomain.StateCompleted, Config: cfg(false)},
		{ID: "stk_c", Name: "no-ns", OrgID: "org-2", ClusterID: "c2", Namespace: "", State: stackdomain.StateCompleted, Config: cfg(true)},
		{ID: "stk_d", Name: "failed", OrgID: "org-2", ClusterID: "c2", Namespace: "nullus-x", State: stackdomain.StateFailed, Config: cfg(true)},
	}})

	got, err := lister.ListMonitoredStacks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []domain.MonitoredStack{
		{ID: "stk_a", OrgID: "org-1", Name: "gitlab", Namespace: "nullus-gitlab", ClusterID: "c1"},
	}, got)
}
