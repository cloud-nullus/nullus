package usecase

import (
	"context"
	"errors"
	"fmt"
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

// provisionRepository 는 요청 시 앱 저장소를 준비한다.
//
// 요청하지 않았으면 nil, nil 을 돌려준다. 요청했는데 배선이 없거나 스택이
// 없으면 오류다 — 조용히 넘어가면 사용자는 저장소가 생긴 줄 알고 기다린다.
func (uc *CreatePipeline) provisionRepository(
	ctx context.Context,
	input CreatePipelineInput,
) (*ProvisionPipelineRepositoryOutput, error) {
	if !input.ProvisionRepository {
		return nil, nil
	}
	if uc.provisioner == nil {
		return nil, fmt.Errorf("저장소 프로비저닝이 배선되지 않았습니다")
	}
	if strings.TrimSpace(input.StackID) == "" {
		return nil, fmt.Errorf("저장소를 만들려면 stack_id 가 필요합니다")
	}

	out, err := uc.provisioner.Execute(ctx, ProvisionPipelineRepositoryInput{
		AppName:             input.Name,
		StackID:             input.StackID,
		Namespace:           input.Namespace,
		Port:                input.Port,
		Replicas:            input.Replicas,
		RegistryCredentials: input.RegistryCredentials,
	})
	if err != nil {
		return nil, fmt.Errorf("provision repository for %q: %w", input.Name, err)
	}
	return out, nil
}
