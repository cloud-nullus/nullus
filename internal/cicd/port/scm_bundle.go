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

	// CIJobs 는 CI 서버에 job 을 만드는 수단이다.
	//
	// GitLab CI·GitHub Actions 는 파이프라인 정의를 푸시하면 자동 감지하므로
	// nil 이다. Jenkins 는 job 이 먼저 존재해야 하므로 이 자리가 채워진다.
	// 호출부는 nil 이면 건너뛴다 — 기존 경로 무영향.
	CIJobs CIJobProvisioner
	// Webhooks 는 저장소에 push webhook 을 거는 수단이다.
	// Jenkins multibranch 가 새 커밋을 알려면 필요하다. 지원하지 않으면 nil.
	Webhooks SCMWebhookProvisioner
	// CITrigger 는 CI 서버의 job 을 지금 실행시킨다. 지원하지 않으면 nil.
	//
	// 스택에 묶인 파이프라인의 "배포 실행" 이 이 자리로 온다 — 플랫폼이
	// 빌드하지 않고 러너에게 넘긴다.
	CITrigger CIBuildTrigger
	// CIBuilds 는 CI 서버의 빌드 이력을 읽는다. 지원하지 않으면 nil.
	CIBuilds CIBuildReader
	// CIBaseURL 은 CI 서버의 주소다. webhook 대상 주소를 만드는 데 쓴다.
	CIBaseURL string
	// SCMInClusterURL 은 클러스터 안에서 SCM 에 닿는 주소다.
	//
	// Provisioner 의 base URL 과 구분한다 — 그쪽은 API 서버가 쓰는 주소라
	// 로컬 실행에서 우회 주소(localhost 포트포워드 등)일 수 있다. 반면 이 값은
	// Jenkins·Argo CD 처럼 클러스터 안에서 도는 소비자가 쓰므로 항상 서비스
	// DNS 여야 한다. 둘을 같은 값으로 두면 job 이
	// "Unknown server: http://localhost:3000" 으로 죽는다.
	SCMInClusterURL string
	// Credentials 는 CI 변수 저장소가 없는 SCM(Gitea)에서 파이프라인 자격증명을
	// OpenBao → ESO → K8s Secret 평면으로 나른다. 지원하지 않으면 nil.
	Credentials PipelineCredentialPlane

	// OrgMembers 는 사람을 조직에 넣는 수단이다. 지원하지 않으면 nil.
	//
	// 플랫폼이 만든 저장소는 자동화 계정 소유의 private 조직 안에 있어서,
	// 그대로 두면 정작 소스를 밀어야 할 사람이 보지도 못한다.
	OrgMembers OrgMemberProvisioner

	// RegistryCredentials 는 스택이 설치한 레지스트리(Harbor·Nexus)의 자격증명을
	// 푼다. 그 값은 스택 설치가 OpenBao 에 만들어 두므로 사용자에게 다시 받을
	// 이유가 없다. 플랫폼이 소유하지 않는 레지스트리에서는 아무것도 돌려주지 않는다.
	RegistryCredentials RegistryCredentialResolver

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

// PipelineCredentialPlane 은 파이프라인 자격증명을 준비하고 그것을 클러스터에
// 반영할 매니페스트를 돌려준다.
//
// 적용은 하지 않는다 — 클러스터 접근은 유스케이스가 한곳에서 맡는다.
type PipelineCredentialPlane interface {
	Provision(ctx context.Context, app string, vars []PipelineVariable) (manifest string, err error)
}

// PipelineVariable 은 파이프라인이 환경변수로 읽을 값 하나다.
type PipelineVariable struct {
	Key   string
	Value string
}

// RegistryCredentialResolver 는 CI 가 레지스트리 로그인에 쓸 변수 값을 푼다.
//
// 요청한 변수 중 아는 것만 채워 돌려준다. 모르는 것은 손대지 않는다 — 조용히
// 빈 값을 채우면 CI 가 엉뚱한 자격증명으로 로그인을 시도한다.
type RegistryCredentialResolver interface {
	Resolve(ctx context.Context, variables []string) (map[string]string, error)
}

// SCMBundleFactory 는 스택에 맞는 도구 묶음을 조립한다.
type SCMBundleFactory interface {
	For(ctx context.Context, stackID string) (*SCMBundle, error)
}
