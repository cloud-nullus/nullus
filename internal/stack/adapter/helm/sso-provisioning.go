package helm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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
	slug := o.secretOrgID
	o.mu.Unlock()

	if factory == nil {
		return nil
	}
	accessDomain := ""
	if cfg != nil {
		accessDomain = strings.TrimSpace(cfg.AccessDomain)
	}
	// 스택 슬러그는 접속 도메인 기준이 가장 안정적이다.
	// 도메인이 없으면 조직 ID 로 대체한다.
	stackSlug := strings.TrimSuffix(accessDomain, ".internal")
	if stackSlug == "" {
		stackSlug = slug
	}
	return factory(accessDomain, stackSlug)
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
