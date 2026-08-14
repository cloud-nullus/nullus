package gitea

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	// SecretProvider 는 토큰을 보관할 시크릿 백엔드다.
	SecretProvider = "openbao"

	// AutomationTokenName 은 Nullus 가 발급하는 액세스 토큰의 이름이다.
	// 사람이 만든 토큰과 구분되어야 재발급 시 정리 대상을 특정할 수 있다.
	AutomationTokenName = "nullus-automation"

	// giteaPodSelector 는 gitea CLI 를 실행할 파드를 고르는 레이블이다.
	//
	// 워크로드 종류를 고정하지 않는다 — 차트는 버전·설정에 따라 Deployment 로도
	// StatefulSet 으로도 배포한다(12.7.0 은 Deployment). 종류를 박아 두면
	// "statefulsets.apps \"gitea\" not found" 로 토큰 발급이 죽고, 스택은 정상
	// 설치됐는데 파이프라인 생성만 실패하는 원인이 먼 오류가 된다.
	giteaPodSelector = "app.kubernetes.io/name=gitea"
	giteaContainer   = "gitea"

	// AutomationUser 는 차트가 만드는 관리자 계정이다.
	// 스택 설치의 provisioning_secrets 가 같은 이름으로 자격증명을 만든다
	// (internal/stack/domain.GiteaAdminUser). 갈라지면 인증이 실패한다.
	AutomationUser = "gitea_admin"

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

// TokenIssuer 는 Gitea 액세스 토큰을 확보한다.
//
// Gitea 를 외부에 노출하지 않고도 동작해야 하므로 API 가 아니라 파드 안의
// gitea CLI 로 발급한다 — GitLab 이 toolbox 의 rails 콘솔을 쓰는 것과 같은
// 이유다. 토큰 값은 발급 시점에만 읽을 수 있어 즉시 시크릿 저장소에 보관한다.
type TokenIssuer struct {
	kubeconfig port.KubeconfigProvider
	runKubectl KubectlRunner
	secrets    SecretStore
}

// NewTokenIssuer 는 TokenIssuer 를 만든다.
func NewTokenIssuer(kubeconfig port.KubeconfigProvider, runKubectl KubectlRunner, secrets SecretStore) *TokenIssuer {
	return &TokenIssuer{kubeconfig: kubeconfig, runKubectl: runKubectl, secrets: secrets}
}

// TokenSecretPath 는 액세스 토큰의 시크릿 경로다.
//
// 스택 모듈의 규약(kv/nullus/{env}/{org}/{module}/{provider}/...)을 따른다.
func TokenSecretPath(env, orgID string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		env = defaultTokenEnv
	}
	return fmt.Sprintf("kv/nullus/%s/%s/cicd/gitea/api-token", env, strings.TrimSpace(orgID))
}

// EnsureToken 은 보관된 토큰을 돌려주고, 없거나 Force 면 새로 발급한다.
func (t *TokenIssuer) EnsureToken(ctx context.Context, spec port.SCMTokenSpec) (string, error) {
	namespace := strings.TrimSpace(spec.Namespace)
	if namespace == "" {
		return "", fmt.Errorf("namespace is required to issue a gitea token")
	}
	orgID := strings.TrimSpace(spec.OrgID)
	if orgID == "" {
		return "", fmt.Errorf("org_id is required to issue a gitea token")
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

	token, err := t.issueViaCLI(ctx, spec.ClusterID, namespace)
	if err != nil {
		return "", err
	}

	if t.secrets != nil {
		if err := t.secrets.PutTokenForStack(ctx, SecretProvider, stackID, path, token); err != nil {
			// 보관하지 못하면 다음 호출이 또 발급하며 이전 토큰을 폐기한다.
			// 동시 요청이 서로의 토큰을 무효화할 수 있으므로 실패로 다룬다.
			return "", fmt.Errorf("발급한 Gitea 토큰 저장 실패 (%s): %w", path, err)
		}
	}
	return token, nil
}

// issueViaCLI 는 Gitea 파드에서 액세스 토큰을 발급한다.
//
// 같은 이름의 토큰이 이미 있으면 발급이 실패하므로 먼저 지운다. 그래서 이
// 경로는 항상 "새 토큰" 을 돌려준다 — 보관에 실패하면 이전 토큰까지 잃으므로
// EnsureToken 이 저장 실패를 오류로 다룬다.
func (t *TokenIssuer) issueViaCLI(ctx context.Context, clusterID, namespace string) (string, error) {
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

	// gitea CLI 는 반드시 git 사용자로 돌아야 한다. root 로 실행하면
	// "Gitea is not supposed to be run as root" 로 거부한다.
	//
	// 토큰 삭제는 없을 수도 있으므로 실패를 무시한다(|| true). 발급만 성패를 가른다.
	script := fmt.Sprintf(
		"gitea admin user delete-access-token --username %s --token-name %s >/dev/null 2>&1 || true; "+
			"gitea admin user generate-access-token --username %s --token-name %s --scopes write:organization,write:repository,write:user --raw",
		AutomationUser, AutomationTokenName, AutomationUser, AutomationTokenName)

	pod, err := t.resolveGiteaPod(ctx, kubeconfig, namespace)
	if err != nil {
		return "", err
	}

	args := []string{
		"-n", namespace,
		"exec", pod,
		"-c", giteaContainer,
		"--", "sh", "-lc", script,
	}

	out, err := t.runKubectl(ctx, kubeconfig, args...)
	if err != nil {
		return "", fmt.Errorf("Gitea 토큰 발급 실패 (%s/%s): %w", namespace, pod, err)
	}

	token := parseIssuedToken(string(out))
	if token == "" {
		return "", fmt.Errorf("Gitea 토큰 발급 출력에서 토큰을 찾지 못했습니다 (%s)", namespace)
	}
	return token, nil
}

// resolveGiteaPod 은 gitea CLI 를 실행할 파드 이름을 찾는다.
//
// 파드를 못 찾았는데 그대로 exec 으로 넘어가면 빈 이름으로 호출해 엉뚱한
// 오류가 나므로 여기서 끊는다.
func (t *TokenIssuer) resolveGiteaPod(ctx context.Context, kubeconfig []byte, namespace string) (string, error) {
	args := []string{
		"-n", namespace,
		"get", "pod",
		"-l", giteaPodSelector,
		"--field-selector=status.phase=Running",
		"-o", "jsonpath={.items[0].metadata.name}",
	}

	out, err := t.runKubectl(ctx, kubeconfig, args...)
	if err != nil {
		return "", fmt.Errorf("Gitea 파드 조회 실패 (%s, %s): %w", namespace, giteaPodSelector, err)
	}

	pod := strings.TrimSpace(string(out))
	if pod == "" {
		return "", fmt.Errorf("Gitea 파드를 찾지 못했습니다 (%s, %s)", namespace, giteaPodSelector)
	}
	return pod, nil
}

// parseIssuedToken 은 CLI 출력에서 토큰만 골라낸다.
//
// --raw 를 줘도 셸 초기화 메시지나 경고가 앞뒤에 섞일 수 있어 마지막 비어 있지
// 않은 줄을 쓴다. 토큰은 공백을 포함하지 않으므로 공백이 있는 줄은 버린다.
func parseIssuedToken(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.ContainsAny(line, " \t") {
			continue
		}
		return line
	}
	return ""
}
