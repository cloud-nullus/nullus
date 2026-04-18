package usecase

import (
	"context"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// ToolInstance specifies a tool and how many instances to run.
type ToolInstance struct {
	Name      string
	Instances int
}

// WorkloadInput describes the workload characteristics.
type WorkloadInput struct {
	Developers        int
	ConcurrentRunners int
	WeeklyCommits     int
	BuildFrequency    string // hourly, daily, on-push
}

// EstimateResourcesInput holds parameters for resource estimation.
type EstimateResourcesInput struct {
	Tools    []ToolInstance
	Workload WorkloadInput
}

// ToolResourceEstimate holds the per-tool resource breakdown.
type ToolResourceEstimate struct {
	Name      string
	Instances int
	CPUCores  float64
	MemoryGi  float64
	StorageGi float64
}

type ResourceCostBreakdown struct {
	CPUCostUSD     float64
	MemoryCostUSD  float64
	StorageCostUSD float64
}

// EstimateResourcesOutput holds the full resource estimation result.
type EstimateResourcesOutput struct {
	Summary             domain.ResourceEstimate
	PerTool             []ToolResourceEstimate
	Notes               []string
	WorkloadScaleFactor float64
	ArtifactStorageGi   float64
	CostBreakdown       ResourceCostBreakdown
}

// toolBaseline defines the per-instance baseline resource requirements.
type toolBaseline struct {
	CPUCores  float64
	MemoryGi  float64
	StorageGi float64
}

// toolBaselineMap maps tool names to their baseline resources per instance.
var toolBaselineMap = map[string]toolBaseline{
	"gitlab-ce":       {CPUCores: 4.0, MemoryGi: 8.0, StorageGi: 30.0},
	"gitlab-runner":   {CPUCores: 2.0, MemoryGi: 4.0, StorageGi: 10.0},
	"argocd":          {CPUCores: 2.0, MemoryGi: 3.0, StorageGi: 5.0},
	"prometheus":      {CPUCores: 1.0, MemoryGi: 4.0, StorageGi: 20.0},
	"grafana":         {CPUCores: 1.0, MemoryGi: 2.0, StorageGi: 5.0},
	"minio":           {CPUCores: 1.0, MemoryGi: 2.0, StorageGi: 50.0},
	"opentelemetry":   {CPUCores: 1.0, MemoryGi: 2.0, StorageGi: 0.0},
	"opensearch":      {CPUCores: 2.0, MemoryGi: 4.0, StorageGi: 30.0},
	"harbor":          {CPUCores: 2.0, MemoryGi: 4.0, StorageGi: 40.0},
	"gitlab-registry": {CPUCores: 0.5, MemoryGi: 1.0, StorageGi: 20.0},
	"cert-manager":    {CPUCores: 0.5, MemoryGi: 0.5, StorageGi: 0.0},
	"cnpg":            {CPUCores: 1.0, MemoryGi: 2.0, StorageGi: 10.0},
}

// costPerCPUPerMonth is an approximate AWS on-demand hourly rate scaled to monthly (USD).
const (
	costPerCPUPerMonth     = 12.0 // ~$0.016/hr per vCPU
	costPerGiBMemPerMonth  = 1.5  // ~$0.002/hr per GiB
	costPerGiBStorPerMonth = 0.10 // EBS gp3 ~$0.10/GiB/month
)

// CalculateResources computes estimated resource requirements for a tool set.
type CalculateResources struct{}

// NewCalculateResources constructs a CalculateResources use case.
func NewCalculateResources() *CalculateResources {
	return &CalculateResources{}
}

// Execute computes the resource estimate.
func (uc *CalculateResources) Execute(_ context.Context, input EstimateResourcesInput) (*EstimateResourcesOutput, error) {
	if err := validateWorkload(input.Workload); err != nil {
		return nil, err
	}

	scaleFactor := workloadScaleFactor(input.Workload)

	var (
		totalCPU     float64
		totalMemory  float64
		totalStorage float64
		perTool      []ToolResourceEstimate
		notes        []string
	)

	for _, ti := range input.Tools {
		if ti.Instances <= 0 {
			ti.Instances = 1
		}
		baseline, ok := toolBaselineMap[ti.Name]
		if !ok {
			// Unknown tool: use a minimal default
			baseline = toolBaseline{CPUCores: 0.5, MemoryGi: 1.0, StorageGi: 5.0}
		}

		cpu := baseline.CPUCores * float64(ti.Instances) * scaleFactor
		mem := baseline.MemoryGi * float64(ti.Instances) * scaleFactor
		stor := baseline.StorageGi * float64(ti.Instances)

		// Runner instances get extra note
		if ti.Name == "gitlab-runner" && ti.Instances > 1 {
			notes = append(notes, fmt.Sprintf(
				"GitLab Runner %d instances: CPU +%.1f, Memory +%.1fGi added vs baseline",
				ti.Instances,
				baseline.CPUCores*float64(ti.Instances-1)*scaleFactor,
				baseline.MemoryGi*float64(ti.Instances-1)*scaleFactor,
			))
		}

		totalCPU += cpu
		totalMemory += mem
		totalStorage += stor

		perTool = append(perTool, ToolResourceEstimate{
			Name:      ti.Name,
			Instances: ti.Instances,
			CPUCores:  roundTwo(cpu),
			MemoryGi:  roundTwo(mem),
			StorageGi: roundTwo(stor),
		})
	}

	// Add storage for build artifacts based on weekly commits
	artifactStorage := float64(input.Workload.WeeklyCommits) * 0.2 // ~0.2 GiB per commit/week
	totalStorage += artifactStorage
	if artifactStorage > 0 {
		notes = append(notes, fmt.Sprintf(
			"Weekly commits %d: +%.0fGi storage for build artifacts",
			input.Workload.WeeklyCommits, artifactStorage,
		))
	}

	cpuCost := totalCPU * costPerCPUPerMonth
	memoryCost := totalMemory * costPerGiBMemPerMonth
	storageCost := totalStorage * costPerGiBStorPerMonth
	monthlyCost := cpuCost + memoryCost + storageCost

	return &EstimateResourcesOutput{
		Summary: domain.ResourceEstimate{
			CPUCores:       roundTwo(totalCPU),
			MemoryGi:       roundTwo(totalMemory),
			StorageGi:      roundTwo(totalStorage),
			MonthlyCostUSD: roundTwo(monthlyCost),
		},
		PerTool:             perTool,
		Notes:               notes,
		WorkloadScaleFactor: roundTwo(scaleFactor),
		ArtifactStorageGi:   roundTwo(artifactStorage),
		CostBreakdown: ResourceCostBreakdown{
			CPUCostUSD:     roundTwo(cpuCost),
			MemoryCostUSD:  roundTwo(memoryCost),
			StorageCostUSD: roundTwo(storageCost),
		},
	}, nil
}

func validateWorkload(w WorkloadInput) error {
	if w.Developers < 1 || w.Developers > 10000 {
		return fmt.Errorf("developers must be between 1 and 10000")
	}
	if w.ConcurrentRunners < 1 || w.ConcurrentRunners > 100 {
		return fmt.Errorf("concurrent_runners must be between 1 and 100")
	}
	if w.WeeklyCommits < 1 || w.WeeklyCommits > 10000 {
		return fmt.Errorf("weekly_commits must be between 1 and 10000")
	}
	switch w.BuildFrequency {
	case "hourly", "daily", "on-push", "low", "medium", "high":
	default:
		return fmt.Errorf("build_frequency must be one of: hourly, daily, on-push")
	}
	return nil
}

// workloadScaleFactor returns a multiplier based on developer count and build frequency.
func workloadScaleFactor(w WorkloadInput) float64 {
	base := 1.0

	// Scale by developer count: every 10 devs adds 10%
	devMultiplier := 1.0 + float64(w.Developers)/100.0
	if devMultiplier > 3.0 {
		devMultiplier = 3.0
	}

	// Scale by build frequency
	freqMultiplier := 1.0
	switch w.BuildFrequency {
	case "hourly", "high":
		freqMultiplier = 1.3
	case "daily", "medium":
		freqMultiplier = 1.1
	case "on-push", "low":
		freqMultiplier = 1.0
	}

	return base * devMultiplier * freqMultiplier
}

// roundTwo rounds a float to 2 decimal places.
func roundTwo(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
