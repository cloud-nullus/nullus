package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompatibilityMatrix_CreationWithValidFields(t *testing.T) {
	matrix := CompatibilityMatrix{
		ID:     "matrix-1",
		Name:   "GitLab + ArgoCD verified",
		Status: "verified",
		Kubernetes: KubernetesCompat{
			Min:         "1.26",
			Max:         "1.30",
			Recommended: "1.29",
		},
		Tools: map[string]ToolVersion{
			"ci": {Name: "gitlab", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
		},
	}

	assert.Equal(t, "matrix-1", matrix.ID)
	assert.Equal(t, "GitLab + ArgoCD verified", matrix.Name)
	assert.Equal(t, "verified", matrix.Status)
	assert.Equal(t, "1.26", matrix.Kubernetes.Min)
	assert.Equal(t, "1.30", matrix.Kubernetes.Max)
	assert.Equal(t, "1.29", matrix.Kubernetes.Recommended)

	tool, ok := matrix.Tools["ci"]
	require.True(t, ok)
	assert.Equal(t, "gitlab", tool.Name)
	assert.Equal(t, "8.7.2", tool.HelmVersion)
	assert.Equal(t, "17.7.2", tool.AppVersion)
}

func TestCompatibilityMatrix_ZeroValue(t *testing.T) {
	var matrix CompatibilityMatrix

	assert.Equal(t, "", matrix.ID)
	assert.Equal(t, "", matrix.Name)
	assert.Equal(t, "", matrix.Status)
	assert.Equal(t, "", matrix.Kubernetes.Min)
	assert.Equal(t, "", matrix.Kubernetes.Max)
	assert.Equal(t, "", matrix.Kubernetes.Recommended)
	assert.Nil(t, matrix.Tools)
}

func TestCompatibilityMatrix_ToolsMapAccessAndIteration(t *testing.T) {
	matrix := CompatibilityMatrix{
		Tools: map[string]ToolVersion{
			"ci":            {Name: "gitlab", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
			"cd":            {Name: "argocd", HelmVersion: "7.7.6", AppVersion: "2.13.2"},
			"monitoring":    {Name: "prometheus", HelmVersion: "26.0.1", AppVersion: "3.1.0"},
			"visualization": {Name: "grafana", HelmVersion: "8.7.1", AppVersion: "11.4.0"},
		},
	}

	cdTool, ok := matrix.Tools["cd"]
	require.True(t, ok)
	assert.Equal(t, "argocd", cdTool.Name)

	count := 0
	for category, version := range matrix.Tools {
		assert.NotEmpty(t, category)
		assert.NotEmpty(t, version.Name)
		count++
	}
	assert.Equal(t, 4, count)
}

func TestKubernetesCompat_VersionRangeFields(t *testing.T) {
	compat := KubernetesCompat{
		Min:         "1.27",
		Max:         "1.31",
		Recommended: "1.30",
	}

	assert.Equal(t, "1.27", compat.Min)
	assert.Equal(t, "1.31", compat.Max)
	assert.Equal(t, "1.30", compat.Recommended)
}

func TestToolVersion_Fields(t *testing.T) {
	version := ToolVersion{
		Name:        "opensearch",
		HelmVersion: "2.31.0",
		AppVersion:  "2.18.0",
	}

	assert.Equal(t, "opensearch", version.Name)
	assert.Equal(t, "2.31.0", version.HelmVersion)
	assert.Equal(t, "2.18.0", version.AppVersion)
}
