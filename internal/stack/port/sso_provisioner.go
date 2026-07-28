package port

import "context"

// SSOClientSpec 은 OIDC 클라이언트 등록에 필요한 정보다.
type SSOClientSpec struct {
	// StepName 은 대상 도구를 식별한다 (예: installing_grafana).
	StepName string
	// ClientSecret 은 Nullus 가 생성해 OpenBao 에 기록한 값이다.
	ClientSecret string
}

// SSOProvisioner 는 설치된 OSS 의 OIDC 클라이언트를 IdP 에 등록한다.
//
// stack 모듈이 auth 모듈의 구현을 직접 import 하면 모듈 간 직접 의존 금지
// 규칙에 어긋나므로, 여기에 인터페이스를 두고 main 에서 구현체를 주입한다.
type SSOProvisioner interface {
	// ClientIDFor 는 스택 네임스페이스가 적용된 client ID 를 돌려준다.
	ClientIDFor(stepName string) (string, bool)
	// ToolSteps 는 SSO 대상 스텝 목록이다.
	ToolSteps() []string
	// Provision 은 클라이언트를 등록하거나 갱신한다.
	Provision(ctx context.Context, spec SSOClientSpec) error
}

// SSOProvisionerFactory 는 스택별 접속 도메인/슬러그로 provisioner 를 만든다.
type SSOProvisionerFactory func(accessDomain, stackSlug string) SSOProvisioner
