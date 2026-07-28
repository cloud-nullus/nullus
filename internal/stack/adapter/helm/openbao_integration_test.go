//go:build integration

package helm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 실제 클러스터를 대상으로 P1(OpenBao 운영 모드) 전 경로를 검증한다.
//
//	NULLUS_IT_KUBECONFIG=<path> go test -tags=integration ./internal/stack/adapter/helm/ -run TestIntegration_OpenBao -v
//
// 검증 항목:
//   - 공식 차트로 StatefulSet + PVC 가 생성되는가 (dev 모드가 아님)
//   - init Job 이 금고를 초기화하고 unseal key Secret 을 만드는가
//   - 사이드카가 자동으로 봉인을 해제하는가
//   - init Job 을 다시 돌려도 재초기화하지 않는가 (멱등성)
//   - 파드를 삭제해도 데이터가 유지되고 자동으로 다시 열리는가 (영속성)
func TestIntegration_OpenBao_InstallInitUnseal(t *testing.T) {
	kubeconfig := loadITKubeconfig(t)
	namespace := itNamespace() + "-p1"

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

	// --- 1) 공식 차트 설치 ---
	spec, ok := defaultChartSpecForStep("installing_openbao")
	if !ok {
		t.Fatal("installing_openbao chart spec 을 찾을 수 없음")
	}
	if spec.ChartName == "" || spec.RepoURL == "" {
		t.Fatalf("공식 차트가 설정되지 않음: %+v", spec)
	}

	values := o.valuesForStep("installing_openbao", spec)
	if _, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: "openbao",
		ChartName:   spec.ChartName,
		RepoURL:     spec.RepoURL,
		Version:     spec.Version,
		Namespace:   namespace,
		Values:      values,
	}); err != nil {
		t.Fatalf("openbao 차트 설치 실패: %v", err)
	}

	// dev 모드가 아니라 영속 스토리지를 쓰는지 확인한다.
	waitFor(t, ctx, 3*time.Minute, "PVC 생성", func() bool {
		out, err := o.runKubectl(ctx, "get", "pvc", "data-openbao-0", "-n", namespace, "-o", "jsonpath={.status.phase}")
		return err == nil && strings.Contains(string(out), "Bound")
	})

	// --- 2) init Job 실행 (멱등) ---
	if err := o.runOpenBaoInit(ctx, namespace); err != nil {
		t.Fatalf("openbao init 실패: %v", err)
	}

	// --- 3) unseal key Secret 이 만들어졌는가 ---
	out, err := o.runKubectl(ctx, "get", "secret", OpenBaoUnsealKeysSecret, "-n", namespace, "-o", "jsonpath={.data.key1}")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("unseal key Secret 이 없거나 비어 있음: err=%v out=%q", err, string(out))
	}
	rootOut, err := o.runKubectl(ctx, "get", "secret", OpenBaoUnsealKeysSecret, "-n", namespace, "-o", "jsonpath={.data.root-token}")
	if err != nil || strings.TrimSpace(string(rootOut)) == "" {
		t.Fatalf("root token 이 저장되지 않음: err=%v", err)
	}

	// --- 4) 봉인이 해제되었는가 (preflight gate 통과 조건) ---
	if err := o.waitForOpenBaoUnsealed(ctx, namespace); err != nil {
		t.Fatalf("봉인 해제 실패: %v", err)
	}

	// --- 5) 멱등성: init 을 다시 돌려도 재초기화하지 않는다 ---
	firstKey := strings.TrimSpace(string(out))
	if err := o.runOpenBaoInit(ctx, namespace); err != nil {
		t.Fatalf("init 재실행 실패: %v", err)
	}
	secondOut, err := o.runKubectl(ctx, "get", "secret", OpenBaoUnsealKeysSecret, "-n", namespace, "-o", "jsonpath={.data.key1}")
	if err != nil {
		t.Fatalf("재실행 후 Secret 조회 실패: %v", err)
	}
	if strings.TrimSpace(string(secondOut)) != firstKey {
		t.Fatalf("init 재실행이 금고를 재초기화했습니다 — 멱등성 위반")
	}

	// --- 6) 영속성 + 자동 unseal: 파드를 지워도 데이터가 살아있고 다시 열린다 ---
	mustKubectl(t, o, ctx, "delete", "pod", "openbao-0", "-n", namespace)
	waitFor(t, ctx, 3*time.Minute, "파드 재기동", func() bool {
		out, err := o.runKubectl(ctx, "get", "pod", "openbao-0", "-n", namespace, "-o", "jsonpath={.status.phase}")
		return err == nil && strings.Contains(string(out), "Running")
	})
	if err := o.waitForOpenBaoUnsealed(ctx, namespace); err != nil {
		t.Fatalf("재기동 후 자동 unseal 실패: %v", err)
	}

	initStatus, err := o.runKubectl(ctx, "exec", "-n", namespace, "openbao-0", "-c", "openbao",
		"--", "wget", "-q", "-O", "-", "http://127.0.0.1:8200/v1/sys/init")
	if err != nil {
		t.Fatalf("init 상태 조회 실패: %v", err)
	}
	if !strings.Contains(string(initStatus), `"initialized":true`) {
		t.Fatalf("재기동 후 초기화 상태가 유지되지 않음: %s", string(initStatus))
	}
}

func loadITKubeconfig(t *testing.T) []byte {
	t.Helper()
	path := strings.TrimSpace(os.Getenv("NULLUS_IT_KUBECONFIG"))
	if path == "" {
		t.Skip("NULLUS_IT_KUBECONFIG 미설정 — 통합 테스트 건너뜀")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- 테스트 전용 경로
	if err != nil {
		t.Fatalf("kubeconfig 읽기 실패: %v", err)
	}
	return data
}

func itNamespace() string {
	if ns := strings.TrimSpace(os.Getenv("NULLUS_IT_NAMESPACE")); ns != "" {
		return ns
	}
	return "nullus-it"
}

func mustKubectl(t *testing.T, o *Orchestrator, ctx context.Context, args ...string) {
	t.Helper()
	if _, err := o.runKubectl(ctx, args...); err != nil {
		t.Fatalf("kubectl %s 실패: %v", strings.Join(args, " "), err)
	}
}

func waitFor(t *testing.T, ctx context.Context, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%s 대기 중 컨텍스트 종료", what)
		case <-time.After(5 * time.Second):
		}
	}
	t.Fatalf("%s 대기 시간 초과 (%s)", what, timeout)
}
