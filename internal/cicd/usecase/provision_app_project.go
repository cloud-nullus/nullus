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
	// RepoAccessToken 은 저장소 쓰기에 쓸 토큰이다.
	//
	// 플랫폼이 프로젝트 범위 토큰을 지원하지 않을 때 채워진다(Gitea·GitHub).
	// deploy 단계가 매니페스트 태그를 되쓰려면 필요하다.
	RepoAccessToken string
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
	// StackID 는 배포 매니페스트에 스택 라벨로 실린다. 클러스터에서 워크로드가
	// 어느 스택 소속인지 판별하는 유일한 키다.
	StackID string
	// TemplateID 는 배포 매니페스트에 템플릿 라벨로 실린다.
	// 템플릿별 자원 사용 비교나 템플릿 단위 조회에 쓴다.
	TemplateID string
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
	// ScaffoldSkipped 는 이미 있던 저장소라 스캐폴딩을 쓰지 않았음을 알린다.
	ScaffoldSkipped bool
	// CIJobURL 은 만들어진 CI job 의 주소다. CI 가 SCM 과 분리된 플랫폼
	// (Jenkins)에서만 채워진다.
	CIJobURL string
	// CredentialManifests 는 파이프라인 자격증명 ExternalSecret 이다.
	//
	// 여기서 적용하지 않는다 — 클러스터 접근은 상위 유스케이스가 한곳에서
	// 맡는다(Argo CD 리소스와 같은 경로로 적용된다).
	CredentialManifests []string
}

// ProvisionAppProject 는 앱 저장소를 만들고 CI/CD 가 돌 수 있는 상태로 만든다.
//
// 소스와 배포 매니페스트가 한 저장소에 함께 산다. CI 가 이미지를 올린 뒤
// deploy/ 의 태그를 갱신해 커밋하면 Argo CD 가 그 커밋을 보고 배포한다.
type ProvisionAppProject struct {
	scm      port.SCMProvisioner
	pipeline port.PipelineConfigurator
	registry port.ImageRegistryResolver
	// ciJobs / webhooks 는 CI 가 SCM 과 분리된 플랫폼에서만 채워진다.
	// GitLab CI·GitHub Actions 는 파이프라인 정의를 푸시하면 자동 감지하므로 nil.
	ciJobs    port.CIJobProvisioner
	webhooks  port.SCMWebhookProvisioner
	ciBaseURL string
	scmURL    string
	// creds 는 CI 변수 저장소가 없는 SCM(Gitea)에서 자격증명을 OpenBao → ESO
	// 평면으로 나르는 경로다.
	creds giteaCredentialPlane
}

// giteaCredentialPlane 은 포트의 별칭이다 — 어댑터 타입을 직접 받으면 레이어
// 방향이 뒤집힌다.
type giteaCredentialPlane = port.PipelineCredentialPlane

// WithCredentialPlane 은 CI 변수 저장소가 없는 SCM 의 자격증명 경로를 배선한다.
func (uc *ProvisionAppProject) WithCredentialPlane(plane port.PipelineCredentialPlane) *ProvisionAppProject {
	uc.creds = plane
	return uc
}

// WithCIJobs 는 CI 서버 job 생성 경로를 배선한다.
//
// Jenkins 처럼 job 이 먼저 존재해야 하는 CI 를 위한 것이다. 배선하지 않으면
// job 생성을 건너뛴다 — 리포와 스캐폴딩은 만들어지되 빌드는 돌지 않는다.
func (uc *ProvisionAppProject) WithCIJobs(
	jobs port.CIJobProvisioner,
	webhooks port.SCMWebhookProvisioner,
	ciBaseURL, scmURL string,
) *ProvisionAppProject {
	uc.ciJobs = jobs
	uc.webhooks = webhooks
	uc.ciBaseURL = strings.TrimSpace(ciBaseURL)
	uc.scmURL = strings.TrimSpace(scmURL)
	return uc
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
		StackID:          input.StackID,
		TemplateID:       input.TemplateID,
	})
	if err != nil {
		return nil, fmt.Errorf("render scaffold for %q: %w", app, err)
	}

	// 이미 있던 저장소에는 스캐폴딩을 쓰지 않는다.
	//
	// CommitFiles 는 upsert 라 그대로 두면 CI 가 갱신해 둔 이미지 태그가 초기값
	// (:bootstrap)으로 되돌아간다 — 그 태그는 레지스트리에 없으므로 돌던 배포가
	// ImagePullBackOff 로 떨어졌다가 CI 가 다시 돌아야 복구된다. 사용자가 고친
	// Dockerfile·워크플로·매니페스트도 같이 사라진다.
	scaffoldSkipped := !project.Created
	if !scaffoldSkipped {
		if err := uc.scm.CommitFiles(ctx, project.ID, port.CommitSpec{
			Branch:  project.DefaultBranch,
			Message: "chore(nullus): scaffold pipeline and deployment manifests",
			Files:   files,
		}); err != nil {
			return nil, fmt.Errorf("commit scaffold to %q: %w", project.FullPath, err)
		}
	}

	out := &ProvisionAppProjectOutput{
		Project:         project,
		ImageTarget:     target,
		ScaffoldSkipped: scaffoldSkipped,
	}
	if scaffoldSkipped {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"%s 저장소가 이미 있어 스캐폴딩을 건너뛰었습니다 — 기존 파일을 덮어쓰지 않습니다",
			project.FullPath))
	}
	uc.configurePipeline(ctx, project, target, input, out)
	uc.ensureCIJob(ctx, project, out)

	sort.Strings(out.MissingVariables)
	return out, nil
}

// configureGiteaPipeline 은 파이프라인 자격증명을 OpenBao → ESO 평면에 얹는다.
//
// Gitea 에는 GitLab 같은 프로젝트 CI 변수 저장소가 없다. Jenkinsfile 의
// envFrom 이 참조하는 nullus-ci-<app> Secret 을 여기서 만든다 — 없으면 agent
// 파드가 없는 Secret 을 참조해 기동하지 못한다.
//
// 레지스트리 자격증명은 사용자가 준 것이 없으면 채울 수 없다. 그때는 조용히
// 넘기지 않고 "사람이 채워야 할 목록" 으로 알린다.
func (uc *ProvisionAppProject) configureGiteaPipeline(
	ctx context.Context,
	project *port.SCMProject,
	target *port.ImageTarget,
	input ProvisionAppProjectInput,
	out *ProvisionAppProjectOutput,
) {
	if uc.creds == nil {
		out.Warnings = append(out.Warnings,
			"자격증명 평면이 배선되지 않아 파이프라인 Secret 을 만들지 못했습니다")
		out.MissingVariables = append(out.MissingVariables, target.RequiredVariables...)
		return
	}

	vars := make([]port.PipelineVariable, 0, len(target.RequiredVariables)+2)

	// deploy 단계가 매니페스트 태그를 되쓰려면 저장소 쓰기 권한이 필요하다.
	// Gitea 에는 프로젝트 범위 토큰이 없으므로 스택의 자동화 토큰을 쓴다.
	if token := strings.TrimSpace(input.RepoAccessToken); token != "" {
		vars = append(vars,
			port.PipelineVariable{Key: scaffold.GitUsernameVar, Value: giteaAutomationUser},
			port.PipelineVariable{Key: scaffold.GitPasswordVar, Value: token},
		)
	} else {
		out.Warnings = append(out.Warnings,
			"Gitea 액세스 토큰이 없어 매니페스트 되커밋 자격증명을 채우지 못했습니다")
		out.MissingVariables = append(out.MissingVariables, scaffold.GitPasswordVar)
	}

	for _, key := range target.RequiredVariables {
		value := strings.TrimSpace(input.RegistryCredentials[key])
		if value == "" {
			out.MissingVariables = append(out.MissingVariables, key)
			continue
		}
		vars = append(vars, port.PipelineVariable{Key: key, Value: value})
	}

	if len(vars) == 0 {
		return
	}

	manifest, err := uc.creds.Provision(ctx, project.Name, vars)
	if err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("파이프라인 자격증명 준비 실패: %v", err))
		return
	}
	if strings.TrimSpace(manifest) != "" {
		out.CredentialManifests = append(out.CredentialManifests, manifest)
	}
}

// ensureCIJob 은 CI 서버에 job 을 만들고 저장소에 webhook 을 건다.
//
// Jenkins 는 GitLab CI·GitHub Actions 와 달리 Jenkinsfile 만 커밋해서는 아무
// 일도 일어나지 않는다 — job 이 먼저 존재해야 한다. 배선이 없으면(GitLab/
// GitHub 경로) 조용히 건너뛴다.
//
// 실패로 전체를 되돌리지 않는다. 리포와 스캐폴딩은 이미 만들어졌고, 되돌리면
// 재실행 시 무엇이 남았는지 알 수 없다 — Argo CD Application 생성과 같은 방침이다.
// 대신 무엇이 안 됐는지 경고로 남긴다. 조용히 넘기면 사용자는 파이프라인이
// 준비된 줄 알고 커밋했다가 아무 일도 일어나지 않는 것을 보게 된다.
func (uc *ProvisionAppProject) ensureCIJob(
	ctx context.Context,
	project *port.SCMProject,
	out *ProvisionAppProjectOutput,
) {
	if uc.ciJobs == nil {
		return
	}

	owner, name, ok := splitFullPath(project.FullPath)
	if !ok {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"저장소 경로 %q 를 소유자/이름으로 나눌 수 없어 CI job 을 만들지 못했습니다", project.FullPath))
		return
	}

	job, err := uc.ciJobs.EnsureJob(ctx, port.CIJobSpec{
		Name:         name,
		RepoCloneURL: project.HTTPCloneURL,
		RepoOwner:    owner,
		RepoName:     name,
		ServerURL:    uc.scmURL,
		CredentialID: giteaCredentialID,
		PipelinePath: scaffold.JenkinsfilePath,
	})
	if err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("CI job 생성 실패 (%s): %v", name, err))
		return
	}
	out.CIJobURL = job.URL

	// webhook 이 없으면 job 은 만들어졌지만 새 커밋을 모른다.
	if uc.webhooks == nil || uc.ciBaseURL == "" {
		return
	}
	hookURL := strings.TrimRight(uc.ciBaseURL, "/") + "/gitea-webhook/post"
	if err := uc.webhooks.EnsureWebhook(ctx, project.ID, hookURL, ""); err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"webhook 등록 실패 (%s) — 커밋해도 빌드가 자동으로 시작되지 않습니다: %v", project.FullPath, err))
	}
}

// splitFullPath 는 owner/name 을 나눈다.
func splitFullPath(fullPath string) (owner, name string, ok bool) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(fullPath), "/"), "/")
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[len(parts)-2], parts[len(parts)-1], true
}

// giteaCredentialID 는 Jenkins 에 등록된 Gitea 자격증명 식별자다.
// JCasC 가 ESO 로 동기화한 Secret 을 읽어 이 이름으로 만든다.
const giteaCredentialID = "nullus-gitea"

// giteaAutomationUser 는 Gitea 자동화 계정이다.
// 어댑터(gitea.AutomationUser)와 같아야 한다 — 레이어 방향 때문에 import 하지
// 않고 값을 둔다.
const giteaAutomationUser = "gitea_admin"

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
	if input.Platform == port.SCMPlatformGitea {
		uc.configureGiteaPipeline(ctx, project, target, input, out)
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
