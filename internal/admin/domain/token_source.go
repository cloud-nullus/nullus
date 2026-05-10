package domain

import "time"

type TokenSource struct {
	ID               string
	OrgID            string
	Module           string
	Provider         string
	Path             string
	TokenType        string
	Status           string
	ExpiresAt        *time.Time
	LastRotatedAt    *time.Time
	NextCheckAt      *time.Time
	RequiresApproval bool
	Metadata         map[string]any
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TokenRotationEvent struct {
	ID            string
	TokenSourceID string
	EventType     string
	Result        string
	ReasonCode    string
	DetailJSON    map[string]any
	TraceID       string
	CreatedAt     time.Time
}
