package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

// GetDashboardOutput holds the result of the GetDashboard use case.
type GetDashboardOutput struct {
	Dashboard *domain.Dashboard
}

// GetDashboard retrieves the platform observability dashboard.
type GetDashboard struct {
	dashboardRepo port.DashboardRepository
}

// NewGetDashboard constructs a GetDashboard use case.
func NewGetDashboard(dashboardRepo port.DashboardRepository) *GetDashboard {
	return &GetDashboard{dashboardRepo: dashboardRepo}
}

// Execute returns the current dashboard data.
func (uc *GetDashboard) Execute(ctx context.Context) (*GetDashboardOutput, error) {
	dashboard, err := uc.dashboardRepo.GetDashboard(ctx)
	if err != nil {
		return nil, fmt.Errorf("get dashboard: %w", err)
	}
	return &GetDashboardOutput{Dashboard: dashboard}, nil
}
