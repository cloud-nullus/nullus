package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackVersion_CreationWithAllFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	version := StackVersion{
		ID:      "ver-1",
		StackID: "stack-1",
		Version: 3,
		Config: StackConfig{
			Pipeline: PipelineConfig{
				CDTool: ToolSelection{Name: "argocd", Version: "2.13.2", Enabled: true},
			},
		},
		ChangedBy:    "devops@nullus.dev",
		ChangeReason: "Upgrade CD tool",
		CreatedAt:    now,
	}

	assert.Equal(t, "ver-1", version.ID)
	assert.Equal(t, "stack-1", version.StackID)
	assert.Equal(t, 3, version.Version)
	assert.Equal(t, "argocd", version.Config.Pipeline.CDTool.Name)
	assert.Equal(t, "2.13.2", version.Config.Pipeline.CDTool.Version)
	assert.Equal(t, "devops@nullus.dev", version.ChangedBy)
	assert.Equal(t, "Upgrade CD tool", version.ChangeReason)
	assert.True(t, version.CreatedAt.Equal(now))
}

func TestStackVersion_ZeroValue(t *testing.T) {
	var version StackVersion

	assert.Equal(t, "", version.ID)
	assert.Equal(t, "", version.StackID)
	assert.Equal(t, 0, version.Version)
	assert.Equal(t, "", version.Config.Pipeline.CDTool.Name)
	assert.Equal(t, "", version.ChangedBy)
	assert.Equal(t, "", version.ChangeReason)
	assert.True(t, version.CreatedAt.IsZero())
}

func TestStackVersion_ConfigHoldsValidStackConfig(t *testing.T) {
	config := StackConfig{
		Artifacts: ArtifactsConfig{
			PackageRegistry: ToolSelection{Name: "gitlab", Version: "17.7.2", Enabled: true},
		},
		Pipeline: PipelineConfig{
			CDTool: ToolSelection{Name: "argocd", Version: "2.13.2", Enabled: true},
		},
		Resources: ResourcesConfig{DevCount: 15, BuildFrequency: "daily"},
	}

	version := StackVersion{Config: config}

	require.NotNil(t, &version.Config)
	assert.Equal(t, "gitlab", version.Config.Artifacts.PackageRegistry.Name)
	assert.Equal(t, "argocd", version.Config.Pipeline.CDTool.Name)
	assert.Equal(t, 15, version.Config.Resources.DevCount)
	assert.Equal(t, "daily", version.Config.Resources.BuildFrequency)
}

func TestConfigDiff_Fields(t *testing.T) {
	diff := ConfigDiff{
		Field:    "pipeline.cd_tool.version",
		OldValue: "2.12.0",
		NewValue: "2.13.2",
	}

	assert.Equal(t, "pipeline.cd_tool.version", diff.Field)
	assert.Equal(t, "2.12.0", diff.OldValue)
	assert.Equal(t, "2.13.2", diff.NewValue)
}
