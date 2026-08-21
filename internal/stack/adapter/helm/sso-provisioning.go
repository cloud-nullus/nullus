package helm

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// OIDC client secret 은 시크릿 평면에서 함께 관리한다.
//
// PRD 5.2 는 "OIDC client secret 은 OpenBao 경유로만 주입" 을 규정한다.
// 값의 생성 주체는 Nullus 이고 Keycloak 에는 push 한다 — Keycloak 이 유실돼도
// OpenBao 에서 복원할 수 있어야 하기 때문이다.
//
//	provisioning_secrets  랜덤 생성 → OpenBao write → ExternalSecret → K8s Secret
//	provisioning_sso      OpenBao read → Keycloak client upsert
//	installing_{oss}      K8s Secret 참조

// ssoClientSecretPath 는 client secret 의 OpenBao 경로 접미사다.
func ssoClientSecretPath(clientID string) string {
	return fmt.Sprintf("auth/%s/client-secret", clientID)
}

// GitLabOIDCSecretName 은 GitLab 이 omniauth provider 로 읽는 Secret 이름이다.
//
// GitLab 은 provider 설정을 client secret 하나가 아니라 "블록 전체" 로 받는다.
// 그래서 다른 도구처럼 client-secret 키만 담은 Secret 으로는 안 되고, ESO 의
// target.template 으로 provider YAML 을 만들어 넣는다.
const GitLabOIDCSecretName = "gitlab-oidc-provider" // #nosec G101 -- Secret 리소스 이름

// gitlabOmniauthProvider 는 GitLab 이 읽는 omniauth provider 블록이다.
//
// name 은 콜백 경로와 짝이다(/users/auth/<name>/callback). Keycloak 에 등록한
// redirect 와 갈라지면 로그인이 "redirect_uri" 오류로 막힌다.
// client secret 은 ESO 템플릿에서 채운다 — 여기 값을 박으면 ExternalSecret
// 정의에 평문이 남는다.
func gitlabOmniauthProvider(clientID, issuer, baseURL string) string {
	// omniauth-openid_connect 의 client_options 는 scheme=https / port=443 이
	// 기본값이라 issuer 의 스킴·포트를 따라가지 않는다. 분해해 함께 넣는다.
	scheme, host, port, path := splitIssuerEndpoint(issuer)

	// openid_connect 젬은 discovery URL 을 항상 HTTPS 로 만든다(swd.rb 의
	// url_builder 가 URI::HTTPS 로 고정). 그래서 issuer 가 http 면 discovery
	// 단계에서 평문 포트에 TLS 로 붙어 깨진다:
	//
	//	Ssl connect returned=1 ... state=error: wrong version number
	//
	// client_options 를 고쳐도 소용없다 — discovery 가 그보다 먼저 돈다.
	// http 에서는 discovery 를 끄고 엔드포인트를 직접 준다. https 면 discovery 가
	// 정상 동작하고, 엔드포인트가 바뀌어도 따라가므로 그쪽이 더 견고하다.
	// pkce 는 항상 켠다. SSO 프로비저너가 gitlab 클라이언트를 PKCE(S256) 요구로
	// 등록하므로, 끄면 콜백이 "Missing parameter: code challenge method" 로
	// 깨진다. 젬의 기본 pkce_options 가 이미 S256 이라 방식은 지정하지 않는다.
	discovery := "true"
	endpoints := ""
	if scheme == "http" {
		discovery = "false"
		// client_options 안에서는 전체 URL 이 아니라 경로다
		// (scheme/host/port 와 합쳐진다).
		// authorization/token/userinfo 는 scheme·host·port 와 합쳐지는 경로다.
		// 반면 jwks_uri 는 젬이 그대로 HTTP 요청 대상으로 쓰므로 전체 URL 이어야
		// 한다(http_client.get(client_options.jwks_uri)). 경로만 주면 호스트가
		// 비어 ":80" 으로 붙는다.
		base := fmt.Sprintf("%s://%s:%d", scheme, host, port)
		endpoints = fmt.Sprintf(`
    authorization_endpoint: "%s/protocol/openid-connect/auth"
    token_endpoint: "%s/protocol/openid-connect/token"
    userinfo_endpoint: "%s/protocol/openid-connect/userinfo"
    jwks_uri: "%s%s/protocol/openid-connect/certs"`, path, path, path, base, path)
	}

	return fmt.Sprintf(`name: "openid_connect"
label: "Keycloak"
args:
  name: "openid_connect"
  scope: ["openid","profile","email"]
  response_type: "code"
  issuer: "%s"
  discovery: %s
  uid_field: "preferred_username"
  pkce: true
  client_options:
    identifier: "%s"
    secret: "{{ .clientSecret }}"
    redirect_uri: "%s/users/auth/openid_connect/callback"
    scheme: "%s"
    host: "%s"
    port: %d%s
`, strings.TrimRight(issuer, "/"), discovery, clientID, baseURL, scheme, host, port, endpoints)
}

// splitIssuerEndpoint 는 issuer URL 을 스킴·호스트·포트로 나눈다.
//
// 포트가 없으면 스킴의 기본값을 쓴다 — 비워 두면 젬이 443 을 가정해 http issuer
// 에서 곧바로 어긋난다.
func splitIssuerEndpoint(issuer string) (scheme, host string, port int, path string) {
	parsed, err := url.Parse(strings.TrimSpace(issuer))
	if err != nil || parsed.Host == "" {
		return "https", "", 443, ""
	}

	scheme = parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}
	host = parsed.Hostname()

	port = 443
	if scheme == "http" {
		port = 80
	}
	if p := parsed.Port(); p != "" {
		if n, convErr := strconv.Atoi(p); convErr == nil {
			port = n
		}
	}
	return scheme, host, port, strings.TrimRight(parsed.Path, "/")
}

// ArgoCDSecretName 은 ArgoCD 가 읽는 Secret 이름이다.
// admin 해시와 OIDC client secret 이 한 Secret 에 공존하므로 ESO 가 단독 소유한다.
const ArgoCDSecretName = "argocd-secret" // #nosec G101 -- Secret 리소스 이름

// SSOSecretName 은 client secret 이 복제될 Kubernetes Secret 이름이다.
func SSOSecretName(clientID string) string {
	return fmt.Sprintf("%s-oidc", clientID)
}

// SetSSOProvisionerFactory 는 스택별 SSO provisioner 생성기를 주입한다.
func (o *Orchestrator) SetSSOProvisionerFactory(factory port.SSOProvisionerFactory) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ssoFactory = factory
}

// ssoProvisioner 는 이 스택에 맞는 provisioner 를 만든다.
func (o *Orchestrator) ssoProvisioner() port.SSOProvisioner {
	o.mu.Lock()
	factory := o.ssoFactory
	cfg := o.stackConfig
	namespace := o.namespace
	o.mu.Unlock()

	if factory == nil {
		return nil
	}
	accessDomain := ""
	if cfg != nil {
		accessDomain = strings.TrimSpace(cfg.AccessDomain)
	}
	// client ID 는 공용 realm 안에서 스택을 구분하는 이름이다. 예전에는 접속
	// 도메인에서 뽑았는데, 도메인은 스택마다 다르다는 보장이 없다 — 로컬처럼
	// 모든 스택이 같은 도메인(nullus.local)을 쓰면 서로의 등록을 덮어쓴다.
	// 네임스페이스는 스택마다 하나씩이고 NewOrchestrator 가 항상 채우므로
	// (비면 "nullus") 이 값이 스택 식별자로 가장 안정적이다.
	return factory(accessDomain, strings.TrimSpace(namespace))
}

// ssoManagedSecrets 는 SSO client secret 에 대한 관리 항목을 만든다.
//
// 도구별 client ID 가 스택 단위로 네임스페이싱되므로 provisioner 를 통해 얻는다.
func (o *Orchestrator) ssoManagedSecrets() []ManagedSecret {
	provisioner := o.ssoProvisioner()
	if provisioner == nil {
		return nil
	}

	steps := provisioner.ToolSteps()
	items := make([]ManagedSecret, 0, len(steps))
	for _, step := range steps {
		if !o.isStepEnabled(step) {
			continue // 설치하지 않는 도구는 클라이언트도 만들지 않는다
		}
		clientID, ok := provisioner.ClientIDFor(step)
		if !ok || strings.TrimSpace(clientID) == "" {
			continue
		}
		if step == "installing_argocd" {
			// ArgoCD 는 예외다. 하나의 Secret(argocd-secret)에 admin 비밀번호와
			// OIDC client secret 이 함께 들어가므로 existingSecret 치환이 성립하지
			// 않는다. ESO 가 Secret 전체를 소유하고 두 값을 함께 담는다.
			// 차트 쪽은 configs.secret.createSecret=false 로 생성을 끈다.
			items = append(items, ManagedSecret{
				TargetSecret:    ArgoCDSecretName,
				Consumer:        step,
				RestartRequired: true,
				Entries: []SecretEntry{
					{PathSuffix: ssoClientSecretPath(clientID), TargetKey: "oidc.keycloak.clientSecret"},
					{PathSuffix: "pipeline/argocd/admin-password", TargetKey: "clearPassword"},
					// IdP 가 죽어도 들어갈 수단을 남긴다. ArgoCD 는 bcrypt 해시를
					// admin.password 에서 읽고, mtime 이 없으면 설정을 무시한다.
					// ESO 가 이 Secret 을 단독 소유하므로(creationPolicy=Owner)
					// ArgoCD 가 스스로 써넣어도 다음 동기화에 되돌려진다 —
					// 여기 담지 않으면 비밀번호 로그인이 성립하지 않는다.
					{
						PathSuffix: "pipeline/argocd/admin-password-bcrypt",
						TargetKey:  "admin.password",
						DeriveFrom: "pipeline/argocd/admin-password",
						Derive:     bcryptHash,
					},
					{
						PathSuffix: "pipeline/argocd/admin-password-mtime",
						TargetKey:  "admin.passwordMtime",
						Fixed:      time.Now().UTC().Format(time.RFC3339),
					},
					// server.secretkey 가 없으면 argocd-server 와 dex-server 가
					// 기동 즉시 panic 한다("server.secretkey is missing").
					// 차트는 이 값을 configs.secret.extra 로 넣지만(values.go),
					// 그 경로는 차트가 Secret 을 만들 때만 유효하다. 여기서는
					// createSecret=false 로 꺼 두므로 ESO 가 함께 담아야 한다.
					{PathSuffix: "pipeline/argocd/server-secretkey", TargetKey: "server.secretkey"},
				},
			})
			continue
		}

		if step == "installing_gitlab" {
			// GitLab 은 provider 블록 전체를 담은 Secret 을 읽는다. 그 블록에
			// issuer 가 들어가므로, issuer 가 없으면 만들지 않는다 — 깨진 설정을
			// 읽히면 GitLab 이 omniauth 초기화에서 실패한다.
			block := o.gitlabOmniauthProviderBlock(clientID)
			if block == "" {
				continue
			}
			items = append(items, ManagedSecret{
				TargetSecret:    GitLabOIDCSecretName,
				Consumer:        step,
				RestartRequired: true,
				Entries: []SecretEntry{
					{PathSuffix: ssoClientSecretPath(clientID), TargetKey: "clientSecret"},
				},
				TemplateData: map[string]string{"provider": block},
			})
			continue
		}

		items = append(items, ManagedSecret{
			TargetSecret:    SSOSecretName(clientID),
			Consumer:        step,
			RestartRequired: true,
			Entries: []SecretEntry{
				{PathSuffix: ssoClientSecretPath(clientID), TargetKey: "client-secret"},
			},
		})
	}
	return items
}

// bcryptHash 는 평문 비밀번호의 bcrypt 해시를 만든다. ArgoCD 가 admin.password
// 에서 기대하는 형식이다.
func bcryptHash(plaintext string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt 해시 생성 실패: %w", err)
	}
	return string(hashed), nil
}

// runSSOProvisioning 은 OpenBao 의 client secret 을 읽어 Keycloak 에 등록한다.
//
// Keycloak 이 기동된 뒤여야 하므로 설치 스텝 순서에 제약이 하나 더 붙는다.
func (o *Orchestrator) runSSOProvisioning(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	provisioner := o.ssoProvisioner()
	if provisioner == nil {
		slog.Info("SSO provisioner 가 구성되지 않아 건너뜁니다", "namespace", namespace)
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}

	o.mu.Lock()
	env, orgID := o.secretEnv, o.secretOrgID
	o.mu.Unlock()
	prefix := secretPathPrefix(env, orgID)

	store, err := secrets.NewKubernetesAuthStore(secrets.KubernetesAuthConfig{
		Kubeconfig:     o.kubeconfig,
		Namespace:      namespace,
		Role:           secrets.ControllerRole,
		ServiceAccount: secrets.ControllerServiceAccount,
	})
	if err != nil {
		return fmt.Errorf("OpenBao 컨트롤러 자격 생성 실패: %w", err)
	}

	for _, step := range provisioner.ToolSteps() {
		if !o.isStepEnabled(step) {
			continue
		}
		clientID, ok := provisioner.ClientIDFor(step)
		if !ok {
			continue
		}

		secret, err := store.GetToken(ctx, prefix+ssoClientSecretPath(clientID))
		if err != nil || strings.TrimSpace(secret) == "" {
			return fmt.Errorf("client secret 을 읽지 못했습니다 (%s): %w", clientID, err)
		}

		if err := provisioner.Provision(ctx, port.SSOClientSpec{
			StepName:     step,
			ClientSecret: secret,
		}); err != nil {
			return fmt.Errorf("OIDC 클라이언트 등록 실패 (%s): %w", clientID, err)
		}
		slog.Info("OIDC 클라이언트 등록 완료", "client_id", clientID, "step", step)
	}
	return nil
}

// toolURLScheme 은 접속 도메인 기반 도구 주소의 스킴이다.
//
// 도구는 저마다 자기 기본 주소 설정에서 redirect_uri 를 만든다(Harbor 의
// externalURL, Gitea 의 ROOT_URL). 그 스킴이 Keycloak 에 등록된 redirect 와
// 다르면 로그인이 "Invalid parameter: redirect_uri" 로 막힌다 — Harbor 와
// Gitea 에서 실제로 같은 실패가 났다.
//
// 도구마다 따로 정하면 도구를 추가할 때마다 같은 실수가 반복되므로 여기서
// 한 번만 정한다.
//
// SSO 를 쓰지 않으면 http 그대로 둔다. 이 주소들은 git clone 과 이미지 push 의
// 출처이기도 해서, 스킴을 바꾸면 클라이언트가 게이트웨이 인증서의 CA 를
// 신뢰해야 한다. SSO 를 켜는 환경은 어차피 그 배선을 하지만, 안 쓰는 설치에까지
// 그 부담을 지울 이유가 없다.
func (o *Orchestrator) toolURLScheme() string {
	// 접속 도메인이 TLS 로 열려 있으면 https 다. 이것이 스킴을 정하는 1차 근거다.
	//
	// 예전에는 SSO 켜짐 여부만 봤다. 스킴은 도메인이 TLS 로 서비스되느냐에 달린
	// 것이지 SSO 와는 별개인데 그 둘을 묶어 놓아서, SSO 없이 TLS 만 켠 스택은
	// http 를 받았다.
	//
	// 그 결과는 로그인이 아니라 **업로드**에서 드러난다. Harbor 는 externalURL 의
	// 스킴으로 blob 업로드 주소(Location)를 돌려주는데, 게이트웨이는 http 를
	// https 로 308 리다이렉트한다. 본문이 실린 PATCH 에 308 이 오면 클라이언트는
	// 본문을 다시 보내야 하고 docker 는 거기서 연결을 끊는다 — 2026-08-21 운영에서
	// push 가 모든 layer 를 재시도하다 EOF 로 죽었다. 작은 요청은 다 지나가므로
	// 원인이 멀리 떨어져 보인다.
	o.mu.Lock()
	cfg := o.stackConfig
	issuer := strings.TrimSpace(o.toolOIDCIssuer)
	o.mu.Unlock()

	if cfg != nil && cfg.AccessDomainTLS != nil && cfg.AccessDomainTLS.Enabled {
		return "https"
	}

	// SSO 를 켜면 도구 주소가 Keycloak 에 등록된 redirect 와 스킴이 같아야 한다.
	// 그쪽은 https 이므로 여기서도 https 다.
	if issuer == "" || o.ssoProvisioner() == nil {
		return "http"
	}
	return "https"
}

// gitlabOmniauthProviderBlock 은 이 스택의 접속 도메인과 issuer 로 provider 를 만든다.
func (o *Orchestrator) gitlabOmniauthProviderBlock(clientID string) string {
	o.mu.Lock()
	cfg := o.stackConfig
	issuer := o.toolOIDCIssuer
	o.mu.Unlock()

	accessDomain := ""
	if cfg != nil {
		accessDomain = strings.TrimSpace(cfg.AccessDomain)
	}
	if strings.TrimSpace(issuer) == "" || accessDomain == "" {
		return ""
	}
	baseURL := fmt.Sprintf("%s://gitlab.%s", o.toolURLScheme(), accessDomain)
	return gitlabOmniauthProvider(clientID, issuer, baseURL)
}
