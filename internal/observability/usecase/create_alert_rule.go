package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

type CreateAlertRuleInput struct {
	Name              string
	MetricName        string
	WarningThreshold  float64
	CriticalThreshold float64
	Channel           domain.AlertChannel
	Enabled           bool
}

type CreateAlertRuleOutput struct {
	Rule *domain.AlertRule
}

type CreateAlertRule struct {
	alertRuleRepo port.AlertRuleRepository
}

func NewCreateAlertRule(alertRuleRepo port.AlertRuleRepository) *CreateAlertRule {
	return &CreateAlertRule{alertRuleRepo: alertRuleRepo}
}

func (uc *CreateAlertRule) Execute(ctx context.Context, input CreateAlertRuleInput) (*CreateAlertRuleOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("alert rule name is required")
	}
	if input.MetricName == "" {
		return nil, fmt.Errorf("alert rule metric_name is required")
	}
	if input.WarningThreshold <= 0 {
		return nil, fmt.Errorf("alert rule warning_threshold must be greater than 0")
	}
	if input.CriticalThreshold <= 0 {
		return nil, fmt.Errorf("alert rule critical_threshold must be greater than 0")
	}
	if input.CriticalThreshold < input.WarningThreshold {
		return nil, fmt.Errorf("alert rule critical_threshold must be greater than or equal to warning_threshold")
	}

	rule := &domain.AlertRule{
		ID:                generateID("alr"),
		Name:              input.Name,
		MetricName:        input.MetricName,
		Condition:         fmt.Sprintf("%s >= critical_threshold", input.MetricName),
		WarningThreshold:  input.WarningThreshold,
		CriticalThreshold: input.CriticalThreshold,
		Threshold:         input.CriticalThreshold,
		Channel:           input.Channel,
		Enabled:           input.Enabled,
	}

	if err := uc.alertRuleRepo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("create alert rule: %w", err)
	}

	return &CreateAlertRuleOutput{Rule: rule}, nil
}
