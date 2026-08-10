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
