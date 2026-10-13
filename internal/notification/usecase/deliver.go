package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
)

const (
	// maxAttempts 는 채널 하나에 보내 보는 횟수다(처음 + 재시도 2번).
	maxAttempts = 3
	// baseRetryWait 는 첫 재시도까지의 대기다. 이후 두 배씩 는다.
	baseRetryWait = time.Second
	// maxRetryWait 는 한 번 기다리는 최대 시간이다. 서버가 더 길게 기다리라 해도(429 retry_after)
	// 백업 같은 생산자를 붙들지 않는다.
	maxRetryWait = 10 * time.Second
	// attemptTimeout 은 한 번 보내는 데 쓰는 시간이다(SMTP 연결 · TLS · 전송 포함).
	attemptTimeout = 20 * time.Second
)

// Deliverer 는 채널 하나로 메시지를 보내고(일시적 실패는 재시도) 결과를 발송 이력에 남긴다.
type Deliverer struct {
	senders    map[domain.ChannelType]port.Sender
	deliveries port.DeliveryRepository
	sleep      func(ctx context.Context, d time.Duration) error
	now        func() time.Time
}

func NewDeliverer(senders map[domain.ChannelType]port.Sender, deliveries port.DeliveryRepository) *Deliverer {
	return &Deliverer{senders: senders, deliveries: deliveries, sleep: sleepContext, now: time.Now}
}

// Deliver 는 보낸 결과를 돌려준다. 이력 기록이 실패해도 발송 결과는 그대로다.
func (d *Deliverer) Deliver(ctx context.Context, ch *domain.Channel, msg domain.Message) *domain.Delivery {
	result := &domain.Delivery{
		OrgID: ch.OrgID, ChannelID: ch.ID, ChannelName: ch.Name, ChannelType: ch.Type,
		Event: msg.Event, Title: msg.Title, Status: domain.DeliveryFailed,
	}
	err := d.send(ctx, ch, msg, result)
	if err == nil {
		result.Status = domain.DeliverySent
	} else {
		result.Error = err.Error()
	}
	result.CreatedAt = d.now()
	if d.deliveries != nil {
		if recErr := d.deliveries.Record(ctx, result); recErr != nil {
			slog.Warn("알림 발송 이력 기록 실패", "channel_id", ch.ID, "event", msg.Event, "error", recErr)
		}
	}
	return result
}

func (d *Deliverer) send(ctx context.Context, ch *domain.Channel, msg domain.Message, result *domain.Delivery) error {
	sender, ok := d.senders[ch.Type]
	if !ok || sender == nil {
		result.Attempts = 1
		return fmt.Errorf("지원하지 않는 채널 종류입니다: %s", ch.Type)
	}
	wait := baseRetryWait
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Attempts = attempt
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		err = sender.Send(attemptCtx, ch, msg)
		cancel()
		if err == nil {
			return nil
		}
		se, ok := port.AsSendError(err)
		if !ok || !se.Retryable || attempt == maxAttempts {
			return err
		}
		next := wait
		if se.RetryAfter > 0 {
			next = se.RetryAfter
		}
		if next > maxRetryWait {
			next = maxRetryWait
		}
		if sleepErr := d.sleep(ctx, next); sleepErr != nil {
			return err
		}
		wait *= 2
	}
	return err
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
