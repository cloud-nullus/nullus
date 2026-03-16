package domain

import "time"

// ToolConfig describes a tool included in a template.
type ToolConfig struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	HelmVersion string `json:"helm_version"`
	AppVersion  string `json:"app_version"`
}

// Template represents a Golden Path template for stack deployment.
type Template struct {
	ID                   string        `json:"id"`
	Name                 string        `json:"name"`
	Description          string        `json:"description"`
	Tools                []ToolConfig  `json:"tools"`
	EstimatedInstallTime time.Duration `json:"estimated_install_time"`
	RecommendedUseCase   string        `json:"recommended_use_case"`
	MinResources         string        `json:"min_resources"`
	CreatedBy            string        `json:"created_by,omitempty"`
}
