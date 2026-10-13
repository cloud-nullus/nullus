package usecase

import (
	"context"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
)

// SendTest 는 채널 설정을 확인하는 테스트 메시지를 보낸다. 꺼진 채널에도 보낸다 — 켜기 전에
// 확인하는 버튼이다. 결과는 발송 이력에도 남는다.
type SendTest struct {
	channels  port.ChannelRepository
	deliverer *Deliverer
}

func NewSendTest(channels port.ChannelRepository, deliverer *Deliverer) *SendTest {
	return &SendTest{channels: channels, deliverer: deliverer}
}

func (s *SendTest) Execute(ctx context.Context, orgID, id string) (*domain.Delivery, error) {
	ch, err := s.channels.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return s.deliverer.Deliver(ctx, ch, domain.TestMessage(ch)), nil
}
