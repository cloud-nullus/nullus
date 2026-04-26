package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboard_ConstructWithNestedMetrics(t *testing.T) {
	dashboard := Dashboard{
		ClusterMetrics: ClusterMetrics{
			CPUUsage:     45.5,
			MemoryUsage:  60.2,
			StorageUsage: 37.8,
			PodCount:     120,
		},
		PipelineMetrics: PipelineMetrics{
			TotalRuns:    312,
			SuccessRate:  96.1,
			AvgBuildTime: 185.4,
		},
		ToolHealthList: []ToolHealth{
			{Name: "Prometheus", Status: "running", Version: "3.1.0"},
			{Name: "Grafana", Status: "warning", Version: "11.4.0"},
		},
	}

	assert.Equal(t, 45.5, dashboard.ClusterMetrics.CPUUsage)
	assert.Equal(t, 60.2, dashboard.ClusterMetrics.MemoryUsage)
	assert.Equal(t, 37.8, dashboard.ClusterMetrics.StorageUsage)
	assert.Equal(t, 120, dashboard.ClusterMetrics.PodCount)

	assert.Equal(t, 312, dashboard.PipelineMetrics.TotalRuns)
	assert.Equal(t, 96.1, dashboard.PipelineMetrics.SuccessRate)
	assert.Equal(t, 185.4, dashboard.PipelineMetrics.AvgBuildTime)

	assert.Len(t, dashboard.ToolHealthList, 2)
	assert.Equal(t, "Prometheus", dashboard.ToolHealthList[0].Name)
	assert.Equal(t, "warning", dashboard.ToolHealthList[1].Status)
}

func TestToolHealth_ConstructWithExpectedFields(t *testing.T) {
	tool := ToolHealth{
		Name:    "Argo CD",
		Status:  "running",
		Version: "2.13.2",
	}

	assert.Equal(t, "Argo CD", tool.Name)
	assert.Equal(t, "running", tool.Status)
	assert.Equal(t, "2.13.2", tool.Version)
}
