//go:build integration

package helm

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// P3(ESO 주입 평면) 전 경로를 실제 클러스터에서 검증한다.
//
// 검증 항목:
//   - ESO 공식 차트가 설치되고 준비되는가
//   - SecretStore 가 Ready 가 되는가 (= ESO 가 실제로 OpenBao 에 로그인했다는 증거)
//   - Nullus 생성 → OpenBao → ExternalSecret → K8s Secret 경로가 완결되는가
//   - K8s Secret 의 값이 OpenBao 에 기록한 값과 일치하는가
//   - ESO role 이 읽기 전용인가 (쓰기 시도 거부)
func TestIntegration_ESO_ProvisioningPlane(t *testing.T) {
	kubeconfig := loadITKubeconfig(t)
	namespace := itNamespace() + "-p3"

	o := NewOrchestrator(NewHelmInstaller(kubeconfig), kubeconfig, namespace)
	o.stackConfig = &domain.StackConfig{
		Authentication: &domain.AuthenticationConfig{Provider: "openbao"},
		Storage:        &domain.StorageConfig{StorageClass: os.Getenv("NULLUS_IT_STORAGE_CLASS")},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	mustKubectl(t, o, ctx, "create", "namespace", namespace)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()
		// ESO 커스텀 리소스를 오퍼레이터보다 먼저 지운다 (finalizer 교착 방지).
		for _, kind := range []string{"externalsecrets.external-secrets.io", "secretstores.external-secrets.io"} {
			_, _ = o.runKubectl(cleanupCtx, "delete", kind, "-n", namespace, "--all", "--ignore-not-found", "--timeout=60s")
		}
		_ = o.installer.Uninstall(cleanupCtx, "external-secrets", namespace)
		_ = o.installer.Uninstall(cleanupCtx, "openbao", namespace)
		_, _ = o.runKubectl(cleanupCtx, "delete", "namespace", namespace, "--ignore-not-found", "--wait=false")
	})

	// --- OpenBao 설치 + 초기화 + 부트스트랩 ---
	baoSpec, _ := defaultChartSpecForStep("installing_openbao")
	if _, err := o.installer.Install(ctx, port.HelmInstallRequest{
		ReleaseName: "openbao",
		ChartName:   baoSpec.ChartName,
		RepoURL:     baoSpec.RepoURL,
		Version:     baoSpec.Version,
		Namespace:   namespace,
		Values:      o.valuesForStep("installing_openbao", baoSpec),
	}); err != nil {
		t.Fatalf("openbao 설치 실패: %v", err)
	}
	if err := o.runOpenBaoInit(ctx, namespace); err != nil {
		t.Fatalf("init 실패: %v", err)
	}
	if err := o.runOpenBaoBootstrap(ctx, namespace); err != nil {
		t.Fatalf("bootstrap 실패: %v", err)
	}

	// --- ESO 설치 (오케스트레이터와 동일한 단일 경로) ---
	if err := o.InstallExternalSecrets(ctx, namespace); err != nil {
		t.Fatalf("ESO 설치 실패: %v", err)
	}

	// SecretStore Ready 는 InstallExternalSecrets 안에서 확인된다
	// (= ESO 가 실제로 OpenBao 에 로그인 성공했다는 증거)

	// --- 시크릿 프로비저닝: 생성 → OpenBao → ExternalSecret → K8s Secret ---
	store, err := secrets.NewKubernetesAuthStore(secrets.KubernetesAuthConfig{
		Kubeconfig:     kubeconfig,
		Namespace:      namespace,
		Role:           secrets.ControllerRole,
		ServiceAccount: secrets.ControllerServiceAccount,
	})
	if err != nil {
		t.Fatalf("controller store 생성 실패: %v", err)
	}

	const env, orgID = "it", "org-1"
	if err := o.ProvisionSecrets(ctx, namespace, env, orgID, store); err != nil {
		t.Fatalf("시크릿 프로비저닝 실패: %v", err)
	}

	// --- K8s Secret 값이 OpenBao 값과 일치하는가 ---
	// 차트가 existingSecret 으로 참조하는 키 이름까지 함께 확인한다.
	prefix := secretPathPrefix(env, orgID)
	for _, item := range managedSecrets(namespace) {
		if len(item.TemplateData) > 0 {
			// template 이 있으면 ESO 는 렌더링 결과 키만 남기므로
			// 원본 항목 키는 대상 Secret 에 존재하지 않는다. 아래에서 별도 검증한다.
			t.Logf("템플릿 시크릿: %s (렌더링 결과로 검증)", item.TargetSecret)
			continue
		}
		for _, entry := range item.Entries {
			want, err := store.GetToken(ctx, prefix+entry.PathSuffix)
			if err != nil {
				t.Fatalf("OpenBao 읽기 실패 (%s): %v", entry.PathSuffix, err)
			}
			if strings.TrimSpace(want) == "" {
				t.Fatalf("OpenBao 값이 비어 있음: %s", entry.PathSuffix)
			}

			out, err := o.runKubectl(ctx, "get", "secret", item.TargetSecret, "-n", namespace,
				"-o", "jsonpath={.data."+entry.TargetKey+"}")
			if err != nil {
				t.Fatalf("K8s Secret 조회 실패 (%s/%s): %v", item.TargetSecret, entry.TargetKey, err)
			}
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
			if err != nil {
				t.Fatalf("Secret 값 디코드 실패 (%s): %v", item.TargetSecret, err)
			}
			if string(decoded) != want {
				t.Fatalf("복제된 값이 다름 (%s/%s): k8s=%q openbao=%q",
					item.TargetSecret, entry.TargetKey, string(decoded), want)
			}
		}
		t.Logf("복제 확인: %s (키 %d개, 재시작필요=%v, 소비자=%s)",
			item.TargetSecret, len(item.Entries), item.RestartRequired, item.Consumer)
	}

	// --- ESO template 이 연결 문자열을 렌더링했는가 ---
	connOut, err := o.runKubectl(ctx, "get", "secret", ProvisionedObjectStorageSecret, "-n", namespace,
		"-o", "jsonpath={.data.connection}")
	if err != nil {
		t.Fatalf("object storage connection 조회 실패: %v", err)
	}
	connDecoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(connOut)))
	if err != nil {
		t.Fatalf("connection 디코드 실패: %v", err)
	}
	conn := string(connDecoded)
	if strings.Contains(conn, "{{") {
		t.Fatalf("template 이 렌더링되지 않음: %s", conn)
	}
	if !strings.Contains(conn, "aws_access_key_id: "+MinIORootUser) {
		t.Fatalf("connection 에 access key 가 없음: %s", conn)
	}
	t.Logf("object storage connection 렌더링 확인 (하드코딩 문자열 대체)")

	// --- ESO role 은 읽기 전용이어야 한다 ---
	esoStore, err := secrets.NewKubernetesAuthStore(secrets.KubernetesAuthConfig{
		Kubeconfig:     kubeconfig,
		Namespace:      namespace,
		Role:           OpenBaoESORole,
		ServiceAccount: OpenBaoESOServiceAccount,
	})
	if err != nil {
		t.Fatalf("ESO store 생성 실패: %v", err)
	}
	if err := esoStore.PutToken(ctx, prefix+"artifacts/should-not-write", "nope"); err == nil {
		t.Fatal("ESO role 로 쓰기가 성공했습니다 — 최소권한 정책 위반")
	}
}
