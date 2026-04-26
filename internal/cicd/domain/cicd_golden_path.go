package domain

import "time"

// CICDGoldenPath represents a verified CI/CD tool combination (Golden Path).
type CICDGoldenPath struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	Description          string     `json:"description"`
	Tools                []CICDTool `json:"tools"`
	EstimatedInstallTime int        `json:"estimated_install_time"` // in minutes
	RecommendedUseCase   string     `json:"recommended_use_case"`
	MinResources         string     `json:"min_resources"`
	CreatedAt            time.Time  `json:"created_at"`
}

// CICDTool represents a tool in the CI/CD stack.
type CICDTool struct {
	Category    string `json:"category"` // ci_platform, cd_tool, monitoring, etc.
	Name        string `json:"name"`     // GitLab CI, ArgoCD, Prometheus, etc.
	HelmVersion string `json:"helm_version"`
	AppVersion  string `json:"app_version"`
}
