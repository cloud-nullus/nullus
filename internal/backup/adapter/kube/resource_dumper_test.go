package kube

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 아래 테스트들은 전부 리허설에서 실제로 깨진 것들을 고정한다.
// 고쳐 놓고 지키지 않으면 다시 돌아온다.

// ── 결함 2: 없는 CRD 에서 덤프 전체가 실패했다 ──────────────────────────
//
// `--ignore-not-found` 는 없는 **객체**를 다루지 없는 **리소스 타입**은
// 다루지 못한다. Gateway API 가 없는 클러스터에서 통째로 죽었다.

const apiResourcesWithoutGateway = `
configmaps
secrets
serviceaccounts
services
deployments.apps
statefulsets.apps
daemonsets.apps
roles.rbac.authorization.k8s.io
rolebindings.rbac.authorization.k8s.io
ingresses.networking.k8s.io
`

const apiResourcesWithGateway = apiResourcesWithoutGateway + `
gateways.gateway.networking.k8s.io
httproutes.gateway.networking.k8s.io
`

func TestFilterAvailable_GatewayAPI_가_없으면_뺀다(t *testing.T) {
	got := filterAvailable(dumpKinds, apiResourcesWithoutGateway)

	assert.NotContains(t, got, "gateways.gateway.networking.k8s.io")
	assert.NotContains(t, got, "httproutes.gateway.networking.k8s.io")
	// 있는 것은 그대로 남아야 한다 — 지나치게 걸러내면 조용히 빠진다.
	assert.Contains(t, got, "deployments")
	assert.Contains(t, got, "secrets")
	assert.Contains(t, got, "ingresses")
}

func TestFilterAvailable_GatewayAPI_가_있으면_포함한다(t *testing.T) {
	got := filterAvailable(dumpKinds, apiResourcesWithGateway)
	assert.Contains(t, got, "gateways.gateway.networking.k8s.io")
	assert.Contains(t, got, "httproutes.gateway.networking.k8s.io")
}

func TestFilterAvailable_그룹이_붙은_이름을_짧은_이름과_맞춘다(t *testing.T) {
	// kubectl 은 `deployments.apps` 로 주지만 dumpKinds 는 `deployments` 다.
	got := filterAvailable([]string{"deployments"}, "deployments.apps\n")
	assert.Equal(t, []string{"deployments"}, got)
}

func TestFilterAvailable_아무것도_없으면_빈_목록(t *testing.T) {
	assert.Empty(t, filterAvailable(dumpKinds, ""))
}

// ── 결함 5: PVC 를 볼륨 경로와 리소스 경로가 이중 소유했다 ──────────────
//
// 복구 시 EnsurePVC 가 만든 PVC 를 apply 가 다시 건드려 `spec.volumeName`
// immutable 오류로 죽었다. 소유자는 하나여야 한다.

func TestDumpKinds_PVC_를_담지_않는다(t *testing.T) {
	assert.NotContains(t, dumpKinds, "persistentvolumeclaims",
		"PVC 는 볼륨 경로가 소유한다 — 리소스 덤프에 들어가면 복구가 충돌한다")
}

func TestSkipResource_PVC_는_다른_경로로_들어와도_막는다(t *testing.T) {
	assert.True(t, skipResource(map[string]any{
		"kind":     "PersistentVolumeClaim",
		"metadata": map[string]any{"name": "data-gitlab"},
	}))
}

// ── 결함 4: 덤프에 서버 관리 필드가 그대로 담겼다 ───────────────────────
//
// 복구 apply 가 "the object has been modified" 로 죽었다. 되살릴 것은
// 의도(spec)이지 그때의 서버 상태가 아니다.

func rawList(items ...map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "List", "items": items})
	return b
}

func sanitizedItems(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	out, err := sanitize(raw)
	require.NoError(t, err)
	var parsed struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(out, &parsed))
	return parsed.Items
}

func TestSanitize_서버_관리_필드를_걷어낸다(t *testing.T) {
	items := sanitizedItems(t, rawList(map[string]any{
		"kind": "Deployment",
		"metadata": map[string]any{
			"name":              "gitlab",
			"namespace":         "nullus",
			"resourceVersion":   "12345",
			"uid":               "abc-def",
			"creationTimestamp": "2026-09-01T00:00:00Z",
			"generation":        float64(7),
			"managedFields":     []any{map[string]any{"manager": "kubectl"}},
			"selfLink":          "/apis/apps/v1/...",
			"labels":            map[string]any{"app": "gitlab"},
		},
		"spec":   map[string]any{"replicas": float64(3)},
		"status": map[string]any{"readyReplicas": float64(3)},
	}))

	require.Len(t, items, 1)
	meta := items[0]["metadata"].(map[string]any)

	for _, f := range []string{"resourceVersion", "uid", "creationTimestamp", "generation", "managedFields", "selfLink"} {
		assert.NotContains(t, meta, f, "%s 가 남으면 apply 가 깨진다", f)
	}
	assert.NotContains(t, items[0], "status", "status 는 서버가 만든다")

	// 되살려야 하는 것은 남아야 한다.
	assert.Equal(t, "gitlab", meta["name"])
	assert.Equal(t, map[string]any{"app": "gitlab"}, meta["labels"])
	assert.Equal(t, float64(3), items[0]["spec"].(map[string]any)["replicas"])
}

func TestSanitize_Service_의_clusterIP_를_뺀다(t *testing.T) {
	// 클러스터가 배정하는 값이다. 그대로 되돌리면 이미 쓰이는 주소와
	// 충돌하거나 다른 클러스터에서는 범위 밖이다.
	items := sanitizedItems(t, rawList(map[string]any{
		"kind":     "Service",
		"metadata": map[string]any{"name": "gitlab"},
		"spec": map[string]any{
			"clusterIP":  "10.96.0.10",
			"clusterIPs": []any{"10.96.0.10"},
			"ports":      []any{map[string]any{"port": float64(80)}},
		},
	}))
	require.Len(t, items, 1)
	spec := items[0]["spec"].(map[string]any)
	assert.NotContains(t, spec, "clusterIP")
	assert.NotContains(t, spec, "clusterIPs")
	assert.Contains(t, spec, "ports", "포트 정의는 남아야 한다")
}

func TestSanitize_컨트롤러가_소유한_것은_뺀다(t *testing.T) {
	// 소유자가 다시 만든다. 먼저 되돌리면 소유자와 다투게 된다.
	items := sanitizedItems(t, rawList(map[string]any{
		"kind": "Secret",
		"metadata": map[string]any{
			"name":            "sh.helm.release.v1.nullus.v1",
			"ownerReferences": []any{map[string]any{"kind": "Deployment", "name": "x"}},
		},
	}))
	assert.Empty(t, items)
}

func TestSanitize_자동_생성_객체를_뺀다(t *testing.T) {
	items := sanitizedItems(t, rawList(
		map[string]any{"kind": "ServiceAccount", "metadata": map[string]any{"name": "default"}},
		map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "kube-root-ca.crt"}},
		map[string]any{
			"kind": "Secret", "type": "kubernetes.io/service-account-token",
			"metadata": map[string]any{"name": "default-token-abc"},
		},
		// 이건 남아야 한다.
		map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "nullus-config"}},
	))

	require.Len(t, items, 1)
	assert.Equal(t, "nullus-config", items[0]["metadata"].(map[string]any)["name"])
}

func TestSanitize_Helm_릴리스_Secret_은_남긴다(t *testing.T) {
	// 소유자가 없는 Helm 릴리스 Secret 은 되살려야 한다 — 릴리스 메타데이터가
	// 여기 들어 있다.
	items := sanitizedItems(t, rawList(map[string]any{
		"kind": "Secret", "type": "helm.sh/release.v1",
		"metadata": map[string]any{"name": "sh.helm.release.v1.nullus.v1"},
	}))
	require.Len(t, items, 1)
}

func TestSanitize_깨진_입력(t *testing.T) {
	_, err := sanitize([]byte("not json"))
	require.Error(t, err)
}

// ── 결함: ESO 배선이 빠져 복구가 "성공" 하고도 스택이 뜨지 않았다 ────────
//
// ESO 가 만든 Secret 은 ownerReferences 때문에 건너뛴다(소유자가 다시 만든다).
// 그런데 그 소유자인 CR 까지 빠지면 다시 만들 주체가 없어진다. 실환경
// 리허설에서 Gitea·Harbor·Jenkins 가 CreateContainerConfigError 로 멈췄다.

func TestDumpKinds_ESO_배선을_담는다(t *testing.T) {
	for _, k := range []string{
		"externalsecrets.external-secrets.io",
		"secretstores.external-secrets.io",
	} {
		assert.Contains(t, dumpKinds, k,
			"%s 가 빠지면 ESO 가 만들던 Secret 을 되살릴 주체가 사라진다", k)
	}
}

func TestSanitize_ESO_가_만든_Secret_은_건너뛴다(t *testing.T) {
	// 값은 금고가 SoT 다. 소유자(ExternalSecret)가 복원되면 ESO 가 다시 만든다.
	// 여기서 되살리면 금고와 어긋난 옛 값이 유효한 것처럼 남는다.
	items := sanitizedItems(t, rawList(map[string]any{
		"kind": "Secret",
		"metadata": map[string]any{
			"name": "nullus-postgresql-credentials",
			"ownerReferences": []any{map[string]any{
				"kind": "ExternalSecret", "name": "nullus-postgresql-credentials",
			}},
		},
	}))
	assert.Empty(t, items)
}

func TestSanitize_ExternalSecret_자체는_남긴다(t *testing.T) {
	items := sanitizedItems(t, rawList(map[string]any{
		"kind":     "ExternalSecret",
		"metadata": map[string]any{"name": "nullus-postgresql-credentials"},
		"spec":     map[string]any{"refreshInterval": "1h"},
	}))
	require.Len(t, items, 1, "배선은 되살려야 ESO 가 Secret 을 다시 만든다")
}

// ── 결함: CRD 가 없는 상태에서 CR 을 apply 해 복구가 죽었다 ──────────────
//
// CRD 는 클러스터 범위라 네임스페이스 덤프에 들어가지 않는다. 그래서 복구가
// ESO 의 SecretStore/ExternalSecret 을 밀 때 "no matches for kind" 로 실패했다.

func TestSanitizeOwnedBy_스택이_소유한_것만_남긴다(t *testing.T) {
	// 클러스터 범위라 남의 것까지 되돌리면 클러스터 전체에 영향을 준다.
	raw := rawList(
		map[string]any{
			"kind": "CustomResourceDefinition",
			"metadata": map[string]any{
				"name":        "externalsecrets.external-secrets.io",
				"annotations": map[string]any{"meta.helm.sh/release-namespace": "nullus-app"},
			},
		},
		map[string]any{
			"kind": "CustomResourceDefinition",
			"metadata": map[string]any{
				"name":        "certificates.cert-manager.io",
				"annotations": map[string]any{"meta.helm.sh/release-namespace": "other-ns"},
			},
		},
		map[string]any{
			"kind":     "CustomResourceDefinition",
			"metadata": map[string]any{"name": "no-owner.example.com"},
		},
	)
	out, err := sanitizeOwnedBy(raw, "nullus-app")
	require.NoError(t, err)

	var parsed struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(out, &parsed))
	require.Len(t, parsed.Items, 1)
	assert.Equal(t, "externalsecrets.external-secrets.io",
		parsed.Items[0]["metadata"].(map[string]any)["name"])
}

func TestSanitizeOwnedBy_소유한_것이_없으면_비어_있다(t *testing.T) {
	out, err := sanitizeOwnedBy(rawList(map[string]any{
		"kind":     "CustomResourceDefinition",
		"metadata": map[string]any{"name": "x"},
	}), "nullus-app")
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestClusterScopedKinds_CRD_를_담는다(t *testing.T) {
	assert.Contains(t, clusterScopedKinds, "customresourcedefinitions.apiextensions.k8s.io",
		"CRD 가 빠지면 그 CR 을 복원할 수 없다")
}

func TestClusterScopedKinds_RBAC_를_담는다(t *testing.T) {
	// 컨트롤러의 ClusterRole/ClusterRoleBinding 이 빠지면, CR 과 CRD 를 되돌려도
	// 컨트롤러가 그것을 **볼 권한이 없어** 조정이 시작되지 않는다. 실환경
	// 리허설에서 ESO 가 정확히 그렇게 멈췄다:
	//
	//   externalsecrets.external-secrets.io is forbidden: User
	//   "system:serviceaccount:nullus-app:external-secrets" cannot list
	//   resource "externalsecrets" ... at the cluster scope
	//
	// 그 결과는 결함 ③ 과 똑같다 — 복구가 succeeded 를 반환하고도 Gitea·Harbor·
	// Jenkins 가 CreateContainerConfigError 로 멈춘 채 남는다.
	for _, kind := range []string{
		"clusterroles.rbac.authorization.k8s.io",
		"clusterrolebindings.rbac.authorization.k8s.io",
	} {
		assert.Contains(t, clusterScopedKinds, kind,
			"%s 가 빠지면 컨트롤러가 자기 CR 을 볼 권한을 잃는다", kind)
	}
}

func TestSanitizeOwnedBy_남의_클러스터_RBAC_은_건드리지_않는다(t *testing.T) {
	// ClusterRole/ClusterRoleBinding 은 클러스터 전체에 걸린다. 소유권을 안
	// 보고 되돌리면 스택 복구가 **다른 스택이나 시스템의 권한을 덮어쓴다** —
	// CRD 보다 훨씬 비싼 사고다.
	raw := rawList(
		map[string]any{
			"kind": "ClusterRole",
			"metadata": map[string]any{
				"name":        "external-secrets-controller",
				"annotations": map[string]any{"meta.helm.sh/release-namespace": "nullus-app"},
			},
		},
		map[string]any{
			"kind": "ClusterRoleBinding",
			"metadata": map[string]any{
				"name":        "external-secrets-controller",
				"annotations": map[string]any{"meta.helm.sh/release-namespace": "nullus-app"},
			},
		},
		// 다른 스택의 것
		map[string]any{
			"kind": "ClusterRole",
			"metadata": map[string]any{
				"name":        "other-stack-controller",
				"annotations": map[string]any{"meta.helm.sh/release-namespace": "other-ns"},
			},
		},
		// 쿠버네티스 기본 제공 (어노테이션 없음)
		map[string]any{
			"kind":     "ClusterRole",
			"metadata": map[string]any{"name": "cluster-admin"},
		},
	)
	out, err := sanitizeOwnedBy(raw, "nullus-app")
	require.NoError(t, err)

	var parsed struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(out, &parsed))

	names := make([]string, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		names = append(names, it["metadata"].(map[string]any)["name"].(string))
	}
	assert.ElementsMatch(t, []string{"external-secrets-controller", "external-secrets-controller"}, names)
	assert.NotContains(t, names, "cluster-admin", "시스템 ClusterRole 을 백업에 담으면 복구가 클러스터를 망친다")
	assert.NotContains(t, names, "other-stack-controller")
}

func TestDumpKinds_ArgoCD_애플리케이션을_담는다(t *testing.T) {
	// 앱의 Deployment 는 **Git 에서 파생된다** — Argo CD 가 매니페스트를 보고
	// 만든다. 그래서 Deployment 만 되돌리면 백업 시점의 이미지 태그가 그대로
	// 굳는다. Argo CD 가 다시 맞춰 줘야 하는데, 그러려면 Application CR 이
	// 있어야 한다.
	//
	// 실환경 리허설에서 복구 뒤 Application 이 하나도 없었고, 되살아난
	// Deployment 는 스캐폴드 초기값 :bootstrap 을 가리킨 채 ImagePullBackOff
	// 로 남았다 — 그 태그는 레지스트리에 존재하지 않는다. 조정할 주체가 없어
	// 스스로 회복하지 못한다.
	//
	// delete_stack.go 의 argoCDCRDNames 는 이 세 가지를 이미 알고 있었다.
	// 지우는 쪽만 알고 백업하는 쪽은 몰랐다.
	for _, kind := range []string{
		"applications.argoproj.io",
		"appprojects.argoproj.io",
	} {
		assert.Contains(t, dumpKinds, kind,
			"%s 가 빠지면 복구 후 앱을 배포할 주체가 사라진다", kind)
	}
}
