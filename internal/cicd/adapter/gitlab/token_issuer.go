package gitlab

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	// SecretProvider 는 토큰을 보관할 시크릿 백엔드다.
	SecretProvider = "openbao"

	// AutomationTokenName 은 Nullus 가 발급하는 PAT 의 이름이다.
	// 사람이 만든 토큰과 구분되어야 정리(revoke) 대상을 특정할 수 있다.
	AutomationTokenName = "nullus-automation"

	// toolboxDeployment 는 gitlab-rails 를 실행할 수 있는 워크로드다.
	toolboxDeployment = "deploy/gitlab-toolbox"
	toolboxContainer  = "toolbox"

	defaultTokenEnv = "dev"
)

// KubectlRunner 는 kubectl 실행을 추상화한다.
//
// 다른 모듈의 동일 타입을 재사용하지 않는다 — 모듈 간 직접 import 를 피하기 위해
// CI/CD 컨텍스트가 자기 계약을 소유한다.
type KubectlRunner func(ctx context.Context, kubeconfig []byte, args ...string) ([]byte, error)

// SecretStore 는 토큰 보관에 필요한 최소 동작만 노출한다.
//
// 스택 범위 접근을 쓴다 — OpenBao 는 스택마다 배포되므로 전역 주소가 하나일 수
// 없고, 해당 스택의 저장소에 넣어야 ESO 와 같은 곳을 본다.
type SecretStore interface {
	GetTokenForStack(ctx context.Context, provider, stackID, path string) (string, error)
	PutTokenForStack(ctx context.Context, provider, stackID, path, value string) error
}

// TokenIssuer 는 GitLab API 토큰을 확보한다.
//
// GitLab 을 외부에 노출하지 않고도 동작해야 하므로 API 가 아니라 toolbox 파드의
// gitlab-rails 콘솔로 PAT 를 발급한다. Runner 등록 토큰 발견과 같은 방식이다.
// PAT 값은 생성 시점에만 읽을 수 있어 발급 즉시 시크릿 저장소에 보관한다.
type TokenIssuer struct {
	kubeconfig port.KubeconfigProvider
	runKubectl KubectlRunner
	secrets    SecretStore
}

// NewTokenIssuer 는 TokenIssuer 를 만든다.
func NewTokenIssuer(kubeconfig port.KubeconfigProvider, runKubectl KubectlRunner, secrets SecretStore) *TokenIssuer {
	return &TokenIssuer{kubeconfig: kubeconfig, runKubectl: runKubectl, secrets: secrets}
}

// TokenSecretPath 는 API 토큰의 시크릿 경로다.
//
// 스택 모듈의 규약(kv/nullus/{env}/{org}/{module}/{provider}/...)을 따른다.
func TokenSecretPath(env, orgID string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		env = defaultTokenEnv
	}
	return fmt.Sprintf("kv/nullus/%s/%s/cicd/gitlab/api-token", env, strings.TrimSpace(orgID))
}

// EnsureToken 은 보관된 토큰을 돌려주고, 없거나 Force 면 새로 발급한다.
func (t *TokenIssuer) EnsureToken(ctx context.Context, spec port.SCMTokenSpec) (string, error) {
	namespace := strings.TrimSpace(spec.Namespace)
	if namespace == "" {
		return "", fmt.Errorf("namespace is required to issue a gitlab token")
	}
	orgID := strings.TrimSpace(spec.OrgID)
	if orgID == "" {
		return "", fmt.Errorf("org_id is required to issue a gitlab token")
	}
	stackID := strings.TrimSpace(spec.StackID)
	if stackID == "" {
		return "", fmt.Errorf("stack_id is required to locate the secret store")
	}

	path := TokenSecretPath(spec.Env, orgID)

	if !spec.Force && t.secrets != nil {
		// 조회 실패는 치명적이지 않다 — 저장소가 아직 없으면 새로 발급하면 된다.
		if stored, err := t.secrets.GetTokenForStack(ctx, SecretProvider, stackID, path); err == nil {
			if token := strings.TrimSpace(stored); token != "" {
				return token, nil
			}
		}
	}

	token, err := t.issueViaRails(ctx, spec.ClusterID, namespace)
	if err != nil {
		return "", err
	}

	if t.secrets != nil {
		if err := t.secrets.PutTokenForStack(ctx, SecretProvider, stackID, path, token); err != nil {
			// 보관하지 못하면 다음 호출이 또 발급하며 이전 토큰을 폐기한다.
			// 동시 요청이 서로의 토큰을 무효화할 수 있으므로 실패로 다룬다.
			return "", fmt.Errorf("발급한 GitLab 토큰 저장 실패 (%s): %w", path, err)
		}
	}
	return token, nil
}

// issueViaRails 는 toolbox 파드에서 PAT 를 발급한다.
func (t *TokenIssuer) issueViaRails(ctx context.Context, clusterID, namespace string) (string, error) {
	var kubeconfig []byte
	if t.kubeconfig != nil {
		var err error
		kubeconfig, err = t.kubeconfig.GetKubeconfig(ctx, clusterID)
		if err != nil {
			return "", fmt.Errorf("kubeconfig 조회 실패 (cluster %s): %w", clusterID, err)
		}
	}
	if t.runKubectl == nil {
		return "", fmt.Errorf("kubectl runner is not configured")
	}

	args := []string{
		"-n", namespace,
		"exec", toolboxDeployment,
		"-c", toolboxContainer,
		"--", "bash", "-lc",
		fmt.Sprintf("gitlab-rails runner %q", buildTokenIssueScript()),
	}

	out, err := t.runKubectl(ctx, kubeconfig, args...)
	if err != nil {
		return "", fmt.Errorf("GitLab 토큰 발급 실패 (%s/%s): %w", namespace, toolboxDeployment, err)
	}

	token := parseRailsTokenOutput(string(out))
	if token == "" {
		return "", fmt.Errorf("GitLab 응답에서 토큰을 찾지 못했습니다 (출력: %s)",
			strings.TrimSpace(string(out)))
	}
	return token, nil
}

// buildTokenIssueScript 는 root 사용자의 자동화 PAT 를 재발급하는 Rails 스크립트다.
//
// 이전 자동화 토큰을 먼저 폐기한다 — 재발급이 반복되면 계정에 죽은 토큰이
// 쌓이고, 어느 것이 유효한지 사람이 구분할 수 없게 된다.
//
// 스크립트에 큰따옴표를 쓰지 않는다. 이 문자열은 `bash -lc "gitlab-rails runner \"...\""`
// 형태로 두 겹의 셸 인용을 통과하므로, 큰따옴표가 하나라도 있으면 Ruby 구문이 깨진다.
// (SQL LIKE 대신 Ruby select 를 쓰는 이유도 같다 — 토큰 수가 적어 성능 차이는 없다.)
func buildTokenIssueScript() string {
	return strings.Join([]string{
		`u = User.find_by_username('root')`,
		`raise 'root user not found' if u.nil?`,
		`u.personal_access_tokens.active.select { |t| t.name.to_s.start_with?('` + AutomationTokenName + `') }.each { |t| t.revoke! }`,
		`t = u.personal_access_tokens.create!(name: '` + AutomationTokenName + `', scopes: ['api'], expires_at: 365.days.from_now)`,
		`t.set_token(SecureRandom.hex(20))`,
		`t.save!`,
		`puts t.token`,
	}, "; ")
}

// parseRailsTokenOutput 은 kubectl exec 출력에서 토큰 줄만 고른다.
//
// "Defaulted container ..." 같은 안내가 섞여 들어오므로 공백이 없는 마지막 줄을 택한다.
func parseRailsTokenOutput(output string) string {
	token := ""
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		candidate := strings.TrimSpace(line)
		if candidate == "" || strings.Contains(candidate, " ") {
			continue
		}
		token = candidate
	}
	return token
}
