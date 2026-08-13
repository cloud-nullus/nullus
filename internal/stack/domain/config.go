package domain

import "encoding/json"

// StackConfig holds the full configuration for a DevSecOps stack.
type StackConfig struct {
	AccessDomain             string                        `json:"access_domain,omitempty"`
	AccessDomainTLS          *AccessDomainTLSConfig        `json:"access_domain_tls,omitempty"`
	Authentication           *AuthenticationConfig         `json:"authentication,omitempty"`
	YAMLOverrides            map[string]string             `json:"yaml_overrides,omitempty"`
	Artifacts                ArtifactsConfig               `json:"artifacts"`
	Pipeline                 PipelineConfig                `json:"pipeline"`
	Monitoring               MonitoringConfig              `json:"monitoring"`
	Logging                  LoggingConfig                 `json:"logging"`
	Resources                ResourcesConfig               `json:"resources"`
	OptionOverrides          map[string]map[string]float64 `json:"option_overrides,omitempty"`
	AppliedResourceOverrides map[string]ResourceVector     `json:"applied_resource_overrides,omitempty"`
	RowUnits                 map[string]PlanningRowUnit    `json:"row_units,omitempty"`
	Storage                  *StorageConfig                `json:"storage,omitempty"`
	SourceControl            *SourceControlConfig          `json:"source_control,omitempty"`
}

// SourceControlConfig 는 클러스터 밖 소스 저장소에 붙기 위한 설정이다.
//
// GitHub 처럼 우리가 설치하지 않는 SCM 을 고른 스택에서만 채워진다. 주소도
// 소유자도 우리가 정할 수 없으므로 설치 마법사에서 받아 둔다.
//
// 자격증명(PAT)은 여기 없다 — 배포 요청 본문으로만 흐르고 OpenBao 로 바로
// 들어간다. 이 구조는 stacks.config JSONB 로 평문 저장되므로 토큰을 넣으면
// DB 를 읽을 수 있는 누구에게나 노출된다.
type SourceControlConfig struct {
	// Owner 는 리포지토리가 만들어질 GitHub Organization 또는 사용자 계정이다.
	Owner string `json:"owner,omitempty"`
	// APIBaseURL 은 GitHub Enterprise Server 의 API 주소다.
	// 비면 github.com 을 쓴다.
	APIBaseURL string `json:"api_base_url,omitempty"`
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
	// StorageClass is the Kubernetes StorageClass used by every PVC the stack
	// creates. Empty means "use the cluster default StorageClass"; installs on
	// clusters without a default SC must set it explicitly or PVCs stay Pending.
	StorageClass string `json:"storage_class,omitempty"`
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
	// TraceExporter 는 애플리케이션이 보낸 텔레메트리를 받아 각 백엔드로
	// 흘려보내는 수집기다 (OpenTelemetry Collector).
	//
	// TraceLayer 와 칸을 따로 두는 이유는 역할이 다르기 때문이다 — TraceLayer 는
	// 추적을 저장·조회하는 백엔드(Tempo/Jaeger)이고, 이쪽은 그 앞에 서서 OTLP 를
	// 받아 추적·메트릭·로그를 각각의 저장소로 나눠 보낸다. 한 칸에 묶으면 둘 중
	// 하나만 설치할 수 있어 "수집기를 통해 관측한다"는 구성 자체가 불가능해진다.
	//
	// 프론트는 이전부터 이 값을 배포 요청에 실어 보냈으나 여기에 받는 칸이 없어
	// 조용히 버려졌다 — 설치 마법사의 Exporter/Agent 선택이 아무 일도 하지 않았다.
	TraceExporter ToolSelection `json:"trace_exporter,omitempty"`
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
