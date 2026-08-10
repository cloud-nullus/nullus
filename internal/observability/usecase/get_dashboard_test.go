package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/observability/adapter/repository"
	"github.com/cloud-nullus/draft/internal/observability/domain"
)

func TestGetDashboard_ReturnsData(t *testing.T) {
	repo := repository.NewMemoryDashboardRepository()
	uc := NewGetDashboard(repo)

	out, err := uc.Execute(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Dashboard)
}

func TestGetDashboard_ClusterMetricsValid(t *testing.T) {
	repo := repository.NewMemoryDashboardRepository()
	uc := NewGetDashboard(repo)

	out, err := uc.Execute(context.Background(), "org-1")
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

	out, err := uc.Execute(context.Background(), "org-1")
	require.NoError(t, err)

	pm := out.Dashboard.PipelineMetrics
	assert.Greater(t, pm.TotalRuns, 0, "total runs should be > 0")
	assert.Greater(t, pm.SuccessRate, 0.0, "success rate should be > 0")
	assert.LessOrEqual(t, pm.SuccessRate, 100.0, "success rate should be <= 100")
	assert.Greater(t, pm.AvgBuildTime, 0.0, "avg build time should be > 0")
}

type stubToolHealth struct {
	tools []domain.ToolHealth
	err   error
}

func (s *stubToolHealth) ListToolHealth(_ context.Context, _ string) ([]domain.ToolHealth, error) {
	return s.tools, s.err
}

func TestGetDashboard_LiveToolHealthReplacesSimulatedList(t *testing.T) {
	uc := NewGetDashboard(
		repository.NewMemoryDashboardRepository(),
		WithToolHealth(&stubToolHealth{tools: []domain.ToolHealth{
			{Name: "Harbor", Status: "running", Version: "2.11.0"},
		}}),
	)

	out, err := uc.Execute(context.Background(), "org-1")
	require.NoError(t, err)

	require.Len(t, out.Dashboard.ToolHealthList, 1)
	assert.Equal(t, "Harbor", out.Dashboard.ToolHealthList[0].Name)
}

// 실측에 실패했을 때 시뮬레이션 목록으로 되돌아가면, 죽은 도구가 running 으로
// 보이는 최악의 오답이 된다. 차라리 비워서 "모른다" 를 드러낸다.
func TestGetDashboard_ToolHealthFailureDoesNotFallBackToSimulatedData(t *testing.T) {
	uc := NewGetDashboard(
		repository.NewMemoryDashboardRepository(),
		WithToolHealth(&stubToolHealth{err: errors.New("cluster unreachable")}),
	)

	out, err := uc.Execute(context.Background(), "org-1")
	require.NoError(t, err, "the dashboard still renders without tool health")
	assert.Empty(t, out.Dashboard.ToolHealthList)
}

func TestGetDashboard_ToolHealthNotEmpty(t *testing.T) {
	repo := repository.NewMemoryDashboardRepository()
	uc := NewGetDashboard(repo)

	out, err := uc.Execute(context.Background(), "org-1")
	require.NoError(t, err)

	assert.NotEmpty(t, out.Dashboard.ToolHealthList, "tool health list should not be empty")
	for _, tool := range out.Dashboard.ToolHealthList {
		assert.NotEmpty(t, tool.Name, "tool name should not be empty")
		assert.NotEmpty(t, tool.Status, "tool status should not be empty")
		assert.NotEmpty(t, tool.Version, "tool version should not be empty")
	}
}
