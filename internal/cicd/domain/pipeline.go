package domain

import "time"

// AppType represents the application type for a pipeline.
type AppType string

const (
	AppTypeWeb     AppType = "web"
	AppTypeBackend AppType = "backend"
	AppTypeBatch   AppType = "batch"
)

// PipelineStatus represents the status of a pipeline.
type PipelineStatus string

const (
	PipelineStatusActive   PipelineStatus = "active"
	PipelineStatusInactive PipelineStatus = "inactive"
)

// DeploymentStatus represents the status of a deployment run.
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusRunning    DeploymentStatus = "running"
	DeploymentStatusSuccess    DeploymentStatus = "success"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
)

// Pipeline represents a CI/CD pipeline configuration.
type Pipeline struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	TemplateID string         `json:"template_id"`
	OrgID      string         `json:"org_id"`
	ClusterID  string         `json:"cluster_id"`
	Namespace  string         `json:"namespace"`
	AppType    AppType        `json:"app_type"`
	GitRepoURL string         `json:"git_repo_url"`
	Status     PipelineStatus `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
}

// PipelineTemplate represents a reusable CI/CD pipeline template.
type PipelineTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AppType     AppType  `json:"app_type"`
	Stages      []string `json:"stages"`
	CreatedBy   string   `json:"created_by,omitempty"`
}

// Deployment represents a single deployment run of a pipeline.
type Deployment struct {
	ID          string           `json:"id"`
	PipelineID  string           `json:"pipeline_id"`
	Version     string           `json:"version"`
	Status      DeploymentStatus `json:"status"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	DeployedBy  string           `json:"deployed_by"`
}
