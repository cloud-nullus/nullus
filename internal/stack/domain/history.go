package domain

import "time"

// StackVersion is a point-in-time snapshot of a stack's configuration.
type StackVersion struct {
	ID           string
	StackID      string
	Version      int
	Config       StackConfig
	ChangedBy    string
	ChangeReason string
	CreatedAt    time.Time
}

// ConfigDiff describes a single field change between two stack versions.
type ConfigDiff struct {
	Field    string
	OldValue string
	NewValue string
}
