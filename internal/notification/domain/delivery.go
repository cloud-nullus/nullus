package domain

import (
	"time"

	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// DeliveryStatus 는 채널 하나로 보낸 결과다.
type DeliveryStatus string

const (
	DeliverySent   DeliveryStatus = "sent"
	DeliveryFailed DeliveryStatus = "failed"
)

// Delivery 는 발송 이력 한 줄이다 — 이벤트 하나가 채널 하나로 간 결과(재시도 포함).
type Delivery struct {
	ID          string
	OrgID       string
	ChannelID   string
	ChannelName string
	ChannelType ChannelType
	Event       sharednotify.EventType
	Status      DeliveryStatus
	Title       string
	Attempts    int
	Error       string
	CreatedAt   time.Time
}
