package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// Dispatcher 는 생산자의 이벤트를 그 조직의 구독 채널로 보낸다(sharednotify.Publisher).
type Dispatcher struct {
	channels  port.ChannelRepository
	deliverer *Deliverer
}

func NewDispatcher(channels port.ChannelRepository, deliverer *Deliverer) *Dispatcher {
	return &Dispatcher{channels: channels, deliverer: deliverer}
}

var _ sharednotify.Publisher = (*Dispatcher)(nil)

// Publish 는 켜져 있고 그 이벤트를 구독한 채널마다 보낸다. 한 채널이 실패해도 나머지는 보내고,
// 실패를 모아 돌려준다. 구독한 채널이 없으면 할 일이 없다 — 오류가 아니다.
func (d *Dispatcher) Publish(ctx context.Context, event sharednotify.Event) error {
	if event.OrgID == "" {
		return ErrOrgRequired
	}
	channels, err := d.channels.List(ctx, event.OrgID)
	if err != nil {
		return fmt.Errorf("알림 채널 조회: %w", err)
	}
	msg := domain.NewMessage(event)
	var targets []*domain.Channel
	for _, ch := range channels {
		if ch.Subscribes(event.Type) {
			targets = append(targets, ch)
		}
	}
	// 채널마다 동시에 보낸다. 순서대로 보내면 응답 없는 SMTP 하나가 재시도까지 시간을 다 써서 나머지
	// 채널이 받지 못한다 — 채널은 서로 독립이다.
	failures := make([]error, len(targets))
	var wg sync.WaitGroup
	for i, ch := range targets {
		wg.Add(1)
		go func(i int, ch *domain.Channel) {
			defer wg.Done()
			if res := d.deliverer.Deliver(ctx, ch, msg); res.Status != domain.DeliverySent {
				failures[i] = fmt.Errorf("%s(%s): %s", ch.Name, ch.Type, res.Error)
			}
		}(i, ch)
	}
	wg.Wait()
	return errors.Join(failures...)
}
