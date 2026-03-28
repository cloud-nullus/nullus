package handler

import (
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/stretchr/testify/assert"
)

func TestSelectedToolTypes_FallsBackToCanonicalNames(t *testing.T) {
	cfg := domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			SourceRepository: domain.ToolSelection{Enabled: true},
			StorageBackend:   domain.ToolSelection{Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CDTool: domain.ToolSelection{Enabled: true},
		},
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Enabled: true},
			Visualization: domain.ToolSelection{Enabled: true},
		},
		Logging: domain.LoggingConfig{
			Collection: domain.ToolSelection{Enabled: true},
			Search:     domain.ToolSelection{Enabled: true},
			TraceLayer: domain.ToolSelection{Enabled: true},
		},
	}

	items := selectedToolTypes(cfg)
	nameByKey := map[string]string{}
	for _, item := range items {
		nameByKey[item.Key] = item.Name
	}

	assert.Equal(t, "gitlab", nameByKey["source_repository"])
	assert.Equal(t, "argocd", nameByKey["cd_tool"])
	assert.Equal(t, "prometheus", nameByKey["collection"])
	assert.Equal(t, "grafana", nameByKey["visualization"])
	assert.Equal(t, "loki", nameByKey["logging_collection"])
	assert.Equal(t, "opensearch", nameByKey["logging_search"])
	assert.Equal(t, "tempo", nameByKey["trace_layer"])
	assert.Equal(t, "minio", nameByKey["storage_backend"])
}

func TestSelectedToolTypes_UsesConfiguredNameWhenPresent(t *testing.T) {
	cfg := domain.StackConfig{
		Logging: domain.LoggingConfig{
			Collection: domain.ToolSelection{Name: "Grafana Loki", Enabled: true},
			TraceLayer: domain.ToolSelection{Name: "jaeger", Enabled: true},
		},
	}

	items := selectedToolTypes(cfg)
	nameByKey := map[string]string{}
	for _, item := range items {
		nameByKey[item.Key] = item.Name
	}

	assert.Equal(t, "Grafana Loki", nameByKey["logging_collection"])
	assert.Equal(t, "jaeger", nameByKey["trace_layer"])
}
