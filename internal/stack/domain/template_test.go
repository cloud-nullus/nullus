package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplate_CreationWithAllFields(t *testing.T) {
	template := Template{
		ID:          "gp-enterprise-1",
		Name:        "Enterprise DevSecOps",
		Description: "GitLab + ArgoCD + Prometheus",
		Tools: []ToolConfig{
			{Category: "artifacts", Name: "GitLab", HelmVersion: "8.7.2", AppVersion: "17.7.2", Tool: "gitlab", Version: "17.7.2"},
			{Category: "pipeline", Name: "Argo CD", HelmVersion: "7.7.6", AppVersion: "2.13.2", Tool: "argocd", Version: "2.13.2"},
		},
		EstimatedInstallTime: 20 * time.Minute,
		RecommendedUseCase:   "Large teams with strict governance",
		MinResources:         "8 CPU / 16Gi memory / 100Gi storage",
		CreatedBy:            "admin@nullus.dev",
	}

	assert.Equal(t, "gp-enterprise-1", template.ID)
	assert.Equal(t, "Enterprise DevSecOps", template.Name)
	assert.Equal(t, "GitLab + ArgoCD + Prometheus", template.Description)
	require.Len(t, template.Tools, 2)
	assert.Equal(t, "GitLab", template.Tools[0].Name)
	assert.Equal(t, "argocd", template.Tools[1].Tool)
	assert.Equal(t, 20*time.Minute, template.EstimatedInstallTime)
	assert.Equal(t, "Large teams with strict governance", template.RecommendedUseCase)
	assert.Equal(t, "8 CPU / 16Gi memory / 100Gi storage", template.MinResources)
	assert.Equal(t, "admin@nullus.dev", template.CreatedBy)
}

func TestToolConfig_JSONSerialization(t *testing.T) {
	config := ToolConfig{
		Category:    "monitoring",
		Name:        "Prometheus",
		HelmVersion: "26.0.1",
		AppVersion:  "3.1.0",
		Tool:        "prometheus",
		Version:     "3.1.0",
	}

	bytes, err := json.Marshal(config)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(bytes, &payload)
	require.NoError(t, err)

	assert.Equal(t, "monitoring", payload["category"])
	assert.Equal(t, "Prometheus", payload["name"])
	assert.Equal(t, "26.0.1", payload["helm_version"])
	assert.Equal(t, "3.1.0", payload["app_version"])
	assert.Equal(t, "prometheus", payload["tool"])
	assert.Equal(t, "3.1.0", payload["version"])
}

func TestTemplate_ToolsSliceEmptyVsPopulated(t *testing.T) {
	emptyToolsTemplate := Template{ID: "t-empty", Tools: []ToolConfig{}}
	populatedToolsTemplate := Template{
		ID: "t-populated",
		Tools: []ToolConfig{
			{Category: "logging", Name: "OpenSearch", HelmVersion: "2.31.0", AppVersion: "2.18.0"},
		},
	}

	assert.Equal(t, "t-empty", emptyToolsTemplate.ID)
	assert.Equal(t, "t-populated", populatedToolsTemplate.ID)
	assert.Empty(t, emptyToolsTemplate.Tools)
	require.Len(t, populatedToolsTemplate.Tools, 1)
	assert.Equal(t, "OpenSearch", populatedToolsTemplate.Tools[0].Name)
	assert.Equal(t, "2.18.0", populatedToolsTemplate.Tools[0].AppVersion)
}

func TestTemplate_EstimatedInstallTimeDurationHandling(t *testing.T) {
	template := Template{EstimatedInstallTime: 45*time.Minute + 30*time.Second}

	assert.Equal(t, 45*time.Minute+30*time.Second, template.EstimatedInstallTime)
	assert.Equal(t, int64((45*time.Minute + 30*time.Second).Seconds()), int64(template.EstimatedInstallTime.Seconds()))
}
