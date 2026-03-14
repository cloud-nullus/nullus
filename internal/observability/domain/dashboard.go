package domain

// ClusterMetrics holds resource utilization metrics for a cluster.
type ClusterMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`    // percentage 0-100
	MemoryUsage float64 `json:"memory_usage"` // percentage 0-100
	StorageUsage float64 `json:"storage_usage"` // percentage 0-100
	PodCount    int     `json:"pod_count"`
}

// PipelineMetrics holds CI/CD pipeline execution statistics.
type PipelineMetrics struct {
	TotalRuns    int     `json:"total_runs"`
	SuccessRate  float64 `json:"success_rate"` // percentage 0-100
	AvgBuildTime float64 `json:"avg_build_time_seconds"`
}

// ToolHealth represents the health status of a DevOps tool.
type ToolHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // running, warning, error
	Version string `json:"version"`
}

// Dashboard aggregates all observability data for the platform overview.
type Dashboard struct {
	ClusterMetrics  ClusterMetrics  `json:"cluster_metrics"`
	PipelineMetrics PipelineMetrics `json:"pipeline_metrics"`
	ToolHealthList  []ToolHealth    `json:"tool_health"`
}
