package handler

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
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

func TestSelectedToolTypes_IncludesHarborContainerRegistry(t *testing.T) {
	items := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			ContainerRegistry: domain.ToolSelection{Name: "Harbor", Version: "2.11.0", Enabled: true},
		},
	})

	var harbor *selectedToolType
	for i := range items {
		if items[i].Key == "container_registry" {
			harbor = &items[i]
		}
	}

	if assert.NotNil(t, harbor, "container_registry must be monitored") {
		assert.Equal(t, "Harbor", harbor.Name)
		assert.Contains(t, harbor.PodNamePrefixes, "harbor")
	}
}

func TestSelectedToolTypes_IncludesNexusPackageRegistry(t *testing.T) {
	items := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			PackageRegistry: domain.ToolSelection{Name: "Nexus", Version: "3.70.0", Enabled: true},
		},
	})

	var nexus *selectedToolType
	for i := range items {
		if items[i].Key == "package_registry" {
			nexus = &items[i]
		}
	}

	if assert.NotNil(t, nexus, "package_registry must be monitored") {
		assert.Equal(t, "Nexus", nexus.Name)
		assert.Contains(t, nexus.PodNamePrefixes, "nexus")
	}
}

// Nexus 는 컨테이너 레지스트리와 패키지 저장소를 겸할 수 있다. 같은 파드를
// 가리키는 항목이 둘로 갈라지면 OSS 목록과 파드 커버리지가 이중 계상된다.
func TestSelectedToolTypes_DedupesNexusSelectedForBothRegistries(t *testing.T) {
	items := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			ContainerRegistry: domain.ToolSelection{Name: "Nexus", Version: "3.70.0", Enabled: true},
			PackageRegistry:   domain.ToolSelection{Name: "Nexus", Version: "3.70.0", Enabled: true},
		},
	})

	nexusCount := 0
	for _, item := range items {
		if strings.EqualFold(item.Name, "Nexus") {
			nexusCount++
		}
	}
	assert.Equal(t, 1, nexusCount, "Nexus must appear once even when it serves both registry roles")
}

// GitLab 내장 레지스트리는 gitlab-registry-* 파드로 뜬다. source_repository 의
// "gitlab" 접두사가 이미 이를 포함하므로 별도 항목을 만들면 중복이 된다.
func TestSelectedToolTypes_SkipsGitLabBuiltinRegistry(t *testing.T) {
	items := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			SourceRepository:  domain.ToolSelection{Name: "GitLab CE", Enabled: true},
			ContainerRegistry: domain.ToolSelection{Name: "GitLab Registry", Enabled: true},
		},
	})

	for _, item := range items {
		assert.NotEqual(t, "container_registry", item.Key,
			"GitLab builtin registry is already covered by the gitlab prefix")
	}
}

// 클러스터 밖 레지스트리는 파드가 없다. 항목을 만들면 영구히 0 파드 warning 이 된다.
func TestSelectedToolTypes_SkipsExternalRegistry(t *testing.T) {
	items := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			ContainerRegistry: domain.ToolSelection{Name: "Harbor", Version: "external", Enabled: true},
		},
	})

	for _, item := range items {
		assert.NotEqual(t, "container_registry", item.Key, "external registry must not be monitored")
	}
}

func TestFilterMonitoringToSelectedTools_IncludesHarborPods(t *testing.T) {
	types := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			ContainerRegistry: domain.ToolSelection{Name: "Harbor", Enabled: true},
		},
	})

	pods := []podMonitoringStatus{
		{Name: "harbor-core-56d9c6d84-abcde", Phase: "Running", Ready: true, CPURequestMillicores: 100, MemoryRequestMiB: 256},
		{Name: "harbor-registry-7f8b9c-xyz", Phase: "Running", Ready: true, CPURequestMillicores: 100, MemoryRequestMiB: 256},
		{Name: "opensearch-cluster-master-0", Phase: "Running", Ready: true},
	}

	filtered, _, summary := filterMonitoringToSelectedTools(types, pods)

	assert.Len(t, filtered, 2)
	assert.Equal(t, 2, summary.TotalPods)
	assert.Equal(t, int64(200), summary.CPURequestMillicores)
}

func TestToOSSStatuses_ReportsHarborAndNexusHealth(t *testing.T) {
	types := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			ContainerRegistry: domain.ToolSelection{Name: "Harbor", Enabled: true},
			PackageRegistry:   domain.ToolSelection{Name: "Nexus", Enabled: true},
		},
	})

	pods := []podMonitoringStatus{
		{Name: "harbor-core-56d9c6d84-abcde", Phase: "Running", Ready: true, Status: "running"},
		{Name: "nexus-nexus-repository-manager-0", Phase: "Running", Ready: false, Status: "warning"},
	}

	statusByName := map[string]ossMonitoringStatus{}
	for _, s := range toOSSStatuses(types, pods) {
		statusByName[s.Name] = s
	}

	if assert.Contains(t, statusByName, "Harbor") {
		assert.Equal(t, "running", statusByName["Harbor"].Status)
		assert.Equal(t, 1, statusByName["Harbor"].ReadyPods)
	}
	if assert.Contains(t, statusByName, "Nexus") {
		assert.Equal(t, "warning", statusByName["Nexus"].Status)
		assert.Equal(t, 1, statusByName["Nexus"].PodCount)
	}
}

func TestFilterInstalledResourcesToSelectedTools_KeepsHarborResources(t *testing.T) {
	types := selectedToolTypes(domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			ContainerRegistry: domain.ToolSelection{Name: "Harbor", Enabled: true},
		},
	})

	resources := []installedResourceStatus{
		{Name: "harbor-core", Kind: "Deployment"},
		{Name: "harbor-database", Kind: "StatefulSet"},
		{Name: "grafana", Kind: "Deployment"},
	}

	filtered := filterInstalledResourcesToSelectedTools(types, resources)
	assert.Len(t, filtered, 2)
}

func TestToOSSStatuses_ExcludesSucceededMigrationPodsFromHealth(t *testing.T) {
	types := []selectedToolType{
		{
			Key:                  "source_repository",
			Name:                 "gitlab",
			Version:              "18.5.1",
			Enabled:              true,
			PodNamePrefixes:      []string{"gitlab"},
			ResourceNamePrefixes: []string{"gitlab"},
		},
	}

	pods := []podMonitoringStatus{
		{Name: "gitlab-webservice-default-0", Phase: "Running", Ready: true, Status: "running"},
		{Name: "gitlab-migrations-a8d695e-fg7tp", Phase: "Succeeded", Ready: false, Status: "running"},
	}

	statuses := toOSSStatuses(types, pods)
	if assert.Len(t, statuses, 1) {
		assert.Equal(t, "running", statuses[0].Status)
		assert.Equal(t, 1, statuses[0].PodCount)
		assert.Equal(t, 1, statuses[0].ReadyPods)
		assert.Len(t, statuses[0].Pods, 1)
		assert.Equal(t, "gitlab-webservice-default-0", statuses[0].Pods[0].Name)
	}
}

// 도구 주소는 설치할 때 받은 접속 도메인에서 나온다. 화면이 도메인으로 주소를
// 다시 조립하면 스킴·호스트 규칙이 서버와 갈라지므로 서버가 확정해 내려준다.
func TestSelectedToolTypes_CarriesAccessURLFromAccessDomain(t *testing.T) {
	cfg := domain.StackConfig{
		AccessDomain: "nullus.local",
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Enabled: true},
			Visualization: domain.ToolSelection{Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CDTool: domain.ToolSelection{Enabled: true},
		},
	}

	urlByKey := map[string]string{}
	for _, item := range selectedToolTypes(cfg) {
		urlByKey[item.Key] = item.URL
	}

	assert.Equal(t, "https://grafana.nullus.local", urlByKey["visualization"])
	assert.Equal(t, "https://prometheus.nullus.local", urlByKey["collection"])
	assert.Equal(t, "https://argocd.nullus.local", urlByKey["cd_tool"])
}

func TestSelectedToolTypes_LeavesAccessURLBlankWithoutAccessDomain(t *testing.T) {
	cfg := domain.StackConfig{
		Monitoring: domain.MonitoringConfig{
			Visualization: domain.ToolSelection{Enabled: true},
		},
	}

	items := selectedToolTypes(cfg)
	require.Len(t, items, 1)
	assert.Empty(t, items[0].URL)
}

func TestToOSSStatuses_ExposesAccessURL(t *testing.T) {
	types := []selectedToolType{
		{Key: "visualization", Name: "grafana", Version: "11.5.1", Enabled: true,
			PodNamePrefixes: []string{"grafana"}, URL: "https://grafana.nullus.local"},
	}
	pods := []podMonitoringStatus{
		{Name: "grafana-5d9f", Phase: "Running", Ready: true, Status: "running"},
	}

	out := toOSSStatuses(types, pods)

	require.Len(t, out, 1)
	assert.Equal(t, "https://grafana.nullus.local", out[0].URL)
}
