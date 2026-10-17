package helm

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// ESO 는 스택 네임스페이스에 깔면 안 된다 — 컨트롤러가 그 네임스페이스보다 오래 살아야 한다.
//
// ESO 는 자기가 만든 ExternalSecret 에 externalsecret-cleanup finalizer 를 붙이고,
// 그 finalizer 는 ESO 컨트롤러만 뗄 수 있다. 컨트롤러가 스택 네임스페이스 안에 있으면
// `kubectl delete namespace` 가 파드와 ExternalSecret 을 **병렬로** 지우기 때문에
// 컨트롤러가 먼저 죽고 finalizer 가 영구히 남는다 — 네임스페이스는 Terminating 에서
// 끝나지 않는다.
//
// 실측 2026-10-17 (#327·#328 적용 후, 수동 회수 없이):
//
//	NamespaceDeletionDiscoveryFailure   없음   (#327 이 없앴다)
//	NamespaceDeletionContentFailure     없음   (#328 이 없앴다)
//	NamespaceFinalizersRemaining        14개   ← 컨트롤러가 죽어 남았다
//
// 22분을 기다려도 Terminating 이었고, finalizer 를 손으로 비우니 즉시 끝났다.
// cert-manager(cert-manager) · metrics-server(kube-system) 가 같은 이유로 자기
// 네임스페이스를 못박고 있다 — ESO 도 같은 부류다.
func TestESOChartPinsSharedNamespace(t *testing.T) {
	spec, ok := defaultChartSpecForStep("installing_external_secrets")
	require.True(t, ok)

	assert.Equal(t, ESONamespace, spec.Namespace,
		"비어 있으면 스택 네임스페이스로 설치된다 — 그 네임스페이스를 지울 수 없게 된다")
	assert.NotEmpty(t, ESONamespace)
}

// DB 메타데이터가 비어도 핀이 지켜져야 한다. metrics-server 가 그 경로로 뚫렸다(#327).
func TestESONamespaceSurvivesEmptyDBMetadata(t *testing.T) {
	want, ok := defaultChartSpecForStep("installing_external_secrets")
	require.True(t, ok)
	require.NotEmpty(t, want.Namespace)

	repo := repository.NewMemoryHelmStepMetadataRepository()
	require.NoError(t, repo.Create(context.Background(), &domain.HelmStepMetadata{
		StepName:  "installing_external_secrets",
		ChartName: want.ChartName,
		RepoURL:   want.RepoURL,
		Version:   want.Version,
		Namespace: "",
	}))

	o := &Orchestrator{namespace: "nullus-some-stack", helmStepMetadataRepo: repo}
	got, ok := o.chartSpecForStep("installing_external_secrets")
	require.True(t, ok)

	assert.Equal(t, want.Namespace, got.Namespace,
		"DB 가 비면 코드 핀으로 떨어져야 한다")
}

// OpenBao 의 Kubernetes Auth role 은 **스택** 네임스페이스를 묶는다.
//
// ESO 오퍼레이터를 공용 네임스페이스로 옮기면서 이 바인딩도 그쪽으로 바꿨는데
// 틀렸다. OpenBao 가 보는 것은 "로그인에 쓰인 토큰의 SA 가 어느 네임스페이스인가" 다.
// 그 SA 는 스택 네임스페이스에 있어야 한다 — 네임스페이스 범위 SecretStore 의
// serviceAccountRef 는 다른 네임스페이스를 가리킬 수 없기 때문이다(ESO 가 권한
// 상승을 막으려고 ClusterSecretStore 에만 허용한다).
//
// 공용 네임스페이스를 묶었을 때의 실측(2026-10-17): 토큰 요청은 성공하지만 OpenBao 가
//
//	unable to log in with Kubernetes auth: ... namespace not authorized
//
// 로 거부하고 SecretStore 가 InvalidProviderConfig 에 머문다.
func TestOpenBaoESORoleBindsStackNamespace(t *testing.T) {
	script := openBaoBootstrapScript

	i := strings.Index(script, OpenBaoESOServiceAccount)
	require.Greater(t, i, 0, "ESO ServiceAccount 바인딩을 찾지 못했다")
	rest := script[i:]
	j := strings.Index(rest, "bound_service_account_namespaces=")
	require.Greater(t, j, 0)
	line := rest[j : j+strings.Index(rest[j:], "\n")]

	assert.Contains(t, line, "${NAMESPACE}",
		"스택 네임스페이스를 묶어야 한다 — SA 가 거기 있다 (현재: "+line+")")
	assert.NotContains(t, line, ESONamespace,
		"공용 네임스페이스를 묶으면 namespace not authorized 로 거부된다")
}

// SecretStore 가 참조하는 ServiceAccount 를 스택 네임스페이스에 만들어야 한다.
//
// 오퍼레이터를 공용 네임스페이스로 옮기면 차트가 만드는 SA 도 거기 생긴다. 그러면
// 스택 네임스페이스의 SecretStore 가 참조할 SA 가 없어 ESO 가
// "cannot request Kubernetes service account token" 으로 돈다(2026-10-17 실측).
func TestESOServiceAccountIsCreatedInStackNamespace(t *testing.T) {
	src := esoSourceFile(t)

	assert.Contains(t, src, "func (o *Orchestrator) ensureESOServiceAccount(",
		"스택 네임스페이스에 SA 를 만드는 코드가 없다")

	// applySecretStore 보다 먼저 불러야 한다 — SA 가 없으면 SecretStore 가 Ready 가 되지 않는다.
	sa := strings.Index(src, "o.ensureESOServiceAccount(ctx, namespace)")
	store := strings.Index(src, "o.applySecretStore(ctx, namespace)")
	require.Greater(t, sa, 0, "ensureESOServiceAccount 호출을 찾지 못했다")
	require.Greater(t, store, 0)
	assert.Less(t, sa, store, "SA 를 SecretStore 보다 먼저 만들어야 한다")
}

func esoSourceFile(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("external-secrets.go")
	require.NoError(t, err)
	return string(raw)
}

// ESO 오퍼레이터를 깔 네임스페이스는 스택 네임스페이스가 아니다.
//
// 설치 코드가 하나의 namespace 인자를 차트와 SecretStore 에 함께 쓰고 있었다.
// 오퍼레이터만 옮기고 SecretStore 는 스택 네임스페이스에 남겨야 한다 — 따라가면
// 스택의 ExternalSecret 이 참조할 SecretStore 가 없어 시크릿 평면에서 멈춘다.
func TestESOReleaseNamespaceIsNotTheStackNamespace(t *testing.T) {
	spec, ok := defaultChartSpecForStep("installing_external_secrets")
	require.True(t, ok)

	assert.Equal(t, ESONamespace, esoReleaseNamespace(spec),
		"차트 핀을 그대로 써야 한다")

	// DB 가 비어 핀이 사라진 경우에도 스택 네임스페이스로 떨어지지 않는다.
	assert.Equal(t, ESONamespace, esoReleaseNamespace(ChartSpec{}),
		"핀이 없으면 공용 네임스페이스가 기본이어야 한다")

	// 운영자가 DB 에서 다른 네임스페이스를 지정했으면 그것을 존중한다.
	assert.Equal(t, "eso-custom",
		esoReleaseNamespace(ChartSpec{Namespace: " eso-custom "}))
}

// InstallExternalSecrets 의 인자는 **스택** 네임스페이스다 — 오퍼레이터 자리가 아니다.
//
// 차트에 Namespace 를 못박자 오케스트레이터가 그 값을(= 공용 네임스페이스) 이 함수에
// 넘겼고, SecretStore 가 external-secrets 네임스페이스에 만들어졌다. ESO 는 거기서
// 스택의 OpenBao 를 찾지 못해 "unable to validate store" 로 돌고, 설치는
// installing_external_secrets 에서 "SecretStore 가 Ready 상태가 되지 않았습니다" 로
// 실패했다(2026-10-17 실측).
//
// 공용 네임스페이스를 받았으면 스택 네임스페이스로 되돌린다 — SecretStore 는 그곳에
// 있을 수가 없다.
func TestESOStackNamespaceNeverBecomesTheReleaseNamespace(t *testing.T) {
	const stackNS = "nullus-some-stack"
	o := &Orchestrator{namespace: stackNS}

	assert.Equal(t, stackNS, o.esoStackNamespace(ESONamespace),
		"공용 네임스페이스를 받으면 스택 네임스페이스로 되돌려야 한다")
	assert.Equal(t, stackNS, o.esoStackNamespace(""),
		"비면 스택 네임스페이스다")
	assert.Equal(t, stackNS, o.esoStackNamespace(stackNS))
}
