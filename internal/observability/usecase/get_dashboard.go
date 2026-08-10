package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

// GetDashboardOutput holds the result of the GetDashboard use case.
type GetDashboardOutput struct {
	Dashboard *domain.Dashboard
}

// GetDashboard retrieves the platform observability dashboard.
type GetDashboard struct {
	dashboardRepo  port.DashboardRepository
	toolHealthRepo port.ToolHealthRepository
}

// GetDashboardOption configures optional GetDashboard collaborators.
type GetDashboardOption func(*GetDashboard)

// WithToolHealth supplies live OSS health. Without it the dashboard reports
// whatever tool health the dashboard repository carries (the simulated fallback).
func WithToolHealth(repo port.ToolHealthRepository) GetDashboardOption {
	return func(uc *GetDashboard) { uc.toolHealthRepo = repo }
}

// NewGetDashboard constructs a GetDashboard use case.
func NewGetDashboard(dashboardRepo port.DashboardRepository, opts ...GetDashboardOption) *GetDashboard {
	uc := &GetDashboard{dashboardRepo: dashboardRepo}
	for _, o := range opts {
		o(uc)
	}
	return uc
}

// Execute returns the current dashboard data for the organization.
func (uc *GetDashboard) Execute(ctx context.Context, orgID string) (*GetDashboardOutput, error) {
	dashboard, err := uc.dashboardRepo.GetDashboard(ctx)
	if err != nil {
		return nil, fmt.Errorf("get dashboard: %w", err)
	}

	// 실측 소스가 붙어 있으면 그쪽이 유일한 근거다. 조회에 실패했다고 시뮬레이션
	// 값으로 되돌아가면 죽은 도구가 살아 있는 것처럼 보이므로, 차라리 비운다.
	if uc.toolHealthRepo != nil {
		tools, healthErr := uc.toolHealthRepo.ListToolHealth(ctx, orgID)
		if healthErr != nil {
			slog.Warn("dashboard: tool health unavailable", "org_id", orgID, "error", healthErr)
			tools = nil
		}
		dashboard.ToolHealthList = tools
	}

	return &GetDashboardOutput{Dashboard: dashboard}, nil
}
