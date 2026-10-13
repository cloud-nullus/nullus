package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

const (
	// DefaultAlertRepeatInterval 은 계속 발생 중인 경보를 다시 알리는 간격이다(Alertmanager 기본 repeat_interval 과 같다).
	DefaultAlertRepeatInterval = 4 * time.Hour
	// DefaultResolveGrace 는 이만큼 보이지 않아야 해제한다. Alertmanager 는 경보를 메모리에만 들고 있어
	// 재시작하면 Prometheus 가 다시 보낼 때까지(1분 안팎) 빈 목록을 준다 — 그것을 해제로 읽으면
	// "해제" 와 "발생" 이 한꺼번에 나간다.
	DefaultResolveGrace = 3 * time.Minute
	// publishTimeout 은 알림 하나를 모든 채널로 보내는 시간이다. 수집 ctx 와 따로 준다.
	publishTimeout = 2 * time.Minute
)

// CollectStackAlerts 는 스택마다 Alertmanager 의 활성 경보를 가져와 기록하고, 심각한 것을 조직의 알림
// 채널로 보낸다(nullus-plan#87). 규칙은 스택 Prometheus 의 기본 규칙이다 — 플랫폼은 평가하지 않는다.
//
// 한 번 돌 때:
//  1. 스택마다 활성 경보를 가져와 거른다(심각만, 잡음 제외).
//  2. 새로 보이면 기록하고 발생 알림, 계속 보이면 repeat 이 지났을 때 다시 알림.
//  3. 기록엔 열려 있는데 이번에 안 보이면 해제하고 해제 알림. **Alertmanager 에 닿지 못한 스택은 건너뛴다** —
//     모르는 것을 없는 것으로 읽으면 장애가 나는 순간 모든 경보가 해제됐다고 알린다.
//  4. 목록에 없는 스택(지워졌거나 모니터링을 끈 스택)의 열린 경보는 알리지 않고 닫는다.
//
// API 레플리카가 여럿이면 잠금(CycleLock)을 잡은 하나만 돈다. 그래도 알림 차례와 해제는 저장소가
// 원자적으로 가린다(잠금이 없거나 풀린 틈에도 한 번만 알린다).
//
// 발송은 수집이 끝난 뒤 한꺼번에(동시에), 수집 ctx 와 따로 보낸다 — 응답 없는 채널 하나가 수집 시간을
// 다 써서 다른 스택을 건너뛰거나, 차례를 잡은 알림이 수집 ctx 와 함께 취소되지 않게.
type CollectStackAlerts struct {
	stacks    port.MonitoredStackLister
	source    port.ActiveAlertSource
	repo      port.StackAlertRepository
	publisher sharednotify.Publisher
	lock      port.CycleLock
	filter    domain.AlertFilter
	repeat    time.Duration
	grace     time.Duration
	now       func() time.Time
	logger    *slog.Logger
}

// WithLock 은 레플리카 하나만 돌게 하는 잠금을 배선한다.
func (uc *CollectStackAlerts) WithLock(lock port.CycleLock) *CollectStackAlerts {
	uc.lock = lock
	return uc
}

func NewCollectStackAlerts(stacks port.MonitoredStackLister, source port.ActiveAlertSource, repo port.StackAlertRepository, publisher sharednotify.Publisher) *CollectStackAlerts {
	return &CollectStackAlerts{
		stacks: stacks, source: source, repo: repo, publisher: publisher,
		filter: domain.DefaultAlertFilter(), repeat: DefaultAlertRepeatInterval, grace: DefaultResolveGrace,
		now: time.Now, logger: slog.Default(),
	}
}

// CollectResult 는 한 번 돈 결과다(로그용).
type CollectResult struct {
	// Skipped 는 다른 레플리카가 이번 주기를 돌고 있어 건너뛰었다는 뜻이다.
	Skipped     bool
	Stacks      int
	Unreachable int
	Fired       int
	Repeated    int
	Resolved    int
	Closed      int
}

func (uc *CollectStackAlerts) Run(ctx context.Context) (CollectResult, error) {
	var res CollectResult
	if uc.lock != nil {
		release, ok, err := uc.lock.TryAcquire(ctx)
		if err != nil {
			return res, fmt.Errorf("스택 경보 수집 잠금: %w", err)
		}
		if !ok {
			res.Skipped = true
			return res, nil
		}
		defer release()
	}
	cycleStart := uc.now()
	stacks, err := uc.stacks.ListMonitoredStacks(ctx)
	if err != nil {
		return res, fmt.Errorf("경보를 가져올 스택 조회: %w", err)
	}
	res.Stacks = len(stacks)
	keep := make([]string, 0, len(stacks))
	var events []sharednotify.Event
	for _, stack := range stacks {
		keep = append(keep, stack.ID)
		stackEvents, err := uc.collectStack(ctx, stack, &res)
		if err != nil {
			uc.logger.Warn("스택 경보를 가져오지 못했습니다 — 이번에는 열린 경보를 그대로 둡니다",
				"stack_id", stack.ID, "namespace", stack.Namespace, "error", err)
			res.Unreachable++
			continue
		}
		events = append(events, stackEvents...)
	}
	closed, err := uc.repo.CloseForMissingStacks(ctx, keep, uc.now(), cycleStart)
	if err != nil {
		uc.logger.Warn("지워진 스택의 경보를 닫지 못했습니다", "error", err)
	}
	res.Closed = closed
	uc.publishAll(ctx, events)
	return res, nil
}

// collectStack 은 스택 하나를 처리하고 보낼 알림을 돌려준다. 가져오기 실패만 오류다(그 스택은 해제하지 않는다).
func (uc *CollectStackAlerts) collectStack(ctx context.Context, stack domain.MonitoredStack, res *CollectResult) ([]sharednotify.Event, error) {
	alerts, err := uc.source.ActiveAlerts(ctx, stack)
	if err != nil {
		return nil, err
	}
	now := uc.now()
	var events []sharednotify.Event
	seen := map[string]bool{}
	for _, src := range alerts {
		if !uc.filter.Accept(src) || seen[src.Fingerprint] {
			continue
		}
		seen[src.Fingerprint] = true
		alert := domain.NewStackAlert(stack, src, now)
		if _, err := uc.repo.Upsert(ctx, alert); err != nil {
			uc.logger.Warn("스택 경보 기록 실패", "stack_id", stack.ID, "alert", alert.AlertName, "error", err)
			continue
		}
		// 침묵 · 억제된 경보는 기록만 한다 — 보인 것으로 쳐서 해제하지 않고, 알리지도 않는다.
		if src.Suppressed {
			continue
		}
		claimed, first, err := uc.repo.ClaimNotification(ctx, alert.ID, now, uc.repeat)
		if err != nil || !claimed {
			continue
		}
		if first {
			res.Fired++
		} else {
			res.Repeated++
		}
		events = append(events, alert.FiredEvent(!first))
	}

	open, err := uc.repo.ListOpen(ctx, stack.ID)
	if err != nil {
		uc.logger.Warn("열린 스택 경보 조회 실패", "stack_id", stack.ID, "error", err)
		return events, nil
	}
	staleBefore := now.Add(-uc.grace)
	for _, alert := range open {
		if seen[alert.Fingerprint] {
			continue
		}
		resolved, err := uc.repo.Resolve(ctx, alert.ID, now, staleBefore)
		if err != nil || !resolved {
			continue
		}
		res.Resolved++
		alert.Resolve(now)
		// 발생을 알린 적이 없으면 해제도 알리지 않는다 — 받은 사람에게 짝이 맞지 않는다.
		if alert.LastNotifiedAt != nil {
			events = append(events, alert.ResolvedEvent())
		}
	}
	return events, nil
}

// publishAll 은 모은 알림을 동시에 보낸다. 수집 ctx 가 끝나도(시간 초과 · 종료) 차례를 잡은 알림은 나간다.
func (uc *CollectStackAlerts) publishAll(ctx context.Context, events []sharednotify.Event) {
	if uc.publisher == nil || len(events) == 0 {
		return
	}
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishTimeout)
	defer cancel()
	var wg sync.WaitGroup
	for _, e := range events {
		wg.Add(1)
		go func(e sharednotify.Event) {
			defer wg.Done()
			uc.publish(pubCtx, e)
		}(e)
	}
	wg.Wait()
}

func (uc *CollectStackAlerts) publish(ctx context.Context, e sharednotify.Event) {
	if uc.publisher == nil {
		return
	}
	if err := uc.publisher.Publish(ctx, e); err != nil {
		uc.logger.Warn("스택 경보 알림 발송 실패 — 알림 채널의 발송 이력을 확인하세요", "title", e.Title, "error", err)
	}
}
