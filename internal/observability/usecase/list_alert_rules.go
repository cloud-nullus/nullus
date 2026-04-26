package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

type ListAlertRulesOutput struct {
	Rules []*domain.AlertRule
}

type ListAlertRules struct {
	alertRuleRepo port.AlertRuleRepository
}

func NewListAlertRules(alertRuleRepo port.AlertRuleRepository) *ListAlertRules {
	return &ListAlertRules{alertRuleRepo: alertRuleRepo}
}

func (uc *ListAlertRules) Execute(ctx context.Context) (*ListAlertRulesOutput, error) {
	rules, err := uc.alertRuleRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	return &ListAlertRulesOutput{Rules: rules}, nil
}
