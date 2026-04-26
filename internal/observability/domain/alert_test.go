package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertRule_ConstructWithExpectedFields(t *testing.T) {
	rule := AlertRule{
		ID:                "rule-1",
		Name:              "High CPU",
		MetricName:        "cpu_usage",
		Condition:         "cpu_usage >= critical_threshold",
		WarningThreshold:  70,
		CriticalThreshold: 80,
		Threshold:         80,
		Channel:           AlertChannelSlack,
		Enabled:           true,
	}

	assert.Equal(t, "rule-1", rule.ID)
	assert.Equal(t, "High CPU", rule.Name)
	assert.Equal(t, "cpu_usage", rule.MetricName)
	assert.Equal(t, "cpu_usage >= critical_threshold", rule.Condition)
	assert.Equal(t, 70.0, rule.WarningThreshold)
	assert.Equal(t, 80.0, rule.CriticalThreshold)
	assert.Equal(t, 80.0, rule.Threshold)
	assert.Equal(t, AlertChannelSlack, rule.Channel)
	assert.True(t, rule.Enabled)
}

func TestAlert_ConstructWithResolvedAt(t *testing.T) {
	firedAt := time.Now().UTC().Truncate(time.Second)
	resolvedAt := firedAt.Add(5 * time.Minute)

	alert := Alert{
		ID:         "alert-1",
		RuleID:     "rule-1",
		Severity:   AlertSeverityCritical,
		Message:    "CPU is above threshold",
		FiredAt:    firedAt,
		ResolvedAt: &resolvedAt,
	}

	assert.Equal(t, "alert-1", alert.ID)
	assert.Equal(t, "rule-1", alert.RuleID)
	assert.Equal(t, AlertSeverityCritical, alert.Severity)
	assert.Equal(t, "CPU is above threshold", alert.Message)
	assert.Equal(t, firedAt, alert.FiredAt)
	require.NotNil(t, alert.ResolvedAt)
	assert.Equal(t, resolvedAt, *alert.ResolvedAt)
}

func TestAlertChannel_Constants(t *testing.T) {
	assert.Equal(t, AlertChannel("slack"), AlertChannelSlack)
	assert.Equal(t, AlertChannel("email"), AlertChannelEmail)
}

func TestAlertSeverity_Constants(t *testing.T) {
	assert.Equal(t, AlertSeverity("critical"), AlertSeverityCritical)
	assert.Equal(t, AlertSeverity("warning"), AlertSeverityWarning)
	assert.Equal(t, AlertSeverity("info"), AlertSeverityInfo)
}
