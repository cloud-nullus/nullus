package helm

import (
	"context"
	"fmt"
	"log/slog"
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
