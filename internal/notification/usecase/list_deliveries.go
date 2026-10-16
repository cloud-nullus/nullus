package usecase

import (
	"context"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
)

const (
	defaultDeliveryLimit = 50
	maxDeliveryLimit     = 200
)

// ListDeliveries 는 조직의 최근 발송 이력이다.
type ListDeliveries struct {
	deliveries port.DeliveryRepository
}

func NewListDeliveries(deliveries port.DeliveryRepository) *ListDeliveries {
	return &ListDeliveries{deliveries: deliveries}
}

func (l *ListDeliveries) Execute(ctx context.Context, orgID string, limit int) ([]*domain.Delivery, error) {
	if orgID == "" {
		return nil, ErrOrgRequired
	}
	if limit <= 0 {
		limit = defaultDeliveryLimit
	}
	if limit > maxDeliveryLimit {
		limit = maxDeliveryLimit
	}
	return l.deliveries.List(ctx, orgID, limit)
}
