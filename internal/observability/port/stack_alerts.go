package port

import (
	"context"
	"time"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

// MonitoredStackLister 는 경보를 가져올 스택(설치 완료 · 모니터링 수집 선택)이다. 모든 조직의 것이다.
type MonitoredStackLister interface {
	ListMonitoredStacks(ctx context.Context) ([]domain.MonitoredStack, error)
}

// ActiveAlertSource 는 스택 Alertmanager 의 활성 경보다(침묵 · 억제된 것은 뺀다). 닿지 못하면 오류다 —
// 빈 목록과 구별해야 한다(빈 목록이면 열린 경보를 모두 해제한다).
type ActiveAlertSource interface {
	ActiveAlerts(ctx context.Context, stack domain.MonitoredStack) ([]domain.SourceAlert, error)
}

// StackAlertRepository 는 스택 경보 기록이다. API 레플리카가 여럿이어도 한 번만 알리도록, 알림 차례와
// 해제는 저장소가 원자적으로 가린다.
type StackAlertRepository interface {
	// Upsert 는 열린 경보를 새로 만들거나(true) 마지막 확인 시각 · 내용을 갱신한다. a.ID 를 채운다.
	Upsert(ctx context.Context, a *domain.StackAlert) (created bool, err error)
	// ClaimNotification 은 알릴 차례면(알린 적이 없거나 repeat 이 지났으면) 알린 시각을 적고 claimed 를
	// 돌려준다. firstTime 은 전에 알린 적이 없었는지다.
	ClaimNotification(ctx context.Context, id string, now time.Time, repeat time.Duration) (claimed, firstTime bool, err error)
	ListOpen(ctx context.Context, stackID string) ([]*domain.StackAlert, error)
	// Resolve 는 staleBefore 전부터 보이지 않은 열린 경보를 해제한다. 이 호출이 해제했으면 true 다.
	Resolve(ctx context.Context, id string, at, staleBefore time.Time) (bool, error)
	// CloseForMissingStacks 는 keep 에 없는 스택(지워졌거나 모니터링을 끈 스택)의 열린 경보 가운데
	// seenBefore 전부터 보이지 않은 것을 알리지 않고 닫는다.
	CloseForMissingStacks(ctx context.Context, keep []string, at, seenBefore time.Time) (int, error)
	// List 는 조직의 경보다. openOnly 면 해제되지 않은 것만.
	List(ctx context.Context, orgID string, openOnly bool, limit int) ([]*domain.StackAlert, error)
}

// CycleLock 은 수집 한 번을 레플리카 하나만 돌게 하는 잠금이다. API 레플리카마다 Alertmanager 를 다른
// 시각에 읽으면 한쪽은 경보를 보고 다른 쪽은 못 봐 발생 · 해제가 엇갈린다. 잡지 못하면 이번 주기는 건너뛴다.
type CycleLock interface {
	TryAcquire(ctx context.Context) (release func(), acquired bool, err error)
}
