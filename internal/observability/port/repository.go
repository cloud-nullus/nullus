package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

// DashboardRepository defines the interface for dashboard data retrieval.
type DashboardRepository interface {
	GetDashboard(ctx context.Context) (*domain.Dashboard, error)
}

// AlertRuleRepository defines the interface for alert rule persistence.
type AlertRuleRepository interface {
	Create(ctx context.Context, rule *domain.AlertRule) error
	GetByID(ctx context.Context, id string) (*domain.AlertRule, error)
	List(ctx context.Context) ([]*domain.AlertRule, error)
	Update(ctx context.Context, rule *domain.AlertRule) error
	Delete(ctx context.Context, id string) error
}

// AlertRepository defines the interface for alert history persistence.
type AlertRepository interface {
	Create(ctx context.Context, alert *domain.Alert) error
	List(ctx context.Context) ([]*domain.Alert, error)
}
