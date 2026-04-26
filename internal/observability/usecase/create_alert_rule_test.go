package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	"github.com/cloud-nullus/draft/internal/observability/domain"
)

func TestCreateAlertRule_Execute_Success(t *testing.T) {
	repo := repository.NewMemoryAlertRuleRepository()
	uc := NewCreateAlertRule(repo)

	out, err := uc.Execute(context.Background(), CreateAlertRuleInput{
		Name:              "High CPU",
		MetricName:        "cpu_usage",
		WarningThreshold:  70,
		CriticalThreshold: 85,
		Channel:           domain.AlertChannelSlack,
		Enabled:           true,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Rule)

	assert.NotEmpty(t, out.Rule.ID)
	assert.Contains(t, out.Rule.ID, "alr_")
	assert.Equal(t, "High CPU", out.Rule.Name)
	assert.Equal(t, "cpu_usage", out.Rule.MetricName)
	assert.Equal(t, 70.0, out.Rule.WarningThreshold)
	assert.Equal(t, 85.0, out.Rule.CriticalThreshold)
	assert.Equal(t, 85.0, out.Rule.Threshold)
	assert.Equal(t, domain.AlertChannelSlack, out.Rule.Channel)

	persisted, getErr := repo.GetByID(context.Background(), out.Rule.ID)
	require.NoError(t, getErr)
	assert.Equal(t, out.Rule.ID, persisted.ID)
}

func TestCreateAlertRule_Execute_ValidationError(t *testing.T) {
	repo := repository.NewMemoryAlertRuleRepository()
	uc := NewCreateAlertRule(repo)

	out, err := uc.Execute(context.Background(), CreateAlertRuleInput{
		MetricName:        "cpu_usage",
		WarningThreshold:  60,
		CriticalThreshold: 80,
		Channel:           domain.AlertChannelEmail,
		Enabled:           true,
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "alert rule name is required")
}

func TestCreateAlertRule_Execute_MetricNameRequired(t *testing.T) {
	repo := repository.NewMemoryAlertRuleRepository()
	uc := NewCreateAlertRule(repo)

	out, err := uc.Execute(context.Background(), CreateAlertRuleInput{
		Name:              "High CPU",
		WarningThreshold:  70,
		CriticalThreshold: 85,
		Channel:           domain.AlertChannelSlack,
		Enabled:           true,
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "metric_name is required")
}

func TestCreateAlertRule_Execute_InvalidThresholdOrder(t *testing.T) {
	repo := repository.NewMemoryAlertRuleRepository()
	uc := NewCreateAlertRule(repo)

	out, err := uc.Execute(context.Background(), CreateAlertRuleInput{
		Name:              "High CPU",
		MetricName:        "cpu_usage",
		WarningThreshold:  90,
		CriticalThreshold: 80,
		Channel:           domain.AlertChannelSlack,
		Enabled:           true,
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "critical_threshold")
}

func TestCreateAlertRule_Execute_RepositoryError(t *testing.T) {
	repo := &failingAlertRuleRepository{createErr: errors.New("db unavailable")}
	uc := NewCreateAlertRule(repo)

	out, err := uc.Execute(context.Background(), CreateAlertRuleInput{
		Name:              "High Memory",
		MetricName:        "memory_usage",
		WarningThreshold:  75,
		CriticalThreshold: 90,
		Channel:           domain.AlertChannelSlack,
		Enabled:           true,
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "create alert rule")
	assert.Contains(t, err.Error(), "db unavailable")
}

type failingAlertRuleRepository struct {
	createErr error
}

func (r *failingAlertRuleRepository) Create(context.Context, *domain.AlertRule) error {
	return r.createErr
}

func (r *failingAlertRuleRepository) GetByID(context.Context, string) (*domain.AlertRule, error) {
	return nil, domain.ErrAlertRuleNotFound
}

func (r *failingAlertRuleRepository) List(context.Context) ([]*domain.AlertRule, error) {
	return nil, nil
}

func (r *failingAlertRuleRepository) Update(context.Context, *domain.AlertRule) error {
	return nil
}

func (r *failingAlertRuleRepository) Delete(context.Context, string) error {
	return nil
}
