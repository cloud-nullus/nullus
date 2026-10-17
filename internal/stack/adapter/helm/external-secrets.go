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

	// ESONamespace 는 ESO 를 깔 클러스터 공용 네임스페이스다.
	//
	// 스택 네임스페이스에 깔면 그 네임스페이스를 지울 수 없게 된다. ESO 는 자기가
	// 만든 ExternalSecret 에 externalsecret-cleanup finalizer 를 붙이고 그것을 뗄 수
	// 있는 것은 ESO 컨트롤러뿐인데, `kubectl delete namespace` 는 파드와
	// ExternalSecret 을 병렬로 지운다 — 컨트롤러가 먼저 죽으면 finalizer 가 영구히
	// 남아 네임스페이스가 Terminating 에서 끝나지 않는다(2026-10-17 실측: 22분 뒤에도
	// 그대로였고 finalizer 를 손으로 비우니 즉시 끝났다).
	//
	// 공용 네임스페이스에 두면 컨트롤러가 스택 네임스페이스보다 오래 살아 finalizer 를
	// 스스로 뗀다. cert-manager(cert-manager)·metrics-server(kube-system) 가 같은
	// 이유로 자기 네임스페이스를 못박고 있다. ESO 의 CRD·검증 웹훅도 원래 클러스터에
	// 하나뿐이라 스택마다 깔 이유가 없었다.
	ESONamespace = "external-secrets"

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
			// 웹훅에 닿지 못하면 통과시킨다. 차트 기본값 Fail 은 네임스페이스를
			// 지울 수 없게 만든다 — 검증 웹훅이 DELETE 까지 걸려 있고, ESO 가
			// 스택 네임스페이스에 깔리므로 웹훅 Service 가 가리키는 곳이 바로
			// 지우려는 네임스페이스다. 파드가 먼저 죽으면 ExternalSecret 삭제가
			// 웹훅 호출 실패로 거부되고 네임스페이스는 Terminating 에서 끝나지
			// 않는다(NamespaceDeletionContentFailure).
			//
			// 검증이 약해지는 폭은 좁다. Nullus 의 ExternalSecret·SecretStore 는
			// 제품이 템플릿으로 생성하고(secret-provisioning.go), 설치 경로는
			// 웹훅 rollout 을 기다린 뒤 apply 한다. 웹훅이 내려간 동안 잘못된
			// 스펙이 들어와도 ESO 가 status.conditions 에 실패를 적고 Secret 을
			// 만들지 않으므로 조용히 통과하지 않는다.
			"failurePolicy": "Ignore",
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

// esoClusterScopedKinds 는 ESO 차트가 만드는 클러스터 범위 리소스 종류다.
//
// CRD 만으로는 부족하다. 실측(2026-09-10, 차트 2.7.0)에서 남아 있던 것은
// CRD 24개 · ClusterRole 5개 · ClusterRoleBinding 2개 ·
// ValidatingWebhookConfiguration 2개였다.
//
// 이름으로 찾지 않는다 — 웹훅 설정은 externalsecret-validate /
// secretstore-validate 라 이름에 external-secrets 가 없다. 대신 Helm 이
// 소유권을 판정하는 바로 그 애노테이션(meta.helm.sh/release-name)으로 찾으므로
// 차트가 리소스를 늘려도 따라간다.
var esoClusterScopedKinds = []string{
	"crd",
	"clusterrole",
	"clusterrolebinding",
	"validatingwebhookconfiguration",
	"mutatingwebhookconfiguration",
}

// esoOwnedClusterScoped 는 ESO 릴리스가 소유한 클러스터 범위 리소스를 찾는다.
//
// 소유 네임스페이스는 묻지 않는다 — 다른 네임스페이스가 소유한 것이야말로
// 이번 설치를 막는 대상이기 때문이다.
func (o *Orchestrator) esoOwnedClusterScoped(ctx context.Context, kind, releaseName string) []string {
	out, err := o.runKubectl(ctx, "get", kind,
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.meta\.helm\.sh/release-name}{"\n"}{end}`)
	if err != nil {
		slog.Warn("ESO 클러스터 범위 리소스 조회 실패", "kind", kind, "error", err)
		return nil
	}

	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[1]) == releaseName {
			names = append(names, strings.TrimSpace(parts[0]))
		}
	}
	return names
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

	adopted, skipped := 0, 0
	for _, kind := range esoClusterScopedKinds {
		for _, name := range o.esoOwnedClusterScoped(ctx, kind, releaseName) {
			// 이미 다른 스택이 살아 있는 채로 소유 중이면 탈취하지 않는다.
			// (클러스터 전역이라 소유권을 뺏으면 그 스택 삭제 시 함께 사라진다)
			//
			// 소유권 조회 자체가 실패하면 예전대로 인수한다 — 조회 실패를 이유로
			// 인수를 건너뛰면 다중 네임스페이스 재설치가 다시 막힌다.
			if owner, err := o.clusterScopedOwnerOf(ctx, kind, name); err == nil {
				if strings.TrimSpace(owner.ReleaseName) != "" && owner.ReleaseNamespace != namespace &&
					releaseAliveInNamespace(ctx, o, owner.ReleaseName, owner.ReleaseNamespace) {
					slog.Warn("ESO 리소스를 인수하지 않습니다 — 다른 네임스페이스의 릴리스가 아직 살아 있습니다",
						"kind", kind, "name", name,
						"owner_namespace", owner.ReleaseNamespace, "target_namespace", namespace)
					skipped++
					continue
				}
			}

			if err := o.patchClusterScopedOwnership(ctx, kind, name, releaseName, namespace); err != nil {
				slog.Warn("ESO 리소스 소유권 인수 실패", "kind", kind, "name", name, "error", err)
				continue
			}
			adopted++
		}
	}

	// 건별로 찍으면 30건 넘는 로그가 설치 흐름을 덮는다. 한 줄로 요약하되
	// 0건이면 남기지 않는다 — 첫 설치에는 인수할 것이 없는 게 정상이다.
	if adopted > 0 || skipped > 0 {
		slog.Info("ESO 클러스터 범위 리소스 소유권을 정리했습니다",
			"adopted", adopted, "skipped", skipped, "namespace", namespace)
	}
}

// ensureESOServiceAccount 는 ESO 가 OpenBao 에 로그인할 때 쓸 ServiceAccount 를
// 스택 네임스페이스에 만든다.
//
// ESO 컨트롤러는 클러스터 범위 권한으로 어느 네임스페이스의 SA 든 토큰을 요청할 수
// 있다(ClusterRole external-secrets-controller: serviceaccounts/token create).
// 토큰을 받은 SA 의 네임스페이스가 OpenBao role 의 bound_service_account_namespaces
// 와 맞아야 하므로, SA 는 OpenBao 와 같은 스택 네임스페이스에 둔다.
//
// 공용 네임스페이스의 SA 로 로그인하면 OpenBao 가 "namespace not authorized" 로
// 거부한다(2026-10-17 실측).
func (o *Orchestrator) ensureESOServiceAccount(ctx context.Context, namespace string) error {
	if !looksLikeKubeconfig(o.kubeconfig) {
		return nil
	}
	manifest := "apiVersion: v1\nkind: ServiceAccount\nmetadata:\n" +
		"  name: " + OpenBaoESOServiceAccount + "\n" +
		"  namespace: " + namespace + "\n"
	if err := o.applyManifest(ctx, namespace, manifest); err != nil {
		return fmt.Errorf("ESO ServiceAccount 적용 실패: %w", err)
	}
	return nil
}

// esoReleaseNamespace 는 ESO 오퍼레이터를 깔 네임스페이스다.
//
// 스택 네임스페이스가 아니다. 오퍼레이터가 스택 네임스페이스 안에 있으면 그
// 네임스페이스를 지울 때 컨트롤러가 먼저 죽어 ExternalSecret 의 finalizer 를
// 떼어 줄 주체가 사라진다(ESONamespace 주석 참고). SecretStore 는 스택
// 네임스페이스에 그대로 둔다 — 그쪽은 스택마다 하나씩 필요하다.
func esoReleaseNamespace(spec ChartSpec) string {
	if ns := strings.TrimSpace(spec.Namespace); ns != "" {
		return ns
	}
	return ESONamespace
}

// esoStackNamespace 는 SecretStore·OpenBao 가 있는 스택 네임스페이스다.
//
// 오케스트레이터는 단계마다 "spec.Namespace 가 있으면 그것, 없으면 스택 네임스페이스"
// 로 네임스페이스를 정해 넘긴다. ESO 차트에 공용 네임스페이스를 못박자 그 값이 이
// 함수들로 흘러들어와 SecretStore 가 external-secrets 네임스페이스에 만들어졌다 —
// ESO 는 거기서 스택의 OpenBao 를 찾지 못하고 설치가 멈춘다(2026-10-17 실측).
//
// SecretStore 가 오퍼레이터 네임스페이스에 있을 이유는 없으므로, 그 값을 받으면
// 스택 네임스페이스로 되돌린다.
func (o *Orchestrator) esoStackNamespace(passed string) string {
	ns := strings.TrimSpace(passed)
	if ns == "" || ns == ESONamespace {
		return o.namespace
	}
	return ns
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
	// namespace 는 스택 네임스페이스다 — SecretStore 와 OpenBao 가 거기 있다.
	namespace = o.esoStackNamespace(namespace)

	spec, ok := o.chartSpecForStep("installing_external_secrets")
	if !ok {
		return fmt.Errorf("external-secrets chart spec 을 찾을 수 없습니다")
	}
	spec = o.resolveChartSpecForStep("installing_external_secrets", spec)

	releaseNamespace := esoReleaseNamespace(spec)

	releaseName := o.releaseNameForSpec(spec)
	// 이전 릴리스 삭제로 CRD 가 Terminating 상태면 먼저 정리를 기다린다.
	if err := o.waitForESOCRDsSettled(ctx); err != nil {
		return err
	}
	// 남아 있는 CRD 의 Helm 소유권을 이 릴리스로 인수한 뒤 설치한다.
	o.adoptExistingESOCRDs(ctx, releaseNamespace, releaseName)

	if _, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: releaseName,
		ChartName:   spec.ChartName,
		RepoURL:     spec.RepoURL,
		Version:     spec.Version,
		Namespace:   releaseNamespace,
		Values:      o.valuesForStep("installing_external_secrets", spec),
		Wait:        boolPtr(spec.Wait),
	}); err != nil {
		return fmt.Errorf("ESO 차트 설치 실패: %w", err)
	}

	// 준비 대기는 오퍼레이터가 있는 곳을, SecretStore 는 스택 네임스페이스를 본다.
	if err := o.waitForExternalSecrets(ctx, releaseNamespace); err != nil {
		return err
	}
	// SecretStore 가 가리키는 ServiceAccount 는 스택 네임스페이스에 있어야 한다.
	// 오퍼레이터를 공용 네임스페이스로 옮기면 차트가 만드는 SA 도 거기 생기는데,
	// 네임스페이스 범위 SecretStore 의 serviceAccountRef 는 다른 네임스페이스를
	// 가리킬 수 없다(ESO 가 권한 상승을 막으려고 막아 둔다 — ClusterSecretStore 만
	// 허용한다). 그래서 스택마다 같은 이름의 SA 를 둔다. OpenBao 의 role 도 이
	// 네임스페이스를 묶는다.
	if err := o.ensureESOServiceAccount(ctx, namespace); err != nil {
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
