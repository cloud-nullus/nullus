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
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Condition string       `json:"condition"`
	Threshold float64      `json:"threshold"`
	Channel   AlertChannel `json:"channel"`
	Enabled   bool         `json:"enabled"`
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
