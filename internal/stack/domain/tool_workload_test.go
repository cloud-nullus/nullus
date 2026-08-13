package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func workloadByKey(items []ToolWorkload, key string) (ToolWorkload, bool) {
	for _, item := range items {
		if item.Key == key {
			return item, true
		}
	}
	return ToolWorkload{}, false
}

func TestInstalledToolWorkloads_CoversEveryInstalledCategory(t *testing.T) {
	items := InstalledToolWorkloads(StackConfig{
		Artifacts: ArtifactsConfig{
			SourceRepository:  ToolSelection{Name: "GitLab CE", Version: "17.7.0", Enabled: true},
			ContainerRegistry: ToolSelection{Name: "Harbor", Version: "2.11.0", Enabled: true},
			StorageBackend:    ToolSelection{Name: "MinIO", Enabled: true},
		},
		Pipeline: PipelineConfig{
			CDTool: ToolSelection{Name: "Argo CD", Enabled: true},
		},
		Monitoring: MonitoringConfig{
			Collection:    ToolSelection{Name: "Prometheus", Enabled: true},
			Visualization: ToolSelection{Name: "Grafana", Enabled: true},
		},
	})

	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.Key)
	}

	assert.ElementsMatch(t, []string{
		"source_repository", "cd_tool", "collection",
		"visualization", "storage_backend", "container_registry",
	}, keys)

	harbor, ok := workloadByKey(items, "container_registry")
	require.True(t, ok)
	assert.Equal(t, "Harbor", harbor.Name)
	assert.Equal(t, "2.11.0", harbor.Version)
	assert.Contains(t, harbor.NamePrefixes, HarborReleaseName)
}

// 수집기는 추적 저장소와 별개의 워크로드다. 한 칸으로 묶으면 둘 중 하나가
// 모니터링에서 사라진다 — 파드가 떠 있어도 없는 것처럼 보인다.
func TestInstalledToolWorkloads_SeparatesTraceExporterFromTraceLayer(t *testing.T) {
	items := InstalledToolWorkloads(StackConfig{
		Logging: LoggingConfig{
			TraceLayer:    ToolSelection{Name: "tempo", Version: "2.7.0", Enabled: true},
			TraceExporter: ToolSelection{Name: "opentelemetry-collector", Version: "0.90.0", Enabled: true},
		},
	})

	layer, ok := workloadByKey(items, "trace_layer")
	require.True(t, ok)
	assert.Contains(t, layer.NamePrefixes, "tempo")

	exporter, ok := workloadByKey(items, "trace_exporter")
	require.True(t, ok, "OTel Collector 가 모니터링 대상에서 빠지면 안 된다")
	assert.Equal(t, "opentelemetry-collector", exporter.Name)
	assert.Equal(t, "0.90.0", exporter.Version)
	assert.Contains(t, exporter.NamePrefixes, OTelCollectorReleaseName)
}

// 로그 저장소로 Loki 를 고르면 파드도 loki-* 다. 접두사를 opensearch 로
// 고정해 두면 설치는 정상인데 화면에서만 0 파드 warning 으로 남는다.
func TestInstalledToolWorkloads_MatchesChosenLogStorePods(t *testing.T) {
	loki := InstalledToolWorkloads(StackConfig{
		Logging: LoggingConfig{Search: ToolSelection{Name: "loki", Enabled: true}},
	})
	search, ok := workloadByKey(loki, "logging_search")
	require.True(t, ok)
	assert.Equal(t, []string{"loki"}, search.NamePrefixes)

	opensearch := InstalledToolWorkloads(StackConfig{
		Logging: LoggingConfig{Search: ToolSelection{Name: "opensearch", Enabled: true}},
	})
	search, ok = workloadByKey(opensearch, "logging_search")
	require.True(t, ok)
	assert.Equal(t, []string{"opensearch"}, search.NamePrefixes)
}

// 수집과 검색이 같은 제품이면 릴리스도 하나다. 둘 다 세면 같은 파드를 두 번 센다.
func TestInstalledToolWorkloads_CountsSharedLogStoreOnce(t *testing.T) {
	items := InstalledToolWorkloads(StackConfig{
		Logging: LoggingConfig{
			Collection: ToolSelection{Name: "loki", Enabled: true},
			Search:     ToolSelection{Name: "loki", Enabled: true},
		},
	})

	_, ok := workloadByKey(items, "logging_collection")
	assert.True(t, ok)
	_, ok = workloadByKey(items, "logging_search")
	assert.False(t, ok, "같은 Loki 를 두 항목으로 세면 파드가 이중 계상된다")
}

func TestInstalledToolWorkloads_OmitsDisabledTraceExporter(t *testing.T) {
	items := InstalledToolWorkloads(StackConfig{
		Logging: LoggingConfig{
			TraceExporter: ToolSelection{Name: "opentelemetry-collector", Enabled: false},
		},
	})

	_, ok := workloadByKey(items, "trace_exporter")
	assert.False(t, ok)
}

func TestInstalledToolWorkloads_OmitsDisabledSelections(t *testing.T) {
	items := InstalledToolWorkloads(StackConfig{
		Artifacts: ArtifactsConfig{
			SourceRepository: ToolSelection{Name: "GitLab CE", Enabled: true},
			StorageBackend:   ToolSelection{Name: "MinIO", Enabled: false},
		},
	})

	_, ok := workloadByKey(items, "storage_backend")
	assert.False(t, ok, "disabled tools must not be reported as installed")
}

func TestInstalledToolWorkloads_FallsBackToCanonicalName(t *testing.T) {
	items := InstalledToolWorkloads(StackConfig{
		Pipeline: PipelineConfig{CDTool: ToolSelection{Enabled: true}},
	})

	cd, ok := workloadByKey(items, "cd_tool")
	require.True(t, ok)
	assert.Equal(t, "argocd", cd.Name)
	assert.Equal(t, "-", cd.Version, "missing version renders as a dash, not an empty string")
}

func TestInstalledToolWorkloads_DedupesNexusServingBothRegistryRoles(t *testing.T) {
	items := InstalledToolWorkloads(StackConfig{
		Artifacts: ArtifactsConfig{
			ContainerRegistry: ToolSelection{Name: "Nexus", Enabled: true},
			PackageRegistry:   ToolSelection{Name: "Nexus", Enabled: true},
		},
	})

	count := 0
	for _, item := range items {
		if item.Name == "Nexus" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestInstalledToolWorkloads_SkipsRegistriesWithoutOwnWorkload(t *testing.T) {
	tests := []struct {
		name     string
		registry ToolSelection
	}{
		{"external registry has no pods", ToolSelection{Name: "Harbor", Version: "external", Enabled: true}},
		{"gitlab builtin registry rides the gitlab chart", ToolSelection{Name: "GitLab Registry", Enabled: true}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := InstalledToolWorkloads(StackConfig{
				Artifacts: ArtifactsConfig{ContainerRegistry: tc.registry},
			})
			_, ok := workloadByKey(items, "container_registry")
			assert.False(t, ok)
		})
	}
}

func TestInstalledToolWorkloads_OpenBaoOnlyWhenSelectedAsProvider(t *testing.T) {
	withBao := InstalledToolWorkloads(StackConfig{
		Authentication: &AuthenticationConfig{Provider: "openbao"},
	})
	_, ok := workloadByKey(withBao, "authentication")
	assert.True(t, ok)

	withKeycloak := InstalledToolWorkloads(StackConfig{
		Authentication: &AuthenticationConfig{Provider: "keycloak"},
	})
	_, ok = workloadByKey(withKeycloak, "authentication")
	assert.False(t, ok, "only openbao is installed into the stack namespace")
}

func TestDecodeStackConfig(t *testing.T) {
	want := StackConfig{
		Artifacts: ArtifactsConfig{
			ContainerRegistry: ToolSelection{Name: "Harbor", Version: "2.11.0", Enabled: true},
		},
	}

	t.Run("typed value passes through", func(t *testing.T) {
		assert.Equal(t, want, DecodeStackConfig(want))
	})

	t.Run("pointer is dereferenced", func(t *testing.T) {
		assert.Equal(t, want, DecodeStackConfig(&want))
	})

	t.Run("jsonb map is decoded", func(t *testing.T) {
		raw := map[string]any{
			"artifacts": map[string]any{
				"container_registry": map[string]any{
					"name": "Harbor", "version": "2.11.0", "enabled": true,
				},
			},
		}
		assert.Equal(t, want, DecodeStackConfig(raw))
	})

	t.Run("nil and garbage degrade to empty", func(t *testing.T) {
		assert.Equal(t, StackConfig{}, DecodeStackConfig(nil))
		assert.Equal(t, StackConfig{}, DecodeStackConfig(make(chan int)))
	})
}
