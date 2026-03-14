package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

// generateID returns a prefixed random ID, e.g. "alr_a1b2c3d4".
func generateID(prefix string) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s_000000000000", prefix)
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

// CreateAlertRuleInput holds the parameters for creating an alert rule.
type CreateAlertRuleInput struct {
	Name      string
	Condition string
	Threshold float64
	Channel   domain.AlertChannel
	Enabled   bool
}

// CreateAlertRuleOutput holds the result of creating an alert rule.
type CreateAlertRuleOutput struct {
	Rule *domain.AlertRule
}

// CreateAlertRule creates a new alert rule.
type CreateAlertRule struct {
	alertRuleRepo port.AlertRuleRepository
}

// NewCreateAlertRule constructs a CreateAlertRule use case.
func NewCreateAlertRule(alertRuleRepo port.AlertRuleRepository) *CreateAlertRule {
	return &CreateAlertRule{alertRuleRepo: alertRuleRepo}
}

// Execute creates and persists a new alert rule.
func (uc *CreateAlertRule) Execute(ctx context.Context, input CreateAlertRuleInput) (*CreateAlertRuleOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("alert rule name is required")
	}
	if input.Condition == "" {
		return nil, fmt.Errorf("alert rule condition is required")
	}

	rule := &domain.AlertRule{
		ID:        generateID("alr"),
		Name:      input.Name,
		Condition: input.Condition,
		Threshold: input.Threshold,
		Channel:   input.Channel,
		Enabled:   input.Enabled,
	}

	if err := uc.alertRuleRepo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("create alert rule: %w", err)
	}

	return &CreateAlertRuleOutput{Rule: rule}, nil
}

// ListAlertsOutput holds the result of listing alert history.
type ListAlertsOutput struct {
	Alerts []*domain.Alert
}

// ListAlerts lists all fired alerts.
type ListAlerts struct {
	alertRepo port.AlertRepository
}

// NewListAlerts constructs a ListAlerts use case.
func NewListAlerts(alertRepo port.AlertRepository) *ListAlerts {
	return &ListAlerts{alertRepo: alertRepo}
}

// Execute returns all alerts from history.
func (uc *ListAlerts) Execute(ctx context.Context) (*ListAlertsOutput, error) {
	alerts, err := uc.alertRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	return &ListAlertsOutput{Alerts: alerts}, nil
}
