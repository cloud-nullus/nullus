package usecase

import (
	"context"
	"testing"
)

func BenchmarkCalculateResources(b *testing.B) {
	uc := NewCalculateResources()
	input := EstimateResourcesInput{
		Tools: []ToolInstance{
			{Name: "gitlab-ce", Instances: 1},
			{Name: "gitlab-runner", Instances: 4},
			{Name: "argocd", Instances: 1},
			{Name: "prometheus", Instances: 1},
			{Name: "grafana", Instances: 1},
		},
		Workload: WorkloadInput{
			Developers:        20,
			ConcurrentRunners: 4,
			WeeklyCommits:     50,
			BuildFrequency:    "medium",
		},
	}
	for i := 0; i < b.N; i++ {
		uc.Execute(context.Background(), input)
	}
}
