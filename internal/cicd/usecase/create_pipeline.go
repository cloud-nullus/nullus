package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// Sentinel errors for cross-context validation.
var (
	ErrStackNotFound    = errors.New("referenced stack does not exist")
	ErrStackOrgMismatch = errors.New("stack belongs to a different organization")
)

// CreatePipelineInput holds the parameters for creating a new pipeline.
type CreatePipelineInput struct {
	Name           string
	ExecutionMode  string
	TemplateID     string
	OrgID          string
	ClusterID      string
	StackID        string // optional — links pipeline to a stack
	Namespace      string
	AppType        domain.AppType
	GitRepoURL     string
	DockerfilePath string
	DockerContext  string
	EnvVars        map[string]string
	// ProvisionRepository 가 true 면 앱 저장소를 만들고 스캐폴딩을 커밋한 뒤
	// Argo CD Application 까지 연결한다. StackID 가 필요하다.
	ProvisionRepository bool
	// RequestedByEmail 은 요청한 사람의 이메일이다. 저장소가 든 조직의 멤버로
	// 넣는 데 쓴다 — 넣지 않으면 자기 저장소를 보지도 못한다.
	RequestedByEmail string
	// Port / Replicas 는 스캐폴딩 매니페스트에 반영된다.
	Port     int32
	Replicas int32
	// RegistryCredentials 는 외부 레지스트리 자격증명이다.
	RegistryCredentials map[string]string
}

// RepositoryProvisioner 는 파이프라인용 저장소 일체를 준비한다.
//
// 유스케이스끼리 직접 묶지 않고 인터페이스로 받는다 — 배선되지 않은 환경에서도
// CreatePipeline 이 그대로 동작해야 한다.
type RepositoryProvisioner interface {
	Execute(ctx context.Context, input ProvisionPipelineRepositoryInput) (*ProvisionPipelineRepositoryOutput, error)
}

// CreatePipelineOutput holds the result of creating a pipeline.
type CreatePipelineOutput struct {
	Pipeline     *domain.Pipeline
	StackWarning string `json:"stack_warning,omitempty"` // non-empty when stack exists but is not completed

	// 아래는 저장소 프로비저닝을 수행했을 때만 채워진다.
	RepositoryPath         string   `json:"repository_path,omitempty"`
	ArgoApplicationCreated bool     `json:"argo_application_created,omitempty"`
	MissingVariables       []string `json:"missing_variables,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
	// ScaffoldSkipped 는 이미 있던 저장소라 스캐폴딩을 쓰지 않았음을 알린다.
	// 사용자가 파일이 갱신됐다고 오해하면 배포되지 않는 원인을 엉뚱한 데서 찾는다.
	ScaffoldSkipped bool `json:"scaffold_skipped,omitempty"`
}

// CreatePipeline creates a new pipeline configuration.
type CreatePipeline struct {
	pipelineRepo port.PipelineRepository
	templateRepo port.PipelineTemplateRepository
	stackReader  port.StackReader // optional — nil disables stack validation
	provisioner  RepositoryProvisioner
}

// WithRepositoryProvisioner 는 저장소 프로비저닝 기능을 켠다.
//
// 주입하지 않으면 ProvisionRepository 요청이 오류가 된다 — 조용히 무시하면
// 사용자는 저장소가 만들어진 줄 알고 기다리게 된다.
func (uc *CreatePipeline) WithRepositoryProvisioner(p RepositoryProvisioner) *CreatePipeline {
	uc.provisioner = p
	return uc
}

// NewCreatePipeline constructs a CreatePipeline use case.
func NewCreatePipeline(
	pipelineRepo port.PipelineRepository,
	templateRepo port.PipelineTemplateRepository,
	stackReader ...port.StackReader,
) *CreatePipeline {
	uc := &CreatePipeline{
		pipelineRepo: pipelineRepo,
		templateRepo: templateRepo,
	}
	if len(stackReader) > 0 {
		uc.stackReader = stackReader[0]
	}
	return uc
}

// Execute creates a new pipeline.
func (uc *CreatePipeline) Execute(ctx context.Context, input CreatePipelineInput) (*CreatePipelineOutput, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("pipeline name is required")
	}
	if input.OrgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}
	if input.ClusterID == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	var tmpl *domain.PipelineTemplate
	if input.TemplateID != "" {
		var err error
		tmpl, err = uc.templateRepo.GetByID(ctx, input.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("template not found: %w", err)
		}
	}

	envVars := make(map[string]string)
	if tmpl != nil {
		maps.Copy(envVars, tmpl.EnvVars)
	}
	maps.Copy(envVars, input.EnvVars)

	// --- Cross-context validation: Stack reference ---
	var stackWarning string
	if input.StackID != "" && uc.stackReader != nil {
		summary, err := uc.stackReader.GetStackSummary(ctx, input.StackID)
		if err != nil {
			return nil, fmt.Errorf("validate stack: %w", err)
		}
		if summary == nil {
			return nil, ErrStackNotFound
		}
		if summary.OrgID != input.OrgID {
			return nil, ErrStackOrgMismatch
		}
		// Warn (but allow) when stack is not yet deployed.
		if summary.State != "completed" {
			stackWarning = fmt.Sprintf(
				"stack %q is in state %q — CI/CD tools may not be available yet",
				input.StackID, summary.State,
			)
		}
	}

	// --- 저장소 프로비저닝 ---
	// 파이프라인 레코드를 만들기 전에 수행한다. 저장소가 없으면 배포할 것이
	// 없으므로, 실패한 채로 레코드만 남기지 않는다.
	gitRepoURL := input.GitRepoURL
	provisionOut, err := uc.provisionRepository(ctx, input)
	if err != nil {
		return nil, err
	}
	if provisionOut != nil {
		gitRepoURL = provisionOut.RepoURL
		if repo := strings.TrimSpace(provisionOut.ImageRepository); repo != "" {
			envVars[envRegistryURL] = repo
		}
	}

	pipeline := &domain.Pipeline{
		ID:             generateID("pip"),
		Name:           input.Name,
		ExecutionMode:  input.ExecutionMode,
		TemplateID:     input.TemplateID,
		OrgID:          input.OrgID,
		ClusterID:      input.ClusterID,
		StackID:        input.StackID,
		Namespace:      input.Namespace,
		AppType:        input.AppType,
		GitRepoURL:     gitRepoURL,
		DockerfilePath: input.DockerfilePath,
		DockerContext:  input.DockerContext,
		EnvVars:        envVars,
		Status:         domain.PipelineStatusActive,
		CreatedAt:      time.Now(),
	}
	if pipeline.ExecutionMode == "" {
		if pipeline.StackID != "" {
			pipeline.ExecutionMode = "stack_integrated"
		} else {
			pipeline.ExecutionMode = "emergency_direct"
		}
	}

	if err := uc.pipelineRepo.Create(ctx, pipeline); err != nil {
		return nil, fmt.Errorf("create pipeline: %w", err)
	}

	out := &CreatePipelineOutput{
		Pipeline:     pipeline,
		StackWarning: stackWarning,
	}
	if provisionOut != nil {
		out.RepositoryPath = provisionOut.Project.FullPath
		out.ArgoApplicationCreated = provisionOut.ArgoApplicationCreated
		out.MissingVariables = provisionOut.MissingVariables
		out.Warnings = provisionOut.Warnings
		out.ScaffoldSkipped = provisionOut.ScaffoldSkipped
	}
	return out, nil
}

// shouldProvisionRepository 는 이 파이프라인의 저장소·CI job 을 플랫폼이
// 준비할지다.
//
// 스택에 묶인 파이프라인은 명시적 요청이 없어도 준비한다. 통합모드가 그것들이
// 있다는 전제 위에 서 있기 때문이다 — 러너가 실행할 Jenkinsfile 도, Argo CD 가
// 동기화할 매니페스트도, 배포 실행을 넘길 job 도 전부 이 단계에서 만들어진다.
// 준비 없이 만든 파이프라인은 배포를 눌러도 넘길 job 이 없다.
//
// 실제로 그랬다. 프론트는 provision_repository 를 한 번도 보내지 않아, UI 로
// 만든 파이프라인에는 저장소조차 없었다.
//
// EnsureProject 는 멱등하다 — 이미 있는 저장소는 그대로 쓰므로 기존 저장소를
// 지정한 경우에도 안전하다.
func shouldProvisionRepository(input CreatePipelineInput) bool {
	if input.ProvisionRepository {
		return true
	}
	// 긴급 직접 배포는 스택 컴포넌트를 쓸 수 없을 때의 경로다. 그 상황에서
	// 저장소 생성을 시도하면 쓰지 못하는 도구에 붙으려다 파이프라인 생성 자체가
	// 막힌다.
	if strings.TrimSpace(input.ExecutionMode) == domain.ExecutionModeEmergencyDirect {
		return false
	}
	return strings.TrimSpace(input.StackID) != ""
}

// provisionRepository 는 앱 저장소와 CI job 을 준비한다.
//
// 준비 대상이 아니면 nil, nil 을 돌려준다. 대상인데 배선이 없거나 스택이
// 없으면 오류다 — 조용히 넘어가면 사용자는 저장소가 생긴 줄 알고 기다린다.
func (uc *CreatePipeline) provisionRepository(
	ctx context.Context,
	input CreatePipelineInput,
) (*ProvisionPipelineRepositoryOutput, error) {
	if !shouldProvisionRepository(input) {
		return nil, nil
	}
	if uc.provisioner == nil {
		// 사용자가 명시적으로 요청했으면 오류다 — 요청을 받고 아무것도 하지
		// 않으면 저장소가 생긴 줄 알고 기다린다.
		if input.ProvisionRepository {
			return nil, fmt.Errorf("저장소 프로비저닝이 배선되지 않았습니다")
		}
		// 자동 준비는 건너뛴다. 배선이 없는 구성에서 파이프라인 생성 자체를
		// 막지는 않는다 — 요청하지 않은 부가 작업이 본래 작업을 무너뜨리면 안 된다.
		slog.Warn("저장소 프로비저닝 배선이 없어 자동 준비를 건너뜁니다",
			"pipeline", input.Name, "stack_id", input.StackID)
		return nil, nil
	}
	if strings.TrimSpace(input.StackID) == "" {
		return nil, fmt.Errorf("저장소를 만들려면 stack_id 가 필요합니다")
	}

	out, err := uc.provisioner.Execute(ctx, ProvisionPipelineRepositoryInput{
		AppName:             input.Name,
		StackID:             input.StackID,
		TemplateID:          input.TemplateID,
		Namespace:           input.Namespace,
		Port:                input.Port,
		Replicas:            input.Replicas,
		AppType:             input.AppType,
		RegistryCredentials: input.RegistryCredentials,
		RequestedByEmail:    input.RequestedByEmail,
	})
	if err != nil {
		return nil, fmt.Errorf("provision repository for %q: %w", input.Name, err)
	}
	return out, nil
}
