package domain

import "time"

// AlertChannel represents the notification channel for an alert.
type AlertChannel string

const (
	AlertChannelSlack AlertChannel = "slack"
	AlertChannelEmail AlertChannel = "email"
)

// AlertSeverity represents the severity level of a fired alert.
type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityInfo     AlertSeverity = "info"
)

// AlertRule defines a condition that triggers an alert when met.
type AlertRule struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	MetricName        string       `json:"metric_name"`
	Condition         string       `json:"condition,omitempty"`
	WarningThreshold  float64      `json:"warning_threshold"`
	CriticalThreshold float64      `json:"critical_threshold"`
	Threshold         float64      `json:"threshold,omitempty"`
	Channel           AlertChannel `json:"channel"`
	Enabled           bool         `json:"enabled"`
}

// Alert represents a fired alert instance.
type Alert struct {
	ID         string        `json:"id"`
	RuleID     string        `json:"rule_id"`
	Severity   AlertSeverity `json:"severity"`
	Message    string        `json:"message"`
	FiredAt    time.Time     `json:"fired_at"`
	ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
}
