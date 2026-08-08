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

	// GroupPath 는 조직 그룹 경로다. 프로젝트가 이 아래에 만들어진다.
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
