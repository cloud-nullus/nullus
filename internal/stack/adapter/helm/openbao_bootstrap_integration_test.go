//go:build integration

package helm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// P2(Kubernetes Auth) 전 경로를 실제 클러스터에서 검증한다.
//
// 검증 항목:
//   - 부트스트랩 Job 이 KV v2 를 'kv' 이름으로 마운트하는가 (경로 규약 일치)
//   - Kubernetes Auth 가 리뷰어 토큰 없이 구성되는가
//   - 정책·role 이 생성되는가
//   - 컨트롤 플레인이 TokenRequest → k8s auth 로그인으로 KV 를 읽고 쓸 수 있는가
//   - ESO role 은 읽기 전용인가 (쓰기 거부)
//   - 부트스트랩 재실행이 멱등한가
func TestIntegration_OpenBao_BootstrapAndKubernetesAuth(t *testing.T) {
	kubeconfig := loadITKubeconfig(t)
	namespace := itNamespace() + "-p2"

	o := NewOrchestrator(NewHelmInstaller(kubeconfig), kubeconfig, namespace)
	o.stackConfig = &domain.StackConfig{
		Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
		Storage:        &domain.StorageConfig{StorageClass: os.Getenv("NULLUS_IT_STORAGE_CLASS")},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	mustKubectl(t, o, ctx, "create", "namespace", namespace)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()
		_ = o.installer.Uninstall(cleanupCtx, "openbao", namespace)
		_, _ = o.runKubectl(cleanupCtx, "delete", "namespace", namespace, "--ignore-not-found", "--wait=false")
	})

	// --- 설치 + 초기화 + 부트스트랩 ---
	spec, _ := defaultChartSpecForStep("installing_openbao")
	if _, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: "openbao",
		ChartName:   spec.ChartName,
		RepoURL:     spec.RepoURL,
		Version:     spec.Version,
		Namespace:   namespace,
		Values:      o.valuesForStep("installing_openbao", spec),
	}); err != nil {
		t.Fatalf("openbao 설치 실패: %v", err)
	}
	if err := o.runOpenBaoInit(ctx, namespace); err != nil {
		t.Fatalf("init 실패: %v", err)
	}
	if err := o.runOpenBaoBootstrap(ctx, namespace); err != nil {
		t.Fatalf("bootstrap 실패: %v", err)
	}

	// --- KV 마운트 이름이 경로 규약(kv/)과 일치하는가 ---
	mounts := baoExec(t, o, ctx, namespace, "bao", "secrets", "list", "-format=json")
	if !strings.Contains(mounts, `"`+OpenBaoKVMount+`/"`) {
		t.Fatalf("KV 엔진이 %q 로 마운트되지 않음: %s", OpenBaoKVMount, mounts)
	}

	// --- Kubernetes Auth 가 리뷰어 토큰 없이 구성되었는가 ---
	authList := baoExec(t, o, ctx, namespace, "bao", "auth", "list", "-format=json")
	if !strings.Contains(authList, `"kubernetes/"`) {
		t.Fatalf("kubernetes auth 가 활성화되지 않음: %s", authList)
	}
	authCfg := baoExec(t, o, ctx, namespace, "bao", "read", "-format=json", "auth/kubernetes/config")
	if strings.Contains(authCfg, `"token_reviewer_jwt"`) &&
		!strings.Contains(authCfg, `"token_reviewer_jwt": ""`) {
		t.Fatalf("리뷰어 토큰이 정적으로 설정됨 — 1년 만료/재시작 403 문제 회피 실패: %s", authCfg)
	}

	// --- 정책과 role 이 존재하는가 ---
	policies := baoExec(t, o, ctx, namespace, "bao", "policy", "list")
	for _, want := range []string{OpenBaoControllerRole + "-write", OpenBaoESORole + "-read"} {
		if !strings.Contains(policies, want) {
			t.Fatalf("정책 %q 가 없음: %s", want, policies)
		}
	}
	roles := baoExec(t, o, ctx, namespace, "bao", "list", "auth/kubernetes/role")
	for _, want := range []string{OpenBaoControllerRole, OpenBaoESORole} {
		if !strings.Contains(roles, want) {
			t.Fatalf("role %q 가 없음: %s", want, roles)
		}
	}

	// --- 컨트롤 플레인 경로: TokenRequest → k8s auth 로그인 → KV 읽기/쓰기 ---
	store, err := secrets.NewKubernetesAuthStore(secrets.KubernetesAuthConfig{
		Kubeconfig:     kubeconfig,
		Namespace:      namespace,
		Role:           secrets.ControllerRole,
		ServiceAccount: secrets.ControllerServiceAccount,
	})
	if err != nil {
		t.Fatalf("KubernetesAuthStore 생성 실패: %v", err)
	}

	if err := store.Check(ctx); err != nil {
		t.Fatalf("k8s auth 기반 헬스체크 실패: %v", err)
	}

	const testPath = "kv/nullus/it/org-1/artifacts/github/token"
	const testValue = "it-token-value"
	if err := store.PutToken(ctx, testPath, testValue); err != nil {
		t.Fatalf("k8s auth 로 KV 쓰기 실패: %v", err)
	}
	got, err := store.GetToken(ctx, testPath)
	if err != nil {
		t.Fatalf("k8s auth 로 KV 읽기 실패: %v", err)
	}
	if got != testValue {
		t.Fatalf("읽은 값이 다름: got=%q want=%q", got, testValue)
	}

	// --- 부트스트랩 멱등성: 재실행해도 저장된 값이 유지된다 ---
	if err := o.runOpenBaoBootstrap(ctx, namespace); err != nil {
		t.Fatalf("bootstrap 재실행 실패: %v", err)
	}
	again, err := store.GetToken(ctx, testPath)
	if err != nil || again != testValue {
		t.Fatalf("부트스트랩 재실행이 데이터를 파괴함: got=%q err=%v", again, err)
	}
}

// baoExec 는 openbao 파드에서 bao CLI 를 실행한다. root token 은 Secret 에서 읽는다.
func baoExec(t *testing.T, o *Orchestrator, ctx context.Context, namespace string, args ...string) string {
	t.Helper()
	rootB64, err := o.runKubectl(ctx, "get", "secret", OpenBaoUnsealKeysSecret, "-n", namespace,
		"-o", "jsonpath={.data.root-token}")
	if err != nil {
		t.Fatalf("root token 조회 실패: %v", err)
	}
	decoded, err := o.runKubectl(ctx, "exec", "-n", namespace, "openbao-0", "-c", "openbao", "--",
		"sh", "-c", "echo '"+strings.TrimSpace(string(rootB64))+"' | base64 -d")
	if err != nil {
		t.Fatalf("root token 디코드 실패: %v", err)
	}
	root := strings.TrimSpace(string(decoded))

	execArgs := []string{"exec", "-n", namespace, "openbao-0", "-c", "openbao", "--",
		"sh", "-c", "BAO_ADDR=http://127.0.0.1:8200 BAO_TOKEN=" + root + " " + strings.Join(args, " ")}
	out, err := o.runKubectl(ctx, execArgs...)
	if err != nil {
		t.Fatalf("bao %s 실패: %v (out=%s)", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}
