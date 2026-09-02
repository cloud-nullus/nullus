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
