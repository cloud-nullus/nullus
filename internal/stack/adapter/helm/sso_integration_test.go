//go:build integration

package helm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	keycloakadapter "github.com/cloud-nullus/draft/internal/auth/adapter/keycloak"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// P3-SSO 전 경로를 실제 클러스터에서 검증한다.
//
// 검증 항목:
//   - client secret 이 시크릿 지도에 편입되어 OpenBao → K8s Secret 으로 복제되는가
//   - client ID 가 스택 단위로 네임스페이싱되는가 (공용 realm 충돌 방지)
//   - provisioning_sso 가 OpenBao 값을 읽어 IdP 에 push 하는가
//   - argocd-secret 이 admin 비밀번호와 OIDC secret 을 함께 담는가
func TestIntegration_SSO_ProvisioningPlane(t *testing.T) {
	kubeconfig := loadITKubeconfig(t)
	namespace := itNamespace() + "-sso"

	// IdP 스텁 — 실제 Keycloak 대신 등록 요청만 관찰한다.
	var registered []map[string]any
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/protocol/openid-connect/token"):
			_, _ = w.Write([]byte(`{"access_token":"t","expires_in":300}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/clients"):
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clients"):
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			registered = append(registered, payload)
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer idp.Close()

	o := NewOrchestrator(NewHelmInstaller(kubeconfig), kubeconfig, namespace)
	o.stackConfig = &domain.StackConfig{
		AccessDomain:   "sso-test.internal",
		Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
		Storage:        &domain.StorageConfig{StorageClass: os.Getenv("NULLUS_IT_STORAGE_CLASS")},
		Monitoring: domain.MonitoringConfig{
			Visualization: domain.ToolSelection{Name: "grafana", Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CDTool: domain.ToolSelection{Name: "argocd", Enabled: true},
		},
	}
	o.SetSecretScope("it", "org-sso")
	o.SetSSOProvisionerFactory(newTestSSOFactory(idp.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	mustKubectl(t, o, ctx, "create", "namespace", namespace)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()
		for _, kind := range []string{"externalsecrets.external-secrets.io", "secretstores.external-secrets.io"} {
			_, _ = o.runKubectl(cleanupCtx, "delete", kind, "-n", namespace, "--all", "--ignore-not-found", "--timeout=60s")
		}
		_ = o.installer.Uninstall(cleanupCtx, "external-secrets", namespace)
		_ = o.installer.Uninstall(cleanupCtx, "openbao", namespace)
		_, _ = o.runKubectl(cleanupCtx, "delete", "namespace", namespace, "--ignore-not-found", "--wait=false")
	})

	// --- 시크릿 평면 준비 ---
	baoSpec, _ := defaultChartSpecForStep("installing_openbao")
	if _, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: "openbao", ChartName: baoSpec.ChartName, RepoURL: baoSpec.RepoURL,
		Version: baoSpec.Version, Namespace: namespace,
		Values: o.valuesForStep("installing_openbao", baoSpec),
	}); err != nil {
		t.Fatalf("openbao 설치 실패: %v", err)
	}
	if err := o.runOpenBaoInit(ctx, namespace); err != nil {
		t.Fatalf("init 실패: %v", err)
	}
	if err := o.runOpenBaoBootstrap(ctx, namespace); err != nil {
		t.Fatalf("bootstrap 실패: %v", err)
	}
	if err := o.InstallExternalSecrets(ctx, namespace); err != nil {
		t.Fatalf("ESO 설치 실패: %v", err)
	}

	// --- client ID 네임스페이싱 확인 ---
	provisioner := o.ssoProvisioner()
	if provisioner == nil {
		t.Fatal("SSO provisioner 가 구성되지 않았습니다")
	}
	grafanaID, ok := provisioner.ClientIDFor("installing_grafana")
	if !ok || !strings.HasPrefix(grafanaID, "sso-test-") {
		t.Fatalf("client ID 가 스택 단위로 네임스페이싱되지 않음: %q", grafanaID)
	}

	// --- 시크릿 프로비저닝 (SSO client secret 포함) ---
	store, err := secrets.NewKubernetesAuthStore(secrets.KubernetesAuthConfig{
		Kubeconfig: kubeconfig, Namespace: namespace,
		Role: secrets.ControllerRole, ServiceAccount: secrets.ControllerServiceAccount,
	})
	if err != nil {
		t.Fatalf("controller store 생성 실패: %v", err)
	}
	if err := o.ProvisionSecrets(ctx, namespace, "it", "org-sso", store); err != nil {
		t.Fatalf("시크릿 프로비저닝 실패: %v", err)
	}

	// --- client secret 이 K8s Secret 으로 복제되었는가 ---
	grafanaSecret := SSOSecretName(grafanaID)
	out, err := o.runKubectl(ctx, "get", "secret", grafanaSecret, "-n", namespace,
		"-o", "jsonpath={.data.client-secret}")
	if err != nil {
		t.Fatalf("client secret 조회 실패 (%s): %v", grafanaSecret, err)
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if strings.TrimSpace(string(decoded)) == "" {
		t.Fatalf("client secret 이 비어 있음: %s", grafanaSecret)
	}
	t.Logf("client secret 복제 확인: %s → %s", grafanaID, grafanaSecret)

	// --- argocd-secret 이 두 값을 함께 담는가 ---
	for _, key := range []string{"oidc.keycloak.clientSecret", "clearPassword"} {
		out, err := o.runKubectl(ctx, "get", "secret", ArgoCDSecretName, "-n", namespace,
			"-o", "jsonpath={.data."+strings.ReplaceAll(key, ".", `\.`)+"}")
		if err != nil || strings.TrimSpace(string(out)) == "" {
			t.Fatalf("argocd-secret 에 %s 가 없음: err=%v", key, err)
		}
	}
	t.Logf("argocd-secret 단독 소유 확인 (admin 비밀번호 + OIDC secret 통합)")

	// --- provisioning_sso 가 IdP 에 push 하는가 ---
	if err := o.runSSOProvisioning(ctx, namespace); err != nil {
		t.Fatalf("SSO 프로비저닝 실패: %v", err)
	}
	if len(registered) == 0 {
		t.Fatal("IdP 에 클라이언트가 등록되지 않았습니다")
	}
	for _, payload := range registered {
		if payload["secret"] == nil || payload["secret"] == "" {
			t.Fatalf("client secret 이 push 되지 않음: %v", payload)
		}
		if payload["publicClient"] != false {
			t.Fatalf("confidential client 여야 합니다: %v", payload)
		}
	}
	t.Logf("IdP 등록 확인: %d개 클라이언트 (secret push 방식)", len(registered))
}

// newTestSSOFactory 는 스텁 IdP 를 향하는 provisioner 팩토리를 만든다.
func newTestSSOFactory(idpURL string) port.SSOProvisionerFactory {
	return keycloakadapter.NewStackSSOFactory(
		keycloakadapter.NewKeycloakClient(idpURL, "nullus", "admin", "admin"),
	)
}
