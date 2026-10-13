package usecase

import (
	"context"
	"errors"
	"fmt"

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
	var failures []error
	for _, ch := range channels {
		if !ch.Subscribes(event.Type) {
			continue
		}
		if res := d.deliverer.Deliver(ctx, ch, msg); res.Status != domain.DeliverySent {
			failures = append(failures, fmt.Errorf("%s(%s): %s", ch.Name, ch.Type, res.Error))
		}
	}
	return errors.Join(failures...)
}
