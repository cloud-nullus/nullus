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
	// Platform 은 파이프라인 파일 형식과 토큰 확보 경로를 정한다.
	// 비면 GitLab 으로 본다.
	Platform port.SCMPlatform
	// SharedAccessToken 은 리포 범위 토큰을 발급할 수 없는 플랫폼에서
	// Argo CD 인증에 재사용할 토큰이다 (GitHub 의 조직 PAT).
	SharedAccessToken string
	// AccessDomain / GatewayName / GatewayNamespace 가 있으면
	// 외부 접근용 HTTPRoute 도 스캐폴딩에 포함한다.
	AccessDomain     string
	GatewayName      string
	GatewayNamespace string
}

// argoReadTokenName 은 Argo CD 가 저장소를 읽을 때 쓰는 토큰 이름이다.
// CI 의 쓰기 토큰과 분리한다 — Argo CD 에는 읽기 권한만 있으면 된다.
const argoReadTokenName = "nullus-argocd-read"

// maskableMinLength 는 GitLab 이 masked 변수에 요구하는 최소 길이다.
const maskableMinLength = 8

// canMaskVariableValue 는 GitLab 의 마스킹 요건을 값이 만족하는지 본다.
//
// 요건을 못 채우는 값을 masked 로 등록하면 GitLab 이 400
// (`{"message":{"value":["is invalid"]}}`) 으로 거부한다. 거부되면 변수 자체가
// 등록되지 않아 CI 가 `docker login` 에서 죽는데, 오류 메시지가 마스킹과
// 무관해 보여 원인을 찾기 어렵다.
//
// 요건은 GitLab 의 Ci::Maskable::REGEX 와 같다 — 8자 이상이고
// `a-zA-Z0-9_+=/@:.~-` 만 쓸 수 있다(공백·줄바꿈·`$` 불가).
// 문서는 "Base64 알파벳 + @:.~" 라고만 적고 있으나 실제 API 는 `-` 와 `_` 도
// 받는다 — GitLab 17.7 에 직접 질의해 확인한 집합이다.
func canMaskVariableValue(value string) bool {
	if len(value) < maskableMinLength {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("_+=/@:.~-", r):
		default:
			return false
		}
	}
	return true
}

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
		Platform:         input.Platform,
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
	if input.Platform == port.SCMPlatformGitHub {
		uc.configureGitHubPipeline(ctx, project, target, input, out)
		return
	}
	uc.configureGitLabPipeline(ctx, project, target, input, out)
}

// configureGitHubPipeline 은 GitHub 리포의 파이프라인 설정을 맞춘다.
//
// 발급할 토큰이 없다. 워크플로는 contents:write 권한의 내장 GITHUB_TOKEN 으로
// 매니페스트를 되쓰므로 되쓰기 토큰 변수가 필요 없고, GitHub 에는 리포 범위
// 토큰 API 도 없어 Argo CD 인증에는 조직 PAT 를 그대로 쓴다.
func (uc *ProvisionAppProject) configureGitHubPipeline(
	ctx context.Context,
	project *port.SCMProject,
	target *port.ImageTarget,
	input ProvisionAppProjectInput,
	out *ProvisionAppProjectOutput,
) {
	out.ArgoReadToken = strings.TrimSpace(input.SharedAccessToken)
	if out.ArgoReadToken == "" {
		out.Warnings = append(out.Warnings,
			"GitHub PAT 가 전달되지 않아 Argo CD 저장소 자격증명을 만들지 못했습니다 — "+
				"private 저장소 동기화가 실패할 수 있습니다")
	}
	uc.setRegistryVariables(ctx, project, target, input, out)
}

// configureGitLabPipeline 은 프로젝트 범위 토큰을 발급해 변수로 건다.
func (uc *ProvisionAppProject) configureGitLabPipeline(
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

	uc.setRegistryVariables(ctx, project, target, input, out)
}

// setRegistryVariables 는 레지스트리 로그인 자격증명을 파이프라인에 건다.
//
// 사용자가 준 것만 등록할 수 있다. 내장 값만 쓰는 구성(GHCR)에서는
// target.RequiredVariables 가 비어 있어 아무 일도 하지 않는다.
func (uc *ProvisionAppProject) setRegistryVariables(
	ctx context.Context,
	project *port.SCMProject,
	target *port.ImageTarget,
	input ProvisionAppProjectInput,
	out *ProvisionAppProjectOutput,
) {
	if len(target.RequiredVariables) == 0 {
		return
	}
	if uc.pipeline == nil {
		out.Warnings = append(out.Warnings, "파이프라인 설정 어댑터가 없어 레지스트리 변수를 등록하지 못했습니다")
		out.MissingVariables = append(out.MissingVariables, target.RequiredVariables...)
		return
	}

	for _, key := range target.RequiredVariables {
		value, ok := input.RegistryCredentials[key]
		if !ok || strings.TrimSpace(value) == "" {
			out.MissingVariables = append(out.MissingVariables, key)
			continue
		}

		// 마스킹 요건은 GitLab 에만 있다. GitHub Actions 는 등록된 시크릿을
		// 항상 로그에서 가리므로 값 형태를 따지지 않는다.
		masked := true
		if input.Platform != port.SCMPlatformGitHub {
			// 요건을 못 채우는 값(예: Harbor 기본 계정명 "admin")을 masked 로
			// 밀면 GitLab 이 등록을 통째로 거부해 변수가 아예 없는 상태가 된다
			// — 가려지지 않더라도 등록되는 편이 낫다.
			masked = canMaskVariableValue(value)
			if !masked {
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"변수 %s 는 GitLab 마스킹 요건(한 줄, %d자 이상, Base64 문자+@:.~)을 "+
						"만족하지 않아 마스킹 없이 등록합니다 — job 로그에 노출될 수 있습니다",
					key, maskableMinLength))
			}
		}

		if err := uc.pipeline.SetProjectVariable(ctx, project.ID, port.ProjectVariable{
			Key: key, Value: value, Masked: masked,
		}); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("변수 %s 등록 실패: %v", key, err))
			out.MissingVariables = append(out.MissingVariables, key)
		}
	}
}
