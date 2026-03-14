package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateResources_BasicEstimate(t *testing.T) {
	uc := NewCalculateResources()

	out, err := uc.Execute(context.Background(), EstimateResourcesInput{
		Tools: []ToolInstance{
			{Name: "gitlab-ce", Instances: 1},
			{Name: "argocd", Instances: 1},
			{Name: "prometheus", Instances: 1},
			{Name: "grafana", Instances: 1},
			{Name: "minio", Instances: 1},
		},
		Workload: WorkloadInput{
			Developers:        10,
			ConcurrentRunners: 2,
			WeeklyCommits:     50,
			BuildFrequency:    "daily",
		},
	})

	require.NoError(t, err)
	assert.Greater(t, out.Summary.CPUCores, 0.0)
	assert.Greater(t, out.Summary.MemoryGi, 0.0)
	assert.Greater(t, out.Summary.StorageGi, 0.0)
	assert.Greater(t, out.Summary.MonthlyCostUSD, 0.0)
	assert.Len(t, out.PerTool, 5)
}

func TestCalculateResources_RunnerScaling(t *testing.T) {
	uc := NewCalculateResources()

	// Single runner
	single, err := uc.Execute(context.Background(), EstimateResourcesInput{
		Tools: []ToolInstance{
			{Name: "gitlab-runner", Instances: 1},
		},
		Workload: WorkloadInput{
			Developers: 5, ConcurrentRunners: 1, WeeklyCommits: 10, BuildFrequency: "on-push",
		},
	})
	require.NoError(t, err)

	// Four runners
	multi, err := uc.Execute(context.Background(), EstimateResourcesInput{
		Tools: []ToolInstance{
			{Name: "gitlab-runner", Instances: 4},
		},
		Workload: WorkloadInput{
			Developers: 5, ConcurrentRunners: 4, WeeklyCommits: 10, BuildFrequency: "on-push",
		},
	})
	require.NoError(t, err)

	assert.Greater(t, multi.Summary.CPUCores, single.Summary.CPUCores, "4 runners should require more CPU than 1")
	assert.Greater(t, multi.Summary.MemoryGi, single.Summary.MemoryGi, "4 runners should require more memory than 1")
	assert.NotEmpty(t, multi.Notes, "should have scaling notes for multiple runners")
}

func TestCalculateResources_BuildFrequencyScaling(t *testing.T) {
	uc := NewCalculateResources()

	base := EstimateResourcesInput{
		Tools: []ToolInstance{{Name: "gitlab-ce", Instances: 1}},
		Workload: WorkloadInput{
			Developers: 10, ConcurrentRunners: 2, WeeklyCommits: 20,
		},
	}

	base.Workload.BuildFrequency = "on-push"
	low, err := uc.Execute(context.Background(), base)
	require.NoError(t, err)

	base.Workload.BuildFrequency = "hourly"
	high, err := uc.Execute(context.Background(), base)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, high.Summary.CPUCores, low.Summary.CPUCores, "hourly build frequency should require >= CPU than on-push")
}

func TestCalculateResources_InvalidWorkload(t *testing.T) {
	uc := NewCalculateResources()

	tests := []struct {
		name    string
		input   WorkloadInput
		wantErr string
	}{
		{
			name:    "zero developers",
			input:   WorkloadInput{Developers: 0, ConcurrentRunners: 1, WeeklyCommits: 10, BuildFrequency: "daily"},
			wantErr: "developers",
		},
		{
			name:    "too many runners",
			input:   WorkloadInput{Developers: 1, ConcurrentRunners: 101, WeeklyCommits: 10, BuildFrequency: "daily"},
			wantErr: "concurrent_runners",
		},
		{
			name:    "invalid build frequency",
			input:   WorkloadInput{Developers: 1, ConcurrentRunners: 1, WeeklyCommits: 10, BuildFrequency: "weekly"},
			wantErr: "build_frequency",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), EstimateResourcesInput{
				Tools:    []ToolInstance{{Name: "argocd", Instances: 1}},
				Workload: tc.input,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestCalculateResources_ArtifactStorageFromCommits(t *testing.T) {
	uc := NewCalculateResources()

	lowCommits, err := uc.Execute(context.Background(), EstimateResourcesInput{
		Tools: []ToolInstance{{Name: "minio", Instances: 1}},
		Workload: WorkloadInput{
			Developers: 5, ConcurrentRunners: 1, WeeklyCommits: 1, BuildFrequency: "daily",
		},
	})
	require.NoError(t, err)

	highCommits, err := uc.Execute(context.Background(), EstimateResourcesInput{
		Tools: []ToolInstance{{Name: "minio", Instances: 1}},
		Workload: WorkloadInput{
			Developers: 5, ConcurrentRunners: 1, WeeklyCommits: 500, BuildFrequency: "daily",
		},
	})
	require.NoError(t, err)

	assert.Greater(t, highCommits.Summary.StorageGi, lowCommits.Summary.StorageGi,
		"more weekly commits should result in more storage")
}
