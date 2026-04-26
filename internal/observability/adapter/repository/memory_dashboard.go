package repository

import (
	"context"
	"sync"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

// MemoryDashboardRepository provides simulated dashboard data.
type MemoryDashboardRepository struct {
	mu        sync.RWMutex
	dashboard *domain.Dashboard
}

// NewMemoryDashboardRepository constructs a MemoryDashboardRepository.
func NewMemoryDashboardRepository() *MemoryDashboardRepository {
	return &MemoryDashboardRepository{
		dashboard: defaultDashboard(),
	}
}

// GetDashboard returns simulated platform metrics and tool health data.
func (r *MemoryDashboardRepository) GetDashboard(_ context.Context) (*domain.Dashboard, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneDashboard(r.dashboard), nil
}

func defaultDashboard() *domain.Dashboard {
	return &domain.Dashboard{
		ClusterMetrics: domain.ClusterMetrics{
			CPUUsage:     42.5,
			MemoryUsage:  61.3,
			StorageUsage: 35.0,
			PodCount:     128,
		},
		PipelineMetrics: domain.PipelineMetrics{
			TotalRuns:    247,
			SuccessRate:  94.3,
			AvgBuildTime: 183.5,
		},
		ToolHealthList: []domain.ToolHealth{
			{Name: "GitLab CE", Status: "running", Version: "17.7.2"},
			{Name: "Argo CD", Status: "running", Version: "2.13.2"},
			{Name: "Harbor", Status: "running", Version: "2.11.0"},
			{Name: "Prometheus", Status: "running", Version: "3.1.0"},
			{Name: "Grafana", Status: "running", Version: "11.4.0"},
			{Name: "MinIO", Status: "warning", Version: "2024.11.7"},
		},
	}
}

func cloneDashboard(d *domain.Dashboard) *domain.Dashboard {
	if d == nil {
		return nil
	}
	cp := *d
	if d.ToolHealthList != nil {
		cp.ToolHealthList = make([]domain.ToolHealth, len(d.ToolHealthList))
		copy(cp.ToolHealthList, d.ToolHealthList)
	}
	return &cp
}

func cloneAlertRule(rule *domain.AlertRule) *domain.AlertRule {
	if rule == nil {
		return nil
	}
	cp := *rule
	return &cp
}

func cloneAlert(alert *domain.Alert) *domain.Alert {
	if alert == nil {
		return nil
	}
	cp := *alert
	if alert.ResolvedAt != nil {
		resolvedAt := *alert.ResolvedAt
		cp.ResolvedAt = &resolvedAt
	}
	return &cp
}
