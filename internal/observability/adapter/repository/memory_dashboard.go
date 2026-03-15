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

// MemoryAlertRuleRepository is an in-memory implementation of port.AlertRuleRepository.
type MemoryAlertRuleRepository struct {
	mu    sync.RWMutex
	rules map[string]*domain.AlertRule
}

// NewMemoryAlertRuleRepository constructs an empty MemoryAlertRuleRepository.
func NewMemoryAlertRuleRepository() *MemoryAlertRuleRepository {
	return &MemoryAlertRuleRepository{
		rules: make(map[string]*domain.AlertRule),
	}
}

// Create stores a new alert rule.
func (r *MemoryAlertRuleRepository) Create(_ context.Context, rule *domain.AlertRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rule.ID] = cloneAlertRule(rule)
	return nil
}

// GetByID retrieves an alert rule by its ID.
func (r *MemoryAlertRuleRepository) GetByID(_ context.Context, id string) (*domain.AlertRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rule, ok := r.rules[id]
	if !ok {
		return nil, domain.ErrAlertRuleNotFound
	}
	return cloneAlertRule(rule), nil
}

// List returns all alert rules.
func (r *MemoryAlertRuleRepository) List(_ context.Context) ([]*domain.AlertRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.AlertRule, 0, len(r.rules))
	for _, rule := range r.rules {
		result = append(result, cloneAlertRule(rule))
	}
	return result, nil
}

func (r *MemoryAlertRuleRepository) Update(_ context.Context, rule *domain.AlertRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.rules[rule.ID]; !ok {
		return domain.ErrAlertRuleNotFound
	}
	r.rules[rule.ID] = cloneAlertRule(rule)
	return nil
}

func (r *MemoryAlertRuleRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.rules[id]; !ok {
		return domain.ErrAlertRuleNotFound
	}
	delete(r.rules, id)
	return nil
}

// MemoryAlertRepository is an in-memory implementation of port.AlertRepository.
type MemoryAlertRepository struct {
	mu     sync.RWMutex
	alerts []*domain.Alert
}

// NewMemoryAlertRepository constructs an empty MemoryAlertRepository.
func NewMemoryAlertRepository() *MemoryAlertRepository {
	return &MemoryAlertRepository{}
}

// Create stores a new alert.
func (r *MemoryAlertRepository) Create(_ context.Context, alert *domain.Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.alerts = append(r.alerts, cloneAlert(alert))
	return nil
}

// List returns all alerts.
func (r *MemoryAlertRepository) List(_ context.Context) ([]*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Alert, len(r.alerts))
	for i, alert := range r.alerts {
		result[i] = cloneAlert(alert)
	}
	return result, nil
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
