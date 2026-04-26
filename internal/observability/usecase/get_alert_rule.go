package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

type GetAlertRuleInput struct {
	ID string
}

type GetAlertRuleOutput struct {
	Rule *domain.AlertRule
}

type GetAlertRule struct {
	alertRuleRepo port.AlertRuleRepository
}

func NewGetAlertRule(alertRuleRepo port.AlertRuleRepository) *GetAlertRule {
	return &GetAlertRule{alertRuleRepo: alertRuleRepo}
}

func (uc *GetAlertRule) Execute(ctx context.Context, input GetAlertRuleInput) (*GetAlertRuleOutput, error) {
	rule, err := uc.alertRuleRepo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("get alert rule: %w", err)
	}
	return &GetAlertRuleOutput{Rule: rule}, nil
}
