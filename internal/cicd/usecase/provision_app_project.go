package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/scaffold"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// DeployTokenVariable 은 CI 의 deploy 단계가 매니페스트를 되쓸 때 쓰는 변수다.
// 렌더러가 만드는 스크립트와 이름이 같아야 한다.
const DeployTokenVariable = scaffold.DeployTokenVar

// deployTokenName 은 발급하는 프로젝트 토큰의 이름이다.
const deployTokenName = "nullus-deploy"

// ProvisionAppProjectInput 은 애플리케이션 프로젝트 프로비저닝 요청이다.
type ProvisionAppProjectInput struct {
	AppName   string
	GroupPath string
	GroupID   string
	// Namespace 는 배포 대상 쿠버네티스 네임스페이스다.
	Namespace string
	Port      int32
	Replicas  int32
	// RegistryCredentials 는 외부 레지스트리 자격증명이다.
	// SCM 프로젝트 레지스트리를 쓰는 구성에서는 필요 없다.
	RegistryCredentials map[string]string
	// AccessDomain / GatewayName / GatewayNamespace 가 있으면
	// 외부 접근용 HTTPRoute 도 스캐폴딩에 포함한다.
	AccessDomain     string
	GatewayName      string
	GatewayNamespace string
}

// argoReadTokenName 은 Argo CD 가 저장소를 읽을 때 쓰는 토큰 이름이다.
// CI 의 쓰기 토큰과 분리한다 — Argo CD 에는 읽기 권한만 있으면 된다.
const argoReadTokenName = "nullus-argocd-read"

// ProvisionAppProjectOutput 은 프로비저닝 결과다.
type ProvisionAppProjectOutput struct {
	Project     *port.SCMProject
	ImageTarget *port.ImageTarget
	// ArgoReadToken 은 Argo CD 가 저장소를 읽는 데 쓸 토큰이다.
	// 발급에 실패하면 비어 있고 Warnings 에 사유가 담긴다.
	ArgoReadToken string
	// MissingVariables 는 파이프라인이 돌기 전에 사람이 채워야 할 변수다.
	MissingVariables []string
	// Warnings 는 치명적이지 않지만 알려야 하는 문제다.
	Warnings []string
}

// ProvisionAppProject 는 앱 저장소를 만들고 CI/CD 가 돌 수 있는 상태로 만든다.
//
// 소스와 배포 매니페스트가 한 저장소에 함께 산다. CI 가 이미지를 올린 뒤
// deploy/ 의 태그를 갱신해 커밋하면 Argo CD 가 그 커밋을 보고 배포한다.
type ProvisionAppProject struct {
	scm      port.SCMProvisioner
	pipeline port.PipelineConfigurator
	registry port.ImageRegistryResolver
}

// NewProvisionAppProject 는 유스케이스를 만든다.
func NewProvisionAppProject(
	scm port.SCMProvisioner,
	pipeline port.PipelineConfigurator,
	registry port.ImageRegistryResolver,
) *ProvisionAppProject {
	return &ProvisionAppProject{scm: scm, pipeline: pipeline, registry: registry}
}

// Execute 는 프로젝트 생성 → 이미지 대상 결정 → 스캐폴딩 커밋 → CI 변수 등록을 수행한다.
func (uc *ProvisionAppProject) Execute(
	ctx context.Context,
	input ProvisionAppProjectInput,
) (*ProvisionAppProjectOutput, error) {
	app := strings.TrimSpace(input.AppName)
	if app == "" {
		return nil, fmt.Errorf("app_name is required")
	}
	groupPath := strings.TrimSpace(input.GroupPath)
	if groupPath == "" {
		return nil, fmt.Errorf("group_path is required")
	}

	project, err := uc.scm.EnsureProject(ctx, port.ProjectSpec{
		Name:        app,
		Path:        app,
		GroupID:     input.GroupID,
		GroupPath:   groupPath,
		Description: "Nullus 가 생성한 애플리케이션 프로젝트",
		Visibility:  "private",
		// 기본 브랜치가 없으면 스캐폴딩 커밋이 실패한다.
		InitReadme: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure app project %q: %w", app, err)
	}

	// 이미지 저장 위치는 스택 구성이 정한다 — 여기서 GitLab 을 전제하지 않는다.
	target, err := uc.registry.Resolve(ctx, port.ImageTargetSpec{
		AppName:        app,
		SCMProjectPath: project.FullPath,
		SCMRegistryURL: project.RegistryURL,
		OrgPath:        groupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve image target for %q: %w", app, err)
	}

	files, err := scaffold.Render(scaffold.Input{
		AppName:          app,
		Namespace:        input.Namespace,
		Port:             input.Port,
		Replicas:         input.Replicas,
		ImageTarget:      target,
		AccessDomain:     input.AccessDomain,
		GatewayName:      input.GatewayName,
		GatewayNamespace: input.GatewayNamespace,
	})
	if err != nil {
		return nil, fmt.Errorf("render scaffold for %q: %w", app, err)
	}

	if err := uc.scm.CommitFiles(ctx, project.ID, port.CommitSpec{
		Branch:  project.DefaultBranch,
		Message: "chore(nullus): scaffold pipeline and deployment manifests",
		Files:   files,
	}); err != nil {
		return nil, fmt.Errorf("commit scaffold to %q: %w", project.FullPath, err)
	}

	out := &ProvisionAppProjectOutput{Project: project, ImageTarget: target}
	uc.configurePipeline(ctx, project, target, input, out)

	sort.Strings(out.MissingVariables)
	return out, nil
}

// configurePipeline 은 CI 변수를 등록하고 무엇이 빠졌는지 모은다.
//
// 변수 등록 실패로 전체를 되돌리지 않는다 — 프로젝트와 스캐폴딩은 이미
// 만들어졌고, 되돌리면 재실행 시 무엇이 남았는지 알 수 없다.
// 대신 사람이 채워야 할 목록으로 알린다.
func (uc *ProvisionAppProject) configurePipeline(
	ctx context.Context,
	project *port.SCMProject,
	target *port.ImageTarget,
	input ProvisionAppProjectInput,
	out *ProvisionAppProjectOutput,
) {
	if uc.pipeline == nil {
		out.Warnings = append(out.Warnings, "파이프라인 설정 어댑터가 없어 CI 변수를 등록하지 못했습니다")
		out.MissingVariables = append(out.MissingVariables, DeployTokenVariable)
		out.MissingVariables = append(out.MissingVariables, target.RequiredVariables...)
		return
	}

	// deploy 단계가 매니페스트를 되쓰려면 저장소 쓰기 권한이 필요하다.
	token, err := uc.pipeline.CreateProjectAccessToken(ctx, project.ID, port.AccessTokenSpec{
		Name:   deployTokenName,
		Scopes: []string{"write_repository"},
	})
	switch {
	case err != nil:
		out.Warnings = append(out.Warnings,
			fmt.Sprintf("배포 토큰 발급 실패 — %s 를 직접 등록해야 합니다: %v", DeployTokenVariable, err))
		out.MissingVariables = append(out.MissingVariables, DeployTokenVariable)
	default:
		if setErr := uc.pipeline.SetProjectVariable(ctx, project.ID, port.ProjectVariable{
			Key: DeployTokenVariable, Value: token, Masked: true,
		}); setErr != nil {
			out.Warnings = append(out.Warnings,
				fmt.Sprintf("배포 토큰 변수 등록 실패: %v", setErr))
			out.MissingVariables = append(out.MissingVariables, DeployTokenVariable)
		}
	}

	// Argo CD 가 private 저장소를 읽을 토큰. 없으면 동기화가
	// "authentication required" 로 실패한다.
	readToken, readErr := uc.pipeline.CreateProjectAccessToken(ctx, project.ID, port.AccessTokenSpec{
		Name:   argoReadTokenName,
		Scopes: []string{"read_repository", "read_registry"},
	})
	if readErr != nil {
		out.Warnings = append(out.Warnings,
			fmt.Sprintf("Argo CD 읽기 토큰 발급 실패 — private 저장소 동기화가 실패할 수 있습니다: %v", readErr))
	} else {
		out.ArgoReadToken = readToken
	}

	// 레지스트리 자격증명은 사용자가 준 것만 등록할 수 있다.
	for _, key := range target.RequiredVariables {
		value, ok := input.RegistryCredentials[key]
		if !ok || strings.TrimSpace(value) == "" {
			out.MissingVariables = append(out.MissingVariables, key)
			continue
		}
		if err := uc.pipeline.SetProjectVariable(ctx, project.ID, port.ProjectVariable{
			Key: key, Value: value, Masked: true,
		}); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("변수 %s 등록 실패: %v", key, err))
			out.MissingVariables = append(out.MissingVariables, key)
		}
	}
}
