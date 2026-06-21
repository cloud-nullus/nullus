package domain

import "time"

// HelmStepMetadata stores the DB-backed Helm chart configuration for a stack step.
type HelmStepMetadata struct {
	StepName    string    `json:"step_name"`
	ReleaseName string    `json:"release_name,omitempty"`
	ChartName   string    `json:"chart_name"`
	RepoURL     string    `json:"repo_url,omitempty"`
	Version     string    `json:"version,omitempty"`
	Namespace   string    `json:"namespace,omitempty"`
	Phase       string    `json:"phase,omitempty"`
	SortOrder   int       `json:"sort_order"`
	Wait        bool      `json:"wait"`
	IsEnabled   bool      `json:"is_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
