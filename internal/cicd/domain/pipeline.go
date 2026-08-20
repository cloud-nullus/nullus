package domain

import (
	"strings"
	"time"
)

// AppType represents the application type for a pipeline.
type AppType string

const (
	AppTypeWeb     AppType = "web"
	AppTypeBackend AppType = "backend"
	AppTypeBatch   AppType = "batch"
)

// PipelineStatus represents the status of a pipeline.
type PipelineStatus string

const (
	PipelineStatusActive   PipelineStatus = "active"
	PipelineStatusInactive PipelineStatus = "inactive"
)

// DeploymentStatus represents the status of a deployment run.
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusRunning    DeploymentStatus = "running"
	DeploymentStatusSuccess    DeploymentStatus = "success"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
)

// Pipeline represents a CI/CD pipeline configuration.
type Pipeline struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	ExecutionMode  string            `json:"execution_mode,omitempty"`
	TemplateID     string            `json:"template_id"`
	OrgID          string            `json:"org_id"`
	ClusterID      string            `json:"cluster_id"`
	StackID        string            `json:"stack_id,omitempty"`
	Namespace      string            `json:"namespace"`
	AppType        AppType           `json:"app_type"`
	GitRepoURL     string            `json:"git_repo_url"`
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	DockerContext  string            `json:"docker_context,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	Status         PipelineStatus    `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
}

// 실행 모드. 어느 쪽이 이미지를 만들고 클러스터에 반영하는지를 가른다.
//
// 자세한 설계는 docs/20_아키텍처/Stack_CICD_통합모드_설계.md 와
// Stack_CICD_긴급모드_설계.md 에 있다.
const (
	// ExecutionModeStackIntegrated 는 스택의 컴포넌트가 실행을 맡는 일반 경로다.
	// CI 플랫폼이 빌드해 이미지 레지스트리에 올리고, CD 도구가 클러스터에 반영한다.
	ExecutionModeStackIntegrated = "stack_integrated"
	// ExecutionModeEmergencyDirect 는 스택 컴포넌트를 쓸 수 없을 때 플랫폼이
	// 직접 빌드·배포하는 장애 대응 경로다.
	ExecutionModeEmergencyDirect = "emergency_direct"
)

// DelegatesBuildToRunner 는 이미지 빌드를 스택의 CI 러너가 맡는지다.
//
// 플랫폼이 직접 빌드하는 경로는 API 서버가 host 에서 돌던 시절에 만들어졌다.
// 지금 API 는 파드 안에서 돌고 그 안에는 도커 데몬이 없다 — 직접 빌드는
// 성공할 수 없는 경로다. 스택에 묶인 파이프라인의 빌드는 스택의 CI 플랫폼이
// 맡는다(통합모드 설계 3.2: "Nullus API 서버가 정상 실행 과정에서 직접
// git clone, docker build, kind load, kubectl apply 를 수행하지 않는다").
//
// 빌드가 없는 파이프라인은 위임할 것이 없으므로 기존 경로를 그대로 둔다.
func (p *Pipeline) DelegatesBuildToRunner() bool {
	if p == nil {
		return false
	}
	if strings.TrimSpace(p.DockerfilePath) == "" || strings.TrimSpace(p.StackID) == "" {
		return false
	}
	return strings.TrimSpace(p.ExecutionMode) != ExecutionModeEmergencyDirect
}

// PipelineTemplate represents a reusable CI/CD pipeline template.
type PipelineTemplate struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	AppType        AppType           `json:"app_type"`
	Stages         []string          `json:"stages"`
	GitRepoURL     string            `json:"git_repo_url,omitempty"`
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	DockerContext  string            `json:"docker_context,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	CreatedBy      string            `json:"created_by,omitempty"`
}

// DeployStep tracks progress of a single resource application.
type DeployStep struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Kind      string   `json:"kind"`
	Message   string   `json:"message,omitempty"`
	AppliedAt string   `json:"applied_at,omitempty"`
	Logs      []string `json:"logs,omitempty"`
}

// Deployment represents a single deployment run of a pipeline.
type Deployment struct {
	ID          string           `json:"id"`
	PipelineID  string           `json:"pipeline_id"`
	Version     string           `json:"version"`
	Status      DeploymentStatus `json:"status"`
	Steps       []DeployStep     `json:"steps,omitempty"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	DeployedBy  string           `json:"deployed_by"`
}
