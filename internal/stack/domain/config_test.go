package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStackConfig_ZeroValue(t *testing.T) {
	var cfg StackConfig
	// Zero value should be valid Go struct with empty fields
	assert.Equal(t, "", cfg.Artifacts.PackageRegistry.Name)
	assert.Equal(t, "", cfg.Pipeline.CDTool.Version)
	assert.Equal(t, false, cfg.Logging.Collection.Enabled)
}

func TestStackConfig_ToolSelection(t *testing.T) {
	cfg := StackConfig{
		Artifacts: ArtifactsConfig{
			PackageRegistry: ToolSelection{Name: "gitlab", Version: "17.7.2", Enabled: true},
			SourceRepository: ToolSelection{Name: "gitlab", Version: "17.7.2", Enabled: true},
			ContainerRegistry: ToolSelection{Name: "gitlab-registry", Version: "17.7.2", Enabled: true},
			StorageBackend: ToolSelection{Name: "minio", Version: "2024.11.7", Enabled: true},
		},
		Pipeline: PipelineConfig{
			CIPlatform: ToolSelection{Name: "gitlab-ci", Version: "17.7.2", Enabled: true},
			CDTool:     ToolSelection{Name: "argocd", Version: "2.13.2", Enabled: true},
		},
		Monitoring: MonitoringConfig{
			Collection:    ToolSelection{Name: "prometheus", Version: "3.1.0", Enabled: true},
			Visualization: ToolSelection{Name: "grafana", Version: "11.4.0", Enabled: true},
		},
		Logging: LoggingConfig{
			Collection: ToolSelection{Name: "opentelemetry", Version: "0.115.0", Enabled: true},
			Search:     ToolSelection{Name: "opensearch", Version: "2.18.0", Enabled: true},
		},
		Resources: ResourcesConfig{
			DevCount:          20,
			ConcurrentRunners: 5,
			CommitsPerWeek:    100,
			BuildFrequency:    "hourly",
		},
	}

	assert.Equal(t, "gitlab", cfg.Artifacts.PackageRegistry.Name)
	assert.Equal(t, "17.7.2", cfg.Artifacts.PackageRegistry.Version)
	assert.True(t, cfg.Artifacts.PackageRegistry.Enabled)

	assert.Equal(t, "argocd", cfg.Pipeline.CDTool.Name)
	assert.Equal(t, "2.13.2", cfg.Pipeline.CDTool.Version)

	assert.Equal(t, "prometheus", cfg.Monitoring.Collection.Name)
	assert.Equal(t, "grafana", cfg.Monitoring.Visualization.Name)

	assert.Equal(t, "opentelemetry", cfg.Logging.Collection.Name)
	assert.Equal(t, "opensearch", cfg.Logging.Search.Name)

	assert.Equal(t, 20, cfg.Resources.DevCount)
	assert.Equal(t, 5, cfg.Resources.ConcurrentRunners)
	assert.Equal(t, 100, cfg.Resources.CommitsPerWeek)
	assert.Equal(t, "hourly", cfg.Resources.BuildFrequency)
}

func TestResourceEstimate_Fields(t *testing.T) {
	est := ResourceEstimate{
		CPUCores:       8.0,
		MemoryGi:       16.0,
		StorageGi:      100.0,
		MonthlyCostUSD: 187.50,
	}

	assert.Equal(t, 8.0, est.CPUCores)
	assert.Equal(t, 16.0, est.MemoryGi)
	assert.Equal(t, 100.0, est.StorageGi)
	assert.Equal(t, 187.50, est.MonthlyCostUSD)
}
