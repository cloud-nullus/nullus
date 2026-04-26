package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

type ListAlertsOutput struct {
	Alerts []*domain.Alert
}

type ListAlerts struct {
	alertRepo port.AlertRepository
}

func NewListAlerts(alertRepo port.AlertRepository) *ListAlerts {
	return &ListAlerts{alertRepo: alertRepo}
}

func (uc *ListAlerts) Execute(ctx context.Context) (*ListAlertsOutput, error) {
	alerts, err := uc.alertRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	return &ListAlertsOutput{Alerts: alerts}, nil
}
