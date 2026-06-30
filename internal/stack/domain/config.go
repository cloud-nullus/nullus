package domain

import "encoding/json"

// StackConfig holds the full configuration for a DevSecOps stack.
type StackConfig struct {
	AccessDomain    string                 `json:"access_domain,omitempty"`
	AccessDomainTLS *AccessDomainTLSConfig `json:"access_domain_tls,omitempty"`
	Authentication  *AuthenticationConfig  `json:"authentication,omitempty"`
	YAMLOverrides   map[string]string      `json:"yaml_overrides,omitempty"`
	Artifacts       ArtifactsConfig        `json:"artifacts"`
	Pipeline        PipelineConfig         `json:"pipeline"`
	Monitoring      MonitoringConfig       `json:"monitoring"`
	Logging         LoggingConfig          `json:"logging"`
	Resources       ResourcesConfig        `json:"resources"`
	OptionOverrides          map[string]map[string]float64 `json:"option_overrides,omitempty"`
	AppliedResourceOverrides map[string]ResourceVector     `json:"applied_resource_overrides,omitempty"`
	RowUnits                 map[string]PlanningRowUnit    `json:"row_units,omitempty"`
	Storage         *StorageConfig         `json:"storage,omitempty"`
}

type AuthenticationConfig struct {
	Provider string `json:"provider,omitempty"`
}

type AccessDomainTLSConfig struct {
	Enabled         bool   `json:"enabled"`
	SecretName      string `json:"secret_name,omitempty"`
	SecretNamespace string `json:"secret_namespace,omitempty"`
	IssuerName      string `json:"issuer_name,omitempty"`
}

type StorageConfig struct {
	PlanMode      string        `json:"plan_mode"`
	Database      StorageTarget `json:"database"`
	ObjectStorage StorageTarget `json:"object_storage"`
}

type StorageTarget struct {
	Mode             string  `json:"mode"`
	ExistingRef      string  `json:"existing_ref,omitempty"`
	Endpoint         string  `json:"endpoint,omitempty"`
	ResourceName     string  `json:"resource_name,omitempty"`
	AccessSecretRef  string  `json:"access_secret_ref,omitempty"`
	AuthID           string  `json:"auth_id,omitempty"`
	AuthPasswordKey  string  `json:"auth_password_key,omitempty"`
	ProviderOrEngine string  `json:"provider_or_engine,omitempty"`
	Version          string  `json:"version,omitempty"`
	Size             float64 `json:"size,omitempty"`
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
	TraceLayer ToolSelection `json:"trace_layer,omitempty"`
}

// ResourcesConfig holds workload parameters and the calculated estimate.
type ResourcesConfig struct {
	DevCount          int              `json:"developers"`
	ConcurrentRunners int              `json:"concurrent_runners"`
	CommitsPerWeek    int              `json:"weekly_commits"`
	BuildFrequency    string           `json:"build_frequency"` // low/medium/high or hourly/daily/on-push
	Calculated        ResourceEstimate `json:"calculated,omitempty"`
}

// ResourceVector captures per-OSS applied request/limit values.
type ResourceVector struct {
	CPURequest       float64 `json:"cpuRequest"`
	CPULimit         float64 `json:"cpuLimit"`
	MemoryRequestGi  float64 `json:"memoryRequestGi"`
	MemoryLimitGi    float64 `json:"memoryLimitGi"`
	StorageRequestGi float64 `json:"storageRequestGi"`
	StorageLimitGi   float64 `json:"storageLimitGi"`
}

// PlanningRowUnit preserves display units chosen in the installer.
type PlanningRowUnit struct {
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

// ToolSelection identifies a chosen tool by name and version.
type ToolSelection struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

func (t *ToolSelection) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*t = ToolSelection{}
		return nil
	}

	var legacyName string
	if err := json.Unmarshal(data, &legacyName); err == nil {
		*t = ToolSelection{
			Name:    legacyName,
			Enabled: legacyName != "",
		}
		return nil
	}

	type toolSelectionAlias ToolSelection
	var current toolSelectionAlias
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}

	*t = ToolSelection(current)
	return nil
}

// ResourceEstimate holds computed resource requirements.
type ResourceEstimate struct {
	CPUCores       float64 `json:"cpu_cores"`
	MemoryGi       float64 `json:"memory_gi"`
	StorageGi      float64 `json:"storage_gi"`
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
}
