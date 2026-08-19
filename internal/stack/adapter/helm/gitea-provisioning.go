package helm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// giteaOAuthSourceName 은 Gitea 에 등록할 OAuth 소스 이름이다.
//
// Keycloak 에 등록한 콜백 경로(/user/oauth2/<이름>/callback)의 <이름> 과 같아야
// 한다. 갈라지면 Gitea 가 콜백을 자기 소스로 찾지 못한다.
const giteaOAuthSourceName = "keycloak"

// giteaOIDCSettings 는 Gitea SSO 를 켤 수 있는지와 그 값을 준다.
func (o *Orchestrator) giteaOIDCSettings() (clientID, issuer string, ok bool) {
	provisioner := o.ssoProvisioner()
	if provisioner == nil {
		return "", "", false
	}
	clientID, found := provisioner.ClientIDFor("installing_gitea")
	if !found || strings.TrimSpace(clientID) == "" {
		return "", "", false
	}

	o.mu.Lock()
	issuer = o.toolOIDCIssuer
	o.mu.Unlock()
	if strings.TrimSpace(issuer) == "" {
		return "", "", false
	}
	return clientID, issuer, true
}

// giteaOAuthScript 는 Gitea 에 Keycloak OAuth 소스를 등록하는 셸 스크립트다.
//
// 비밀값은 인자가 아니라 표준입력으로 받는다 — 인자로 주면 파드의 프로세스
// 목록과 감사 로그에 남는다.
//
// 멱등해야 한다. 스택을 다시 배포하면 이 단계가 또 도는데, 이미 있는 소스를
// 다시 add 하면 Gitea 가 실패하므로 update 로 물러난다.
func giteaOAuthScript(clientID, issuer string) string {
	discovery := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"

	return fmt.Sprintf(`set -eu
read -r OIDC_SECRET
if gitea admin auth add-oauth --name %q --provider openidConnect \
     --key %q --secret "$OIDC_SECRET" --auto-discover-url %q 2>/tmp/err; then
  echo "gitea OAuth 소스 %s 등록"
else
  id=$(gitea admin auth list | awk '$2 == "%s" { print $1 }')
  if [ -z "$id" ]; then
    echo "gitea OAuth 등록 실패"; cat /tmp/err; exit 1
  fi
  gitea admin auth update-oauth --id "$id" --name %q --provider openidConnect \
    --key %q --secret "$OIDC_SECRET" --auto-discover-url %q
  echo "gitea OAuth 소스 %s 갱신"
fi
`, giteaOAuthSourceName, clientID, discovery, giteaOAuthSourceName,
		giteaOAuthSourceName, giteaOAuthSourceName, clientID, discovery, giteaOAuthSourceName)
}

// ensureGiteaSSOProvisioned 는 기동된 Gitea 에 OAuth 소스를 등록한다.
//
// Gitea 는 이 설정을 Helm values 로 받지 않고 CLI 로만 받는다. CLI 는 app.ini 로
// DB 를 찾으므로 별도 Job 이 아니라 기동된 파드에 exec 한다 — Job 으로 띄우면
// 설정과 데이터 볼륨을 다시 만들어 줘야 한다.
func (o *Orchestrator) ensureGiteaSSOProvisioned(ctx context.Context, namespace string) error {
	clientID, issuer, ok := o.giteaOIDCSettings()
	if !ok {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultStackNamespace
	}

	// CLI 는 기동이 끝나야 DB 에 붙는다.
	if _, err := o.runKubectl(ctx, "wait", "-n", namespace,
		"--for=condition=available", "--timeout=600s",
		"deployment/"+domain.GiteaReleaseName); err != nil {
		return fmt.Errorf("gitea 기동을 기다리지 못했습니다: %w", err)
	}

	secret, err := o.readSecretValue(ctx, namespace, SSOSecretName(clientID), "client-secret")
	if err != nil {
		return err
	}

	out, err := o.runKubectlWithStdin(ctx, secret+"\n",
		"exec", "-i", "-n", namespace, "deployment/"+domain.GiteaReleaseName,
		"-c", domain.GiteaReleaseName, "--", "sh", "-c", giteaOAuthScript(clientID, issuer))
	if err != nil {
		return fmt.Errorf("gitea OAuth 소스 등록 실패: %w", err)
	}
	slog.Info("gitea SSO 프로비저닝 완료", "output", strings.TrimSpace(string(out)))
	return nil
}
