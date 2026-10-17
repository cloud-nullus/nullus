package helm

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #330 이전에 깐 스택은 ESO 를 자기 네임스페이스에 갖고 있다 — 그것을 걷어내야 한다.
//
// 공용 ESO 가 서면 CRD·검증 웹훅 소유권은 그쪽으로 넘어간다(adoptExistingESOCRDs,
// 실측 adopted=7). 그런데 옛 릴리스의 컨트롤러·웹훅 Deployment 는 스택 네임스페이스에
// 그대로 남는다. 남겨 두면:
//   - 그 스택의 네임스페이스를 지울 때 컨트롤러가 먼저 죽어 finalizer 가 남는다
//     (#330 이 없애려던 교착이 그 스택에만 그대로 남는다)
//   - 컨트롤러 둘이 같은 ExternalSecret 을 조정한다
//
// 네임스페이스를 가리지 않고 고른다 — 다른 스택의 것이야말로 걷어낼 대상이다.
// 공용 네임스페이스의 릴리스는 제외한다(그것이 새 주인이다).
func TestESOLegacyReleaseNamespacesExcludesSharedNamespace(t *testing.T) {
	out := strings.Join([]string{
		"nullus-gitlab-argocd",
		ESONamespace,
		"nullus-harbor",
		"", // kubectl 출력의 빈 줄
		"nullus-gitlab-argocd",
	}, "\n")

	got := esoLegacyReleaseNamespaces(out)

	assert.Equal(t, []string{"nullus-gitlab-argocd", "nullus-harbor"}, got,
		"공용 네임스페이스는 빼고, 중복은 합치고, 순서는 안정적이어야 한다")
}

func TestESOLegacyReleaseNamespacesEmptyWhenOnlyShared(t *testing.T) {
	assert.Empty(t, esoLegacyReleaseNamespaces(ESONamespace+"\n"))
	assert.Empty(t, esoLegacyReleaseNamespaces(""))
}

// 걷어내는 순서가 중요하다.
//
// 공용 ESO 가 Ready 가 된 **뒤에** 옛 릴리스를 지운다 — 그 사이에도 누군가
// ExternalSecret 의 finalizer 를 처리해야 한다. 그리고 helm uninstall 은 차트가
// 만든 ServiceAccount 까지 지우므로, 지운 뒤에 그 네임스페이스의 SA 를 다시
// 세운다(그 스택의 SecretStore 가 그것을 가리킨다).
func TestESOLegacyMigrationRunsAfterReadinessAndRestoresServiceAccount(t *testing.T) {
	src := esoMigrationSource(t)

	ready := strings.Index(src, "o.waitForExternalSecrets(ctx, releaseNamespace)")
	migrate := strings.Index(src, "o.migrateLegacyESOReleases(ctx)")
	store := strings.Index(src, "o.applySecretStore(ctx, namespace)")

	require.Greater(t, ready, 0, "준비 대기 호출을 찾지 못했다")
	require.Greater(t, migrate, 0, "마이그레이션 호출을 찾지 못했다")
	require.Greater(t, store, 0)

	assert.Less(t, ready, migrate, "공용 ESO 가 Ready 가 된 뒤에 걷어내야 한다")
	assert.Less(t, migrate, store, "SecretStore 적용보다 먼저 끝내야 한다")

	assert.Contains(t, src, "func (o *Orchestrator) migrateLegacyESOReleases(")
	// 지운 네임스페이스마다 SA 를 다시 세운다.
	impl := src[strings.Index(src, "func (o *Orchestrator) migrateLegacyESOReleases("):]
	assert.Contains(t, impl[:min(len(impl), 2500)], "ensureESOServiceAccount",
		"uninstall 이 지운 ServiceAccount 를 다시 세워야 한다")
}

func esoMigrationSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("external-secrets.go")
	require.NoError(t, err)
	return string(raw)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 옛 릴리스는 helm uninstall 로 걷어내지 않는다 — 공용 ESO 의 RBAC 이 함께 사라진다.
//
// uninstall 은 릴리스에 저장된 매니페스트의 자원을 전부 지우는데, 그 매니페스트에는
// ClusterRole·ClusterRoleBinding·웹훅처럼 공용 ESO 가 지금 쓰는 클러스터 범위 자원이
// 함께 들어 있다. 두 가지를 시도했고 둘 다 막지 못했다(2026-10-17 실측):
//
//	① 소유권을 공용 릴리스로 재주석  → ClusterRole NotFound
//	② helm.sh/resource-policy=keep   → ClusterRole NotFound
//
// helm 은 소유권도 resource-policy 도 **저장된 매니페스트**에서 읽기 때문에 살아 있는
// 객체에 주석을 달아도 uninstall 동작은 바뀌지 않는다. 그래서 네임스페이스 범위
// 자원과 릴리스 기록만 직접 지운다.
func TestESOLegacyMigrationDoesNotHelmUninstall(t *testing.T) {
	src := esoMigrationSource(t)
	i := strings.Index(src, "func (o *Orchestrator) migrateLegacyESOReleases(")
	require.Greater(t, i, 0)
	body := src[i:]
	if j := strings.Index(body, "\n}\n"); j > 0 {
		body = body[:j]
	}

	assert.NotContains(t, body, "installer.Uninstall",
		"helm uninstall 은 공용 ESO 가 쓰는 클러스터 범위 자원까지 지운다")
	assert.Contains(t, body, "o.removeLegacyESOWorkloads(",
		"네임스페이스 범위 자원만 지우는 경로를 써야 한다")

	adopt := strings.Index(body, "o.adoptESOClusterScopedFrom(")
	remove := strings.Index(body, "o.removeLegacyESOWorkloads(")
	require.Greater(t, adopt, 0)
	assert.Less(t, adopt, remove, "소유권을 먼저 넘긴 뒤 워크로드를 지운다")
}

// 걷어내기는 클러스터 범위 자원을 건드리지 않는다.
func TestESOLegacyWorkloadRemovalIsNamespaceScopedOnly(t *testing.T) {
	src := esoMigrationSource(t)
	i := strings.Index(src, "func (o *Orchestrator) removeLegacyESOWorkloads(")
	require.Greater(t, i, 0)
	body := src[i:]
	if j := strings.Index(body, "\n}\n"); j > 0 {
		body = body[:j]
	}

	// 지우는 종류에 클러스터 범위가 섞이면 안 된다.
	for _, forbidden := range []string{"clusterrole", "clusterrolebinding", "crd",
		"validatingwebhookconfiguration", "mutatingwebhookconfiguration"} {
		assert.NotContains(t, body, forbidden,
			"클러스터 범위 자원은 공용 ESO 가 쓰고 있다 — 지우면 안 된다")
	}
	assert.Contains(t, body, "-n", "네임스페이스를 한정해야 한다")
	assert.Contains(t, body, "app.kubernetes.io/instance=external-secrets",
		"그 릴리스의 것만 골라야 한다")
}

// 걷어낸 뒤 공용 릴리스를 다시 렌더링해야 한다 — RBAC 내용이 옛 네임스페이스를 가리킨다.
//
// 소유권 주석을 공용 릴리스로 넘겨도 ClusterRoleBinding 의 **내용**은 그대로다.
// 옛 릴리스가 먼저 설치되면서 subject 가 그 네임스페이스로 렌더링됐기 때문이다.
//
// 실측 2026-10-17 (걷어낸 직후):
//
//	subject: ServiceAccount/nullus-legacy-demo/external-secrets
//
// 공용 ESO 의 SA(external-secrets/external-secrets)는 묶이지 않아 reconcile 이
// forbidden 으로 돈다. helm 이 그 자원을 다시 렌더링하도록 공용 릴리스를 한 번 더
// 올려야 한다.
func TestESOLegacyMigrationReinstallsSharedReleaseAfterCleanup(t *testing.T) {
	src := esoMigrationSource(t)
	i := strings.Index(src, "func (o *Orchestrator) migrateLegacyESOReleases(")
	require.Greater(t, i, 0)
	body := src[i:]
	if j := strings.Index(body, "\n\n// "); j > 0 {
		body = body[:j]
	}

	assert.Contains(t, body, "rerenderSharedESO",
		"걷어낸 뒤 공용 릴리스를 다시 렌더링해야 한다")

	remove := strings.Index(body, "o.removeLegacyESOWorkloads(")
	rerender := strings.Index(body, "rerenderSharedESO")
	require.Greater(t, remove, 0)
	assert.Less(t, remove, rerender, "걷어낸 뒤에 다시 렌더링한다")
}
