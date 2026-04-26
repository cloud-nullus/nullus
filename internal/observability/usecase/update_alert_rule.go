package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

type UpdateAlertRuleInput struct {
	ID                string
	Name              *string
	MetricName        *string
	WarningThreshold  *float64
	CriticalThreshold *float64
	Channel           *domain.AlertChannel
	Enabled           *bool
}

type UpdateAlertRuleOutput struct {
	Rule *domain.AlertRule
}

type UpdateAlertRule struct {
	alertRuleRepo port.AlertRuleRepository
}

func NewUpdateAlertRule(alertRuleRepo port.AlertRuleRepository) *UpdateAlertRule {
	return &UpdateAlertRule{alertRuleRepo: alertRuleRepo}
}

func (uc *UpdateAlertRule) Execute(ctx context.Context, input UpdateAlertRuleInput) (*UpdateAlertRuleOutput, error) {
	rule, err := uc.alertRuleRepo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	updated := *rule
	if input.Name != nil {
		updated.Name = *input.Name
	}
	if input.MetricName != nil {
		updated.MetricName = *input.MetricName
		updated.Condition = fmt.Sprintf("%s >= critical_threshold", *input.MetricName)
	}
	if input.WarningThreshold != nil {
		updated.WarningThreshold = *input.WarningThreshold
	}
	if input.CriticalThreshold != nil {
		updated.CriticalThreshold = *input.CriticalThreshold
	}
	if updated.WarningThreshold <= 0 {
		return nil, fmt.Errorf("update alert rule: warning_threshold must be greater than 0")
	}
	if updated.CriticalThreshold <= 0 {
		return nil, fmt.Errorf("update alert rule: critical_threshold must be greater than 0")
	}
	if updated.CriticalThreshold < updated.WarningThreshold {
		return nil, fmt.Errorf("update alert rule: critical_threshold must be greater than or equal to warning_threshold")
	}
	updated.Threshold = updated.CriticalThreshold
	if input.Channel != nil {
		updated.Channel = *input.Channel
	}
	if input.Enabled != nil {
		updated.Enabled = *input.Enabled
	}

	if err := uc.alertRuleRepo.Update(ctx, &updated); err != nil {
		return nil, fmt.Errorf("update alert rule: %w", err)
	}

	return &UpdateAlertRuleOutput{Rule: &updated}, nil
}
