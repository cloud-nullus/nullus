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

func TestFilterMonitoringToSelectedTools_KeepsOnlyEnabledToolPods(t *testing.T) {
	types := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			SourceRepository: domain.ToolSelection{Name: "gitlab", Enabled: true},
		},
		Monitoring: domain.MonitoringConfig{
			Visualization: domain.ToolSelection{Name: "grafana", Enabled: true},
		},
	})

	pods := []podMonitoringStatus{
		{Name: "gitlab-webservice-default-0", Phase: "Running", Ready: true, CPURequestMillicores: 100, CPULimitMillicores: 200, MemoryRequestMiB: 128, MemoryLimitMiB: 256, StorageRequestGiB: 1, StorageLimitGiB: 2, StorageUsageGiB: 0.5},
		{Name: "grafana-7d4f6f8f8f-abcde", Phase: "Running", Ready: false, CPURequestMillicores: 50, CPULimitMillicores: 100, MemoryRequestMiB: 64, MemoryLimitMiB: 128},
		{Name: "opensearch-cluster-master-0", Phase: "Running", Ready: true, CPURequestMillicores: 300, CPULimitMillicores: 600, MemoryRequestMiB: 512, MemoryLimitMiB: 1024},
	}

	filteredPods, counts, summary := filterMonitoringToSelectedTools(types, pods)

	if assert.Len(t, filteredPods, 2) {
		assert.Equal(t, "gitlab-webservice-default-0", filteredPods[0].Name)
		assert.Equal(t, "grafana-7d4f6f8f8f-abcde", filteredPods[1].Name)
	}
	assert.Equal(t, 2, summary.TotalPods)
	assert.Equal(t, 1, summary.ReadyPods)
	assert.Equal(t, int64(150), summary.CPURequestMillicores)
	assert.Equal(t, int64(300), summary.CPULimitMillicores)
	assert.Equal(t, int64(192), summary.MemoryRequestMiB)
	assert.Equal(t, int64(384), summary.MemoryLimitMiB)
	assert.Equal(t, int64(1), summary.StorageRequestGiB)
	assert.Equal(t, int64(2), summary.StorageLimitGiB)
	assert.Equal(t, 0.5, summary.StorageUsageGiB)
	if assert.Len(t, counts, 1) {
		assert.Equal(t, "Running", counts[0].Name)
		assert.Equal(t, 2, counts[0].Count)
	}
}

func TestFilterInstalledResourcesToSelectedTools_KeepsOnlySelectedPrefixes(t *testing.T) {
	types := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			SourceRepository: domain.ToolSelection{Name: "gitlab", Enabled: true},
		},
		Monitoring: domain.MonitoringConfig{
			Visualization: domain.ToolSelection{Name: "grafana", Enabled: true},
		},
	})

	resources := []installedResourceStatus{
		{Name: "gitlab-webservice-default", Kind: "Deployment"},
		{Name: "grafana", Kind: "Deployment"},
		{Name: "opensearch-cluster-master", Kind: "StatefulSet"},
	}

	filtered := filterInstalledResourcesToSelectedTools(types, resources)

	if assert.Len(t, filtered, 2) {
		assert.Equal(t, "gitlab-webservice-default", filtered[0].Name)
		assert.Equal(t, "grafana", filtered[1].Name)
	}
}
