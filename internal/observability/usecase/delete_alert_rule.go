package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/observability/port"
)

type DeleteAlertRuleInput struct {
	ID string
}

type DeleteAlertRule struct {
	alertRuleRepo port.AlertRuleRepository
}

func NewDeleteAlertRule(alertRuleRepo port.AlertRuleRepository) *DeleteAlertRule {
	return &DeleteAlertRule{alertRuleRepo: alertRuleRepo}
}

func (uc *DeleteAlertRule) Execute(ctx context.Context, input DeleteAlertRuleInput) error {
	if err := uc.alertRuleRepo.Delete(ctx, input.ID); err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	return nil
}
