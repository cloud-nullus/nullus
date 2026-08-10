package port

import "context"

// SCMBundle 은 특정 스택에 묶인 프로비저닝 도구 묶음이다.
//
// GitLab 주소·토큰·레지스트리 종류가 모두 스택마다 다르므로 기동 시점에
// 하나로 만들 수 없다. 요청 시점에 스택을 보고 조립한다.
type SCMBundle struct {
	Provisioner SCMProvisioner
	Pipeline    PipelineConfigurator
	Registry    ImageRegistryResolver
	// Images 는 이미지 저장소를 지울 수단이다. 지원하지 않는 레지스트리에서는
	// nil 이며, 호출부는 이를 ErrImageDeletionUnsupported 와 같게 다뤄야 한다
	// — 조용히 건너뛰면 사용자는 이미지가 지워진 줄 안다.
	Images ImageRepositoryDeleter

	// Platform 은 이 묶음이 향하는 SCM 플랫폼이다.
	// 파이프라인 파일 형식이 여기서 갈리므로 렌더러까지 전달돼야 한다.
	Platform SCMPlatform
	// RepoAccessToken 은 플랫폼이 리포 범위 토큰 발급을 지원하지 않을 때
	// Argo CD·이미지 pull 인증에 재사용할 토큰이다.
	//
	// GitLab 에서는 비어 있다 — 프로젝트마다 최소 권한 토큰을 따로 발급한다.
	// GitHub 에는 리포 단위 토큰 API 가 없어 조직 PAT 를 그대로 쓴다.
	RepoAccessToken string

	// GroupPath 는 프로젝트가 만들어질 네임스페이스다.
	// GitLab 은 그룹 경로, GitHub 은 organization/사용자 이름이다.
	GroupPath string
	// ArgoNamespace 는 Argo CD 가 설치된 네임스페이스다.
	// Application 리소스를 여기에 만들어야 컨트롤러가 인식한다.
	ArgoNamespace string
	// ClusterID 는 Application 을 적용할 클러스터다.
	ClusterID string
	// AccessDomain / GatewayName 은 배포된 앱을 외부에 노출할 때 쓴다.
	// 비어 있으면 앱은 클러스터 내부에서만 접근 가능하다.
	AccessDomain string
	GatewayName  string
}

// SCMBundleFactory 는 스택에 맞는 도구 묶음을 조립한다.
type SCMBundleFactory interface {
	For(ctx context.Context, stackID string) (*SCMBundle, error)
}
