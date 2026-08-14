package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/argocd"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// ProvisionPipelineRepositoryInput 은 파이프라인용 저장소 준비 요청이다.
type ProvisionPipelineRepositoryInput struct {
	AppName string
	StackID string
	// TemplateID 는 배포 매니페스트에 템플릿 라벨로 실린다. 비면 라벨을 붙이지 않는다.
	TemplateID string
	// Namespace 는 배포 대상 쿠버네티스 네임스페이스다.
	Namespace string
	Port      int32
	Replicas  int32
	// CommonProjectPath 는 공용 프로젝트 경로다. 비면 기본값.
	CommonProjectPath string
	// RegistryCredentials 는 외부 레지스트리 자격증명이다.
	RegistryCredentials map[string]string
}

// ProvisionPipelineRepositoryOutput 은 준비 결과다.
type ProvisionPipelineRepositoryOutput struct {
	CommonProject *port.SCMProject
	Project       *port.SCMProject
	// RepoURL / ImageRepository 는 파이프라인 레코드에 남길 값이다.
	RepoURL         string
	ImageRepository string

	ArgoApplicationCreated bool
	MissingVariables       []string
	Warnings               []string
	// ScaffoldSkipped 는 이미 있던 저장소라 스캐폴딩을 쓰지 않았음을 알린다.
	ScaffoldSkipped bool
}

// ProvisionPipelineRepository 는 파이프라인 하나가 돌기 위한 저장소 일체를 만든다.
//
// 공용 프로젝트 → 앱 프로젝트 + 스캐폴딩 → Argo CD Application 순이다.
// 앞 단계 결과가 뒷 단계 입력이므로 순서가 고정되어 있다.
type ProvisionPipelineRepository struct {
	factory    port.SCMBundleFactory
	applier    port.ManifestApplier
	kubeconfig port.KubeconfigProvider
}

// NewProvisionPipelineRepository 는 유스케이스를 만든다.
func NewProvisionPipelineRepository(
	factory port.SCMBundleFactory,
	applier port.ManifestApplier,
	kubeconfig port.KubeconfigProvider,
) *ProvisionPipelineRepository {
	return &ProvisionPipelineRepository{factory: factory, applier: applier, kubeconfig: kubeconfig}
}

// Execute 는 저장소를 만들고 Argo CD 가 바라보게 한다.
func (uc *ProvisionPipelineRepository) Execute(
	ctx context.Context,
	input ProvisionPipelineRepositoryInput,
) (*ProvisionPipelineRepositoryOutput, error) {
	app := strings.TrimSpace(input.AppName)
	if app == "" {
		return nil, fmt.Errorf("app_name is required")
	}
	stackID := strings.TrimSpace(input.StackID)
	if stackID == "" {
		return nil, fmt.Errorf("stack_id is required")
	}

	bundle, err := uc.factory.For(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("resolve scm bundle for stack %s: %w", stackID, err)
	}

	commonOut, err := NewProvisionCommonProject(bundle.Provisioner).Execute(ctx, ProvisionCommonProjectInput{
		GroupPath:   bundle.GroupPath,
		ProjectPath: input.CommonProjectPath,
		Platform:    bundle.Platform,
	})
	if err != nil {
		return nil, fmt.Errorf("provision common project: %w", err)
	}

	// CI 가 SCM 과 분리된 플랫폼(Gitea + Jenkins)에서는 job 을 따로 만들어야
	// 한다. GitLab/GitHub 번들은 CIJobs 가 nil 이라 배선해도 동작이 같다.
	appOut, err := NewProvisionAppProject(bundle.Provisioner, bundle.Pipeline, bundle.Registry).
		WithCIJobs(bundle.CIJobs, bundle.Webhooks, bundle.CIBaseURL, scmServerURLFor(bundle)).
		WithCredentialPlane(bundle.Credentials).
		Execute(ctx, ProvisionAppProjectInput{
			AppName:             app,
			GroupPath:           commonOut.Group.FullPath,
			GroupID:             commonOut.Group.ID,
			Namespace:           input.Namespace,
			Port:                input.Port,
			Replicas:            input.Replicas,
			RegistryCredentials: input.RegistryCredentials,
			RepoAccessToken:     bundle.RepoAccessToken,
			Platform:            bundle.Platform,
			SharedAccessToken:   bundle.RepoAccessToken,
			AccessDomain:        bundle.AccessDomain,
			GatewayName:         bundle.GatewayName,
			GatewayNamespace:    bundle.ArgoNamespace,
			StackID:             stackID,
			TemplateID:          input.TemplateID,
		})
	if err != nil {
		return nil, fmt.Errorf("provision app project: %w", err)
	}

	out := &ProvisionPipelineRepositoryOutput{
		CommonProject:    commonOut.Project,
		Project:          appOut.Project,
		RepoURL:          appOut.Project.HTTPCloneURL,
		ImageRepository:  appOut.ImageTarget.Repository,
		MissingVariables: appOut.MissingVariables,
		Warnings:         appOut.Warnings,
		ScaffoldSkipped:  appOut.ScaffoldSkipped,
	}

	uc.applyArgoApplication(ctx, bundle, app, input.Namespace, appOut, out)
	return out, nil
}

// pullSecretUsername 은 kubelet 이 레지스트리에 로그인할 사용자명을 고른다.
//
// GHCR 은 사용자명으로 GitHub 계정을 기대한다. GitLab 쪽 토큰 이름을 그대로
// 넣으면 pull 이 인증 오류로 실패할 수 있고, 오류가 이미지 부재처럼 보인다.
func pullSecretUsername(bundle *port.SCMBundle) string {
	if bundle.Platform == port.SCMPlatformGitHub {
		if owner := strings.TrimSpace(bundle.GroupPath); owner != "" {
			return owner
		}
	}
	return argoReadTokenName
}

// applyArgoApplication 은 Application 리소스를 클러스터에 적용한다.
//
// 실패해도 저장소 프로비저닝을 되돌리지 않는다 — 저장소는 이미 만들어졌고,
// 되돌리면 재실행 시 무엇이 남았는지 알 수 없다. 경고로 알리고 계속한다.
// scmServerURLFor 는 CI 가 저장소를 스캔할 때 쓸 SCM 서버 주소다.
//
// 리포 주소가 아니라 서버 루트여야 한다 — Gitea SCM 소스는 이 주소로 API 를
// 불러 브랜치를 훑는다.
func scmServerURLFor(bundle *port.SCMBundle) string {
	if bundle == nil {
		return ""
	}
	type baseURLProvider interface{ BaseURL() string }
	if p, ok := bundle.Provisioner.(baseURLProvider); ok {
		return p.BaseURL()
	}
	return ""
}

func (uc *ProvisionPipelineRepository) applyArgoApplication(
	ctx context.Context,
	bundle *port.SCMBundle,
	app, destNamespace string,
	appOut *ProvisionAppProjectOutput,
	out *ProvisionPipelineRepositoryOutput,
) {
	if uc.applier == nil {
		out.Warnings = append(out.Warnings, "매니페스트 적용 어댑터가 없어 Argo CD Application 을 만들지 못했습니다")
		return
	}
	project := appOut.Project

	manifests := make([]string, 0, 4)

	// 자격증명이 먼저 있어야 Application 이 첫 동기화에서 저장소를 읽을 수 있다.
	if token := strings.TrimSpace(appOut.ArgoReadToken); token != "" {
		// kubelet 이 private 레지스트리에서 이미지를 받아올 자격증명.
		// 없으면 pull 이 "anonymous token: 403" 으로 실패한다.
		// Argo CD 의 CreateNamespace 보다 먼저 필요하므로 네임스페이스도 함께 만든다.
		if nsManifest, nsErr := kube.RenderNamespace(destNamespace); nsErr == nil {
			manifests = append(manifests, nsManifest)
			if pullSecret, psErr := kube.RenderImagePullSecret(kube.ImagePullSecretSpec{
				Namespace:    destNamespace,
				RegistryHost: appOut.ImageTarget.Host,
				Username:     pullSecretUsername(bundle),
				Password:     token,
			}); psErr == nil {
				manifests = append(manifests, pullSecret)
			} else {
				out.Warnings = append(out.Warnings, fmt.Sprintf("이미지 pull 자격증명 생성 실패: %v", psErr))
			}
		}

		secret, err := argocd.RenderRepositorySecret(argocd.RepositorySecretSpec{
			AppName:       app,
			ArgoNamespace: bundle.ArgoNamespace,
			RepoURL:       project.HTTPCloneURL,
			Password:      token,
		})
		if err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("Argo CD 저장소 자격증명 생성 실패: %v", err))
		} else {
			manifests = append(manifests, secret)
		}
	} else {
		out.Warnings = append(out.Warnings,
			"Argo CD 저장소 자격증명이 없어 private 저장소 동기화가 실패할 수 있습니다")
	}

	// 파이프라인 자격증명 Secret 은 Argo 리소스와 같은 경로로 적용한다.
	// 이것이 없으면 Jenkins agent 파드가 없는 Secret 을 참조해 기동하지 못한다.
	manifests = append(manifests, appOut.CredentialManifests...)

	manifest, err := argocd.RenderApplication(argocd.ApplicationSpec{
		AppName:              app,
		ArgoNamespace:        bundle.ArgoNamespace,
		RepoURL:              project.HTTPCloneURL,
		Path:                 argocd.DefaultManifestPath,
		TargetRevision:       project.DefaultBranch,
		DestinationNamespace: destNamespace,
	})
	if err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Argo CD Application 생성 실패: %v", err))
		return
	}

	var kubeconfig []byte
	if uc.kubeconfig != nil {
		kubeconfig, err = uc.kubeconfig.GetKubeconfig(ctx, bundle.ClusterID)
		if err != nil {
			out.Warnings = append(out.Warnings,
				fmt.Sprintf("kubeconfig 조회 실패로 Argo CD Application 을 적용하지 못했습니다: %v", err))
			return
		}
	}

	manifests = append(manifests, manifest)
	if err := uc.applier.Apply(ctx, kubeconfig, manifests); err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Argo CD Application 적용 실패: %v", err))
		return
	}
	out.ArgoApplicationCreated = true
}
