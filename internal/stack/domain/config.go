package domain

// StackConfig holds the full configuration for a DevSecOps stack.
type StackConfig struct {
	Artifacts  ArtifactsConfig  `json:"artifacts"`
	Pipeline   PipelineConfig   `json:"pipeline"`
	Monitoring MonitoringConfig `json:"monitoring"`
	Logging    LoggingConfig    `json:"logging"`
	Resources  ResourcesConfig  `json:"resources"`
}

// ArtifactsConfig holds tool selections for the artifacts step.
type ArtifactsConfig struct {
	PackageRegistry   ToolSelection `json:"package_registry"`
	SourceRepository  ToolSelection `json:"source_repository"`
	ContainerRegistry ToolSelection `json:"container_registry"`
	StorageBackend    ToolSelection `json:"storage_backend"`
}

// PipelineConfig holds tool selections for the pipeline step.
type PipelineConfig struct {
	CIPlatform ToolSelection `json:"ci_platform"`
	CDTool     ToolSelection `json:"cd_tool"`
}

// MonitoringConfig holds tool selections for the monitoring step.
type MonitoringConfig struct {
	Collection    ToolSelection `json:"collection"`
	Visualization ToolSelection `json:"visualization"`
}

// LoggingConfig holds tool selections for the logging step.
type LoggingConfig struct {
	Collection ToolSelection `json:"collection"`
	Search     ToolSelection `json:"search"`
}

// ResourcesConfig holds workload parameters and the calculated estimate.
type ResourcesConfig struct {
	DevCount          int             `json:"developers"`
	ConcurrentRunners int             `json:"concurrent_runners"`
	CommitsPerWeek    int             `json:"weekly_commits"`
	BuildFrequency    string          `json:"build_frequency"` // low/medium/high or hourly/daily/on-push
	Calculated        ResourceEstimate `json:"calculated,omitempty"`
}

// ToolSelection identifies a chosen tool by name and version.
type ToolSelection struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

// ResourceEstimate holds computed resource requirements.
type ResourceEstimate struct {
	CPUCores       float64 `json:"cpu_cores"`
	MemoryGi       float64 `json:"memory_gi"`
	StorageGi      float64 `json:"storage_gi"`
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
}
