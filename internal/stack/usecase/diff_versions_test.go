package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestDiffVersions_Execute_AddedRemovedChanged(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	seedVersion(t, repo, "stack-1", "v1", 1, domain.StackConfig{
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Name: "GitLab CI", Version: "17.7.0", Enabled: true},
		},
		Resources: domain.ResourcesConfig{
			DevCount: 4,
		},
	})
	seedVersion(t, repo, "stack-1", "v2", 2, domain.StackConfig{
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Name: "GitLab CI", Version: "17.8.0", Enabled: true},
		},
		Resources: domain.ResourcesConfig{
			DevCount: 6,
			Calculated: domain.ResourceEstimate{
				CPUCores: 8,
			},
		},
	})

	uc := NewDiffVersions(repo)
	out, err := uc.Execute(context.Background(), DiffVersionsInput{
		StackID:  "stack-1",
		VersionA: 1,
		VersionB: 2,
	})

	require.NoError(t, err)
	require.Contains(t, out.Added, "resources.calculated.cpu_cores", "added keys: %#v", out.Added)
	assert.Equal(t, float64(8), out.Added["resources.calculated.cpu_cores"])
	assert.NotContains(t, out.Removed, "resources.calculated.cpu_cores")
	assert.Equal(t, [2]any{"17.7.0", "17.8.0"}, out.Changed["pipeline.ci_platform.version"])
	assert.Equal(t, [2]any{float64(4), float64(6)}, out.Changed["resources.developers"])
}

func TestDiffVersions_Execute_RemovedKeys(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	seedVersion(t, repo, "stack-4", "v1", 1, domain.StackConfig{
		Resources: domain.ResourcesConfig{
			DevCount: 8,
			Calculated: domain.ResourceEstimate{
				StorageGi: 120,
			},
		},
	})
	seedVersion(t, repo, "stack-4", "v2", 2, domain.StackConfig{
		Resources: domain.ResourcesConfig{
			DevCount: 8,
		},
	})

	uc := NewDiffVersions(repo)
	out, err := uc.Execute(context.Background(), DiffVersionsInput{
		StackID:  "stack-4",
		VersionA: 1,
		VersionB: 2,
	})

	require.NoError(t, err)
	assert.Equal(t, float64(120), out.Removed["resources.calculated.storage_gi"])
	assert.NotContains(t, out.Added, "resources.calculated.storage_gi")
	assert.Empty(t, out.Changed)
}

func TestDiffVersions_Execute_HandlesNestedObjects(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	seedVersion(t, repo, "stack-2", "v1", 1, domain.StackConfig{
		Monitoring: domain.MonitoringConfig{
			Collection: domain.ToolSelection{Name: "Prometheus", Version: "2.50.0", Enabled: true},
		},
	})
	seedVersion(t, repo, "stack-2", "v2", 2, domain.StackConfig{
		Monitoring: domain.MonitoringConfig{
			Collection: domain.ToolSelection{Name: "VictoriaMetrics", Version: "1.100.0", Enabled: true},
		},
	})

	uc := NewDiffVersions(repo)
	out, err := uc.Execute(context.Background(), DiffVersionsInput{
		StackID:  "stack-2",
		VersionA: 1,
		VersionB: 2,
	})

	require.NoError(t, err)
	assert.Equal(t, [2]any{"Prometheus", "VictoriaMetrics"}, out.Changed["monitoring.collection.name"])
	assert.Equal(t, [2]any{"2.50.0", "1.100.0"}, out.Changed["monitoring.collection.version"])
}

func TestDiffVersions_Execute_IdenticalConfigReturnsEmptyDiff(t *testing.T) {
	repo := repository.NewMemoryHistoryRepository()
	cfg := domain.StackConfig{
		Logging: domain.LoggingConfig{
			Search: domain.ToolSelection{Name: "Loki", Version: "2.9.0", Enabled: true},
		},
	}
	seedVersion(t, repo, "stack-3", "v1", 1, cfg)
	seedVersion(t, repo, "stack-3", "v2", 2, cfg)

	uc := NewDiffVersions(repo)
	out, err := uc.Execute(context.Background(), DiffVersionsInput{
		StackID:  "stack-3",
		VersionA: 1,
		VersionB: 2,
	})

	require.NoError(t, err)
	assert.Empty(t, out.Added)
	assert.Empty(t, out.Removed)
	assert.Empty(t, out.Changed)
}

func seedVersion(t *testing.T, repo *repository.MemoryHistoryRepository, stackID, versionID string, version int, cfg domain.StackConfig) {
	t.Helper()
	err := repo.SaveVersion(context.Background(), &domain.StackVersion{
		ID:        versionID,
		StackID:   stackID,
		Version:   version,
		Config:    cfg,
		ChangedBy: "tester",
		CreatedAt: time.Now(),
	})
	require.NoError(t, err)
}
