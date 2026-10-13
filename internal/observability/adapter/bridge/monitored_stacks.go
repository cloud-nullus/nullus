// Package bridge 는 관측 모듈이 다른 모듈의 것을 읽는 자리다. 다른 모듈의 타입은 여기서만 만지고, 관측
// 모듈 안으로는 관측 도메인 타입으로 옮겨 넣는다(백업 모듈의 bridge 와 같은 방식).
package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
)

// CompletedStackLister 는 모든 조직의 설치 완료 스택이다(스택 모듈의 Postgres 저장소).
type CompletedStackLister interface {
	ListCompleted(ctx context.Context) ([]*stackdomain.Stack, error)
}

// MonitoredStacks 는 port.MonitoredStackLister 다.
type MonitoredStacks struct {
	stacks CompletedStackLister
}

func NewMonitoredStacks(stacks CompletedStackLister) *MonitoredStacks {
	return &MonitoredStacks{stacks: stacks}
}

var _ port.MonitoredStackLister = (*MonitoredStacks)(nil)

// ListMonitoredStacks 는 모니터링 수집(Prometheus)을 고른 완료 스택이다 — Alertmanager 는 그 차트
// (kube-prometheus-stack)에 함께 선다. 수집을 고르지 않은 스택에는 경보를 낼 것이 없다.
func (m *MonitoredStacks) ListMonitoredStacks(ctx context.Context) ([]domain.MonitoredStack, error) {
	if m == nil || m.stacks == nil {
		return nil, fmt.Errorf("스택 저장소가 배선되지 않았습니다")
	}
	stacks, err := m.stacks.ListCompleted(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.MonitoredStack, 0, len(stacks))
	for _, s := range stacks {
		if s == nil || s.State != stackdomain.StateCompleted || strings.TrimSpace(s.Namespace) == "" {
			continue
		}
		if !stackdomain.DecodeStackConfig(s.Config).Monitoring.Collection.Enabled {
			continue
		}
		out = append(out, domain.MonitoredStack{
			ID: s.ID, OrgID: s.OrgID, Name: s.Name, Namespace: s.Namespace, ClusterID: s.ClusterID,
		})
	}
	return out, nil
}
