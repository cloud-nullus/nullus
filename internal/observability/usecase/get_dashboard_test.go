package usecase

import (
	"context"
	"testing"

	"github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDashboard_ReturnsData(t *testing.T) {
	repo := repository.NewMemoryDashboardRepository()
	uc := NewGetDashboard(repo)

	out, err := uc.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Dashboard)
}

func TestGetDashboard_ClusterMetricsValid(t *testing.T) {
	repo := repository.NewMemoryDashboardRepository()
	uc := NewGetDashboard(repo)

	out, err := uc.Execute(context.Background())
	require.NoError(t, err)

	cm := out.Dashboard.ClusterMetrics
	assert.Greater(t, cm.CPUUsage, 0.0, "cpu usage should be > 0")
	assert.LessOrEqual(t, cm.CPUUsage, 100.0, "cpu usage should be <= 100")
	assert.Greater(t, cm.MemoryUsage, 0.0, "memory usage should be > 0")
	assert.Greater(t, cm.PodCount, 0, "pod count should be > 0")
}

func TestGetDashboard_PipelineMetricsValid(t *testing.T) {
	repo := repository.NewMemoryDashboardRepository()
	uc := NewGetDashboard(repo)

	out, err := uc.Execute(context.Background())
	require.NoError(t, err)

	pm := out.Dashboard.PipelineMetrics
	assert.Greater(t, pm.TotalRuns, 0, "total runs should be > 0")
	assert.Greater(t, pm.SuccessRate, 0.0, "success rate should be > 0")
	assert.LessOrEqual(t, pm.SuccessRate, 100.0, "success rate should be <= 100")
	assert.Greater(t, pm.AvgBuildTime, 0.0, "avg build time should be > 0")
}

func TestGetDashboard_ToolHealthNotEmpty(t *testing.T) {
	repo := repository.NewMemoryDashboardRepository()
	uc := NewGetDashboard(repo)

	out, err := uc.Execute(context.Background())
	require.NoError(t, err)

	assert.NotEmpty(t, out.Dashboard.ToolHealthList, "tool health list should not be empty")
	for _, tool := range out.Dashboard.ToolHealthList {
		assert.NotEmpty(t, tool.Name, "tool name should not be empty")
		assert.NotEmpty(t, tool.Status, "tool status should not be empty")
		assert.NotEmpty(t, tool.Version, "tool version should not be empty")
	}
}
