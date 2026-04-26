package repository

import (
	"context"
	"sync"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

type MemoryAlertRuleRepository struct {
	mu    sync.RWMutex
	rules map[string]*domain.AlertRule
}

func NewMemoryAlertRuleRepository() *MemoryAlertRuleRepository {
	return &MemoryAlertRuleRepository{rules: make(map[string]*domain.AlertRule)}
}

func (r *MemoryAlertRuleRepository) Create(_ context.Context, rule *domain.AlertRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rule.Condition = normalizeAlertRuleCondition(rule)
	rule.Threshold = rule.CriticalThreshold
	r.rules[rule.ID] = cloneAlertRule(rule)
	return nil
}

func (r *MemoryAlertRuleRepository) GetByID(_ context.Context, id string) (*domain.AlertRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rule, ok := r.rules[id]
	if !ok {
		return nil, domain.ErrAlertRuleNotFound
	}
	return cloneAlertRule(rule), nil
}

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
	rule.Condition = normalizeAlertRuleCondition(rule)
	rule.Threshold = rule.CriticalThreshold
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

type MemoryAlertRepository struct {
	mu     sync.RWMutex
	alerts []*domain.Alert
}

func NewMemoryAlertRepository() *MemoryAlertRepository {
	return &MemoryAlertRepository{}
}

func (r *MemoryAlertRepository) Create(_ context.Context, alert *domain.Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.alerts = append(r.alerts, cloneAlert(alert))
	return nil
}

func (r *MemoryAlertRepository) List(_ context.Context) ([]*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Alert, len(r.alerts))
	for i, alert := range r.alerts {
		result[i] = cloneAlert(alert)
	}
	return result, nil
}

func normalizeAlertRuleCondition(rule *domain.AlertRule) string {
	if rule == nil {
		return ""
	}
	if rule.MetricName == "" {
		return rule.Condition
	}
	return rule.MetricName + " >= critical_threshold"
}
