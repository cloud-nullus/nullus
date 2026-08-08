package port

import "context"

// SCMTokenSpec 은 SCM API 토큰 확보 요청이다.
type SCMTokenSpec struct {
	// StackID 는 시크릿 저장소를 고르는 키다.
	//
	// OpenBao 는 스택마다 배포되므로 전역 주소가 하나일 수 없다.
	// 스택 범위로 접근해야 해당 스택의 OpenBao 에 저장된다.
	StackID string
	// ClusterID 는 kubeconfig 를 찾는 키다.
	ClusterID string
	// Namespace 는 SCM(GitLab)이 설치된 네임스페이스다.
	Namespace string
	// OrgID / Env 는 시크릿 저장 경로를 구성한다.
	OrgID string
	Env   string
	// Force 가 true 면 저장된 토큰을 무시하고 새로 발급한다.
	// 토큰이 만료되거나 폐기돼 401 을 받은 호출자가 쓴다.
	Force bool
}

// SCMTokenIssuer 는 SCM 플랫폼의 API 토큰을 확보한다.
//
// GitLab 은 PAT 값을 생성 시점에만 돌려주므로, 발급한 값을 시크릿 저장소에
// 보관하고 이후에는 그것을 재사용한다. 저장된 값이 없을 때만 새로 발급한다.
type SCMTokenIssuer interface {
	EnsureToken(ctx context.Context, spec SCMTokenSpec) (string, error)
}
