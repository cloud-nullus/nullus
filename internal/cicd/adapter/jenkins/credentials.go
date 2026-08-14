package jenkins

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	// SecretProvider 는 자격증명을 보관하는 시크릿 백엔드다.
	SecretProvider = "openbao"

	// AdminUser 는 차트가 만드는 Jenkins 관리자 계정이다.
	// 스택 설치의 provisioning_secrets 가 같은 이름으로 자격증명을 만든다
	// (internal/stack/domain.JenkinsAdminUser). 두 값이 갈라지면 인증이 실패한다.
	AdminUser = "admin"

	defaultCredentialEnv = "dev"
)

// SecretStore 는 자격증명 조회에 필요한 최소 동작만 노출한다.
//
// 스택 범위 접근을 쓴다 — OpenBao 는 스택마다 배포되므로 전역 주소가 하나일 수
// 없다. gitea.SecretStore 와 같은 형태지만 각 어댑터가 자기 계약을 소유한다.
type SecretStore interface {
	GetTokenForStack(ctx context.Context, provider, stackID, path string) (string, error)
}

// CredentialResolver 는 port.CICredentialResolver 의 Jenkins 구현체다.
//
// 토큰을 발급하지 않고 읽기만 한다. Jenkins 관리자 비밀번호는 스택 설치의
// provisioning_secrets 가 이미 만들어 OpenBao 에 넣어 두었고, 같은 값을 ESO 가
// K8s Secret 으로 동기화해 컨트롤러가 쓴다 — 여기서 새로 발급하면 그 둘과
// 어긋나 컨트롤러가 자기 비밀번호를 모르게 된다.
type CredentialResolver struct {
	secrets SecretStore
}

// NewCredentialResolver 는 CredentialResolver 를 만든다.
func NewCredentialResolver(secrets SecretStore) *CredentialResolver {
	return &CredentialResolver{secrets: secrets}
}

// AdminPasswordPath 는 관리자 비밀번호의 시크릿 경로다.
//
// 스택 모듈이 쓰는 경로와 정확히 같아야 한다
// (helm/secret-provisioning.go 의 managedSecrets → "cicd/jenkins/admin-password").
// 갈라지면 설치는 성공하는데 job 생성만 401 로 실패한다.
func AdminPasswordPath(env, orgID string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		env = defaultCredentialEnv
	}
	return fmt.Sprintf("kv/nullus/%s/%s/cicd/jenkins/admin-password", env, strings.TrimSpace(orgID))
}

// ResolveCICredential 은 스택의 Jenkins 관리자 자격증명을 읽는다.
func (r *CredentialResolver) ResolveCICredential(
	ctx context.Context,
	spec port.SCMTokenSpec,
) (user, secret string, err error) {
	if r == nil || r.secrets == nil {
		return "", "", fmt.Errorf("jenkins 자격증명 저장소가 배선되지 않았습니다")
	}
	orgID := strings.TrimSpace(spec.OrgID)
	if orgID == "" {
		return "", "", fmt.Errorf("org_id is required to resolve jenkins credentials")
	}
	stackID := strings.TrimSpace(spec.StackID)
	if stackID == "" {
		return "", "", fmt.Errorf("stack_id is required to locate the secret store")
	}

	path := AdminPasswordPath(spec.Env, orgID)
	password, err := r.secrets.GetTokenForStack(ctx, SecretProvider, stackID, path)
	if err != nil {
		return "", "", fmt.Errorf("read jenkins admin password (%s): %w", path, err)
	}
	if strings.TrimSpace(password) == "" {
		return "", "", fmt.Errorf("jenkins 관리자 비밀번호가 비어 있습니다 (%s)", path)
	}
	return AdminUser, password, nil
}
