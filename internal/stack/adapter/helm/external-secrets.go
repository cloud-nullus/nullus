package helm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

const (
	// ESOChartName / ESOChartRepo / ESOChartVersion 은 공식 External Secrets
	// Operator 차트다. 컨트롤러·웹훅·cert-controller 를 코드 수정 없이 그대로
	// 사용하며, Nullus 가 작성하는 것은 SecretStore/ExternalSecret 뿐이다.
	ESOChartName    = "external-secrets"
	ESOChartRepo    = "https://charts.external-secrets.io"
	ESOChartVersion = "2.7.0"

	// ESOSecretStoreName 은 스택 네임스페이스에 만드는 SecretStore 이름이다.
	// 스택이 단일 네임스페이스에 설치되므로 ClusterSecretStore 는 쓰지 않는다.
	ESOSecretStoreName = "nullus-openbao"

	esoReadyTimeout = 5 * time.Minute
)

// externalSecretsValues 는 ESO 차트 values 를 만든다.
//
// ESO 가 OpenBao 에 로그인할 때 쓸 ServiceAccount 이름은 부트스트랩 Job 이
// 만든 role 의 bound_service_account_names 와 일치해야 한다.
func externalSecretsValues() map[string]any {
	return map[string]any{
		"installCRDs": true,
		"serviceAccount": map[string]any{
			"create": true,
			"name":   OpenBaoESOServiceAccount,
		},
		"resources": map[string]any{
			"requests": map[string]any{"cpu": "100m", "memory": "128Mi"},
			"limits":   map[string]any{"cpu": "200m", "memory": "256Mi"},
		},
		"webhook": map[string]any{
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "50m", "memory": "64Mi"},
				"limits":   map[string]any{"cpu": "100m", "memory": "128Mi"},
			},
		},
		"certController": map[string]any{
			"resources": map[string]any{
				"requests": map[string]any{"cpu": "50m", "memory": "64Mi"},
				"limits":   map[string]any{"cpu": "100m", "memory": "128Mi"},
			},
		},
	}
}

// secretStoreManifest 는 OpenBao 를 가리키는 SecretStore 를 만든다.
//
// Kubernetes Auth 로 인증하므로 정적 토큰이 어디에도 등장하지 않는다.
// role 은 읽기 전용 정책(nullus-eso-read)에 바인딩되어 있다.
func secretStoreManifest(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		namespace = "nullus"
	}
	return fmt.Sprintf(`apiVersion: external-secrets.io/v1
kind: SecretStore
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  provider:
    vault:
      server: "http://openbao.%[2]s.svc.cluster.local:8200"
      path: "%[3]s"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "%[4]s"
          serviceAccountRef:
            name: "%[5]s"
`, ESOSecretStoreName, namespace, OpenBaoKVMount, OpenBaoESORole, OpenBaoESOServiceAccount)
}

// esoCRDNames 는 ESO 가 설치하는 CRD 중 존재 여부 판별에 쓰는 대표 항목이다.
var esoCRDNames = []string{
	"externalsecrets.external-secrets.io",
	"secretstores.external-secrets.io",
}

// adoptExistingESOCRDs 는 이미 존재하는 ESO CRD 의 Helm 소유권을 현재 릴리스로 넘긴다.
//
// ESO CRD 는 클러스터 범위이고 Helm 은 릴리스 삭제 시 CRD 를 지우지 않는다.
// 그래서 다른 네임스페이스에 두 번째 스택을 설치하면 Helm 이
// "invalid ownership metadata" 로 실패한다. 멀티 스택 제품에서는 반드시
// 발생하는 상황이다.
//
// 설치를 건너뛰는(installCRDs=false) 방식은 쓰지 않는다. CRD 가 일부만 남은
// 상태에서 전체를 건너뛰면 없는 CRD 가 끝내 만들어지지 않아 SecretStore 생성이
// NotFound 로 실패한다. 대신 존재하는 것만 소유권을 인수하고, 없는 것은 Helm 이
// 정상적으로 만들게 둔다.
func (o *Orchestrator) adoptExistingESOCRDs(ctx context.Context, namespace, releaseName string) {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return
	}
	for _, crd := range esoCRDNames {
		if _, err := o.runKubectl(ctx, "get", "crd", crd); err != nil {
			continue // 없으면 Helm 이 새로 만든다
		}
		patch := fmt.Sprintf(
			`{"metadata":{"labels":{"app.kubernetes.io/managed-by":"Helm"},`+
				`"annotations":{"meta.helm.sh/release-name":%q,"meta.helm.sh/release-namespace":%q}}}`,
			releaseName, namespace)
		if _, err := o.runKubectl(ctx, "patch", "crd", crd, "--type=merge", "-p", patch); err != nil {
			slog.Warn("ESO CRD 소유권 인수 실패", "crd", crd, "error", err)
			continue
		}
		slog.Info("ESO CRD 소유권을 현재 릴리스로 인수했습니다", "crd", crd, "namespace", namespace)
	}
}

// InstallExternalSecrets 는 ESO 설치 전 과정을 하나의 경로로 묶는다.
//
// 차트 설치 → 준비 대기 → SecretStore 적용까지 여기서 처리하므로,
// 오케스트레이터와 통합 테스트가 같은 코드를 통과한다. CRD 소유권 보정도
// 이 안에서 이뤄져 우회 경로가 생기지 않는다.
func (o *Orchestrator) InstallExternalSecrets(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}

	spec, ok := o.chartSpecForStep("installing_external_secrets")
	if !ok {
		return fmt.Errorf("external-secrets chart spec 을 찾을 수 없습니다")
	}
	spec = o.resolveChartSpecForStep("installing_external_secrets", spec)

	releaseName := o.releaseNameForSpec(spec)
	// 이전 릴리스 삭제로 CRD 가 Terminating 상태면 먼저 정리를 기다린다.
	if err := o.waitForESOCRDsSettled(ctx); err != nil {
		return err
	}
	// 남아 있는 CRD 의 Helm 소유권을 이 릴리스로 인수한 뒤 설치한다.
	o.adoptExistingESOCRDs(ctx, namespace, releaseName)

	if _, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: releaseName,
		ChartName:   spec.ChartName,
		RepoURL:     spec.RepoURL,
		Version:     spec.Version,
		Namespace:   namespace,
		Values:      o.valuesForStep("installing_external_secrets", spec),
		Wait:        boolPtr(spec.Wait),
	}); err != nil {
		return fmt.Errorf("ESO 차트 설치 실패: %w", err)
	}

	if err := o.waitForExternalSecrets(ctx, namespace); err != nil {
		return err
	}
	if err := o.applySecretStore(ctx, namespace); err != nil {
		return err
	}

	// SecretStore Ready 는 ESO 가 Kubernetes Auth 로 로그인에 성공했다는 뜻이다.
	// 인증 경로가 증명된 이 시점에 root token 을 폐기한다.
	if err := o.RevokeOpenBaoRootToken(ctx, namespace); err != nil {
		// 폐기 실패가 설치를 막을 이유는 없다. 경고만 남기고 계속한다.
		slog.Warn("root token 폐기 실패 — 수동 폐기가 필요합니다", "namespace", namespace, "error", err)
	}
	return nil
}

// waitForExternalSecrets 는 ESO 구성요소가 준비될 때까지 기다린다.
func (o *Orchestrator) waitForExternalSecrets(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}

	waitCtx, cancel := context.WithTimeout(ctx, esoReadyTimeout)
	defer cancel()

	// 웹훅이 준비되기 전에 SecretStore 를 apply 하면 실패하므로 rollout 을 기다린다.
	//
	// cert-controller 는 웹훅 인증서를 주입하는 역할이라 기동이 느릴 수 있다.
	// 단일 노드 클러스터에서 여러 스택이 함께 뜨면 더 지연되므로, 필수는
	// 컨트롤러와 웹훅으로 한정하고 cert-controller 는 경고만 남긴다.
	// SecretStore Ready 확인이 뒤따르므로 실제 준비 여부는 거기서 판별된다.
	required := []string{"external-secrets", "external-secrets-webhook"}
	optional := []string{"external-secrets-cert-controller"}

	for _, deploy := range required {
		if _, err := o.runKubectl(waitCtx, "rollout", "status",
			"deployment/"+deploy, "-n", namespace, "--timeout=240s"); err != nil {
			return fmt.Errorf("ESO %s 준비 대기 실패: %w", deploy, err)
		}
	}
	for _, deploy := range optional {
		if _, err := o.runKubectl(waitCtx, "rollout", "status",
			"deployment/"+deploy, "-n", namespace, "--timeout=120s"); err != nil {
			slog.Warn("ESO 보조 구성요소 준비 지연 — SecretStore Ready 검사로 판별합니다",
				"deployment", deploy, "namespace", namespace, "error", err)
		}
	}
	return nil
}

// applySecretStore 는 SecretStore 를 적용하고 Valid 상태가 될 때까지 기다린다.
//
// SecretStore 가 Valid 라는 것은 ESO 가 실제로 OpenBao 에 로그인했다는 뜻이므로,
// Kubernetes Auth 구성 전체를 검증하는 지점이기도 하다.
func (o *Orchestrator) applySecretStore(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = o.namespace
	}

	if err := o.applyManifest(ctx, namespace, secretStoreManifest(namespace)); err != nil {
		return fmt.Errorf("SecretStore 적용 실패: %w", err)
	}

	const (
		maxAttempts = 30
		retryDelay  = 5 * time.Second
	)
	var last string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, err := o.runKubectl(ctx, "get", "secretstore", ESOSecretStoreName, "-n", namespace,
			"-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`)
		if err == nil {
			last = strings.TrimSpace(string(out))
			if last == "True" {
				return nil
			}
		}
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}
	return fmt.Errorf("SecretStore 가 Ready 상태가 되지 않았습니다 (마지막 상태: %q)", last)
}

// waitForESOCRDsSettled 는 삭제 진행 중인 ESO CRD 가 정리될 때까지 기다린다.
//
// ESO 차트는 CRD 를 일반 템플릿으로 렌더링하므로 helm uninstall 시 함께
// 삭제된다. 남아 있는 CR 의 finalizer 때문에 CRD 가 Terminating 상태에 한동안
// 머무는데, 그 사이에 재설치하면 CR 생성이
// "create not allowed while custom resource definition is terminating" 로 실패한다.
// 스택을 지우고 다시 설치하는 흐름에서 그대로 재현되므로 여기서 흡수한다.
func (o *Orchestrator) waitForESOCRDsSettled(ctx context.Context) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}

	const (
		maxAttempts = 60
		retryDelay  = 5 * time.Second
	)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		terminating := ""
		for _, crd := range esoCRDNames {
			out, err := o.runKubectl(ctx, "get", "crd", crd,
				"-o", "jsonpath={.metadata.deletionTimestamp}")
			if err != nil {
				continue // 없으면 문제없다
			}
			if strings.TrimSpace(string(out)) != "" {
				terminating = crd
				break
			}
		}
		if terminating == "" {
			return nil
		}
		if attempt == maxAttempts {
			return fmt.Errorf("ESO CRD %q 가 삭제 중 상태에서 벗어나지 않았습니다", terminating)
		}
		slog.Info("ESO CRD 삭제 완료 대기", "crd", terminating)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}
	return nil
}
