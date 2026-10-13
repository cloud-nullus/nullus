// Package port 는 알림 모듈의 바깥 경계다.
package port

import (
	"context"
	"errors"
	"time"

	"github.com/cloud-nullus/draft/internal/notification/domain"
)

// ChannelRepository 는 채널을 저장한다. Secret 의 암호화·복호화는 구현체의 일이다 — 유스케이스는
// 평문만 본다. Get·Update·Delete 는 조직이 다르면 domain.ErrChannelNotFound 다.
type ChannelRepository interface {
	Create(ctx context.Context, ch *domain.Channel) error
	Get(ctx context.Context, orgID, id string) (*domain.Channel, error)
	List(ctx context.Context, orgID string) ([]*domain.Channel, error)
	Update(ctx context.Context, ch *domain.Channel) error
	Delete(ctx context.Context, orgID, id string) error
}

// DeliveryRepository 는 발송 이력이다.
type DeliveryRepository interface {
	Record(ctx context.Context, d *domain.Delivery) error
	List(ctx context.Context, orgID string, limit int) ([]*domain.Delivery, error)
}

// Sender 는 채널 종류 하나로 메시지를 보낸다.
type Sender interface {
	Send(ctx context.Context, ch *domain.Channel, msg domain.Message) error
}

// SendError 는 보내는 쪽이 실패를 분류한 것이다. Retryable 이면 잠시 뒤 다시 보낸다(네트워크 ·
// 5xx · 429). 설정이 틀린 실패(401 · 404 · 잘못된 chat_id)는 다시 보내도 같다.
//
// Err 의 문구에 비밀(봇 토큰 · 웹훅 URL)이 들어가지 않게 하는 것은 보내는 쪽의 책임이다 — 이
// 문구는 발송 이력과 화면에 그대로 남는다.
type SendError struct {
	Err        error
	Retryable  bool
	RetryAfter time.Duration
}

func (e *SendError) Error() string { return e.Err.Error() }
func (e *SendError) Unwrap() error { return e.Err }

// AsSendError 는 err 가 SendError 면 꺼낸다. 아니면 다시 보내지 않는 실패로 본다.
func AsSendError(err error) (*SendError, bool) {
	var se *SendError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}
