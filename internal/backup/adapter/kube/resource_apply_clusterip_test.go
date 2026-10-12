package kube

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeKubectl 은 PATH 앞에 가짜 kubectl 을 둔다. apply 마다 받은 문서를 apply-<n>.json 으로 남기고,
// 첫 apply 의 stdin 에 failWhen 이 있으면 stderr 를 내며 실패한다.
func fakeKubectl(t *testing.T, failWhen, stderr string) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
for a in "$@"; do if [ "$a" = "apply" ]; then
  n=$(ls "` + dir + `" | grep -c '^apply-' || true)
  n=$((n+1))
  cat > "` + dir + `/apply-$n.json"
  if [ -n "` + failWhen + `" ] && grep -q "` + failWhen + `" "` + dir + `/apply-$n.json"; then
    echo '` + stderr + `' >&2; exit 1
  fi
  exit 0
fi; done
exit 0
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func applied(t *testing.T, dir string, n int) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "apply-"+string(rune('0'+n))+".json"))
	require.NoError(t, err)
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc.Items
}

func serviceList() []byte {
	b, _ := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{
		map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "envoy-gateway"},
			"spec": map[string]any{"clusterIP": "10.96.45.47", "clusterIPs": []any{"10.96.45.47"}}},
		map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "openbao-internal"},
			"spec": map[string]any{"clusterIP": "None", "clusterIPs": []any{"None"}}},
		map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": "gitlab"}},
	}})
	return b
}

// 같은 클러스터의 빈 네임스페이스로 되돌리면 옛 IP 로 그대로 선다 — hostAliases 가 맞는다.
func TestApply_되살린_ClusterIP_로_적용한다(t *testing.T) {
	dir := fakeKubectl(t, "", "")

	require.NoError(t, NewResourceDumper([]byte("kubeconfig")).Apply(context.Background(), "nullus", strings.NewReader(string(serviceList()))))

	items := applied(t, dir, 1)
	assert.Equal(t, "10.96.45.47", items[0]["spec"].(map[string]any)["clusterIP"])
	_, err := os.Stat(filepath.Join(dir, "apply-2.json"))
	assert.True(t, os.IsNotExist(err), "한 번에 됐으면 다시 적용하지 않는다")
}

// 그 IP 를 이미 다른 Service 가 쓰거나(같은 클러스터), 범위 밖이면(다른 클러스터) 거절된다. 복구를
// 멈추지 않고 새 주소로 되돌린다 — headless 는 그대로 둔다.
func TestApply_ClusterIP_가_거절되면_새_주소로_되돌린다(t *testing.T) {
	dir := fakeKubectl(t, "10.96.45.47",
		`The Service "envoy-gateway" is invalid: spec.clusterIPs: Invalid value: []string{"10.96.45.47"}: failed to allocate IP 10.96.45.47: provided IP is already allocated`)

	require.NoError(t, NewResourceDumper([]byte("kubeconfig")).Apply(context.Background(), "nullus", strings.NewReader(string(serviceList()))))

	retry := applied(t, dir, 2)
	require.Len(t, retry, 3)
	gw := retry[0]["spec"].(map[string]any)
	assert.NotContains(t, gw, "clusterIP")
	assert.NotContains(t, gw, "clusterIPs")
	assert.Equal(t, "None", retry[1]["spec"].(map[string]any)["clusterIP"], "headless 는 None 이 정체다")
}

// 거절된 Service 만 새 주소로 돌린다. 나머지는 첫 적용에서 이미 백업 때의 IP 로 섰다 — 그것까지
// "새 주소" 라고 알리면 운영자가 멀쩡한 게이트웨이의 hostAliases 를 고치러 간다.
func TestApply_거절된_Service_만_새_주소로_돌린다(t *testing.T) {
	dir := fakeKubectl(t, "10.96.0.77",
		`The Service "other" is invalid: spec.clusterIPs: Invalid value: ["10.96.0.77"]: failed to allocate IP 10.96.0.77: provided IP is already allocated`)
	list, _ := json.Marshal(map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{
		map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "envoy-gateway"},
			"spec": map[string]any{"clusterIP": "10.96.45.47", "clusterIPs": []any{"10.96.45.47"}}},
		map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "other"},
			"spec": map[string]any{"clusterIP": "10.96.0.77", "clusterIPs": []any{"10.96.0.77"}}},
	}})

	require.NoError(t, NewResourceDumper([]byte("kubeconfig")).Apply(context.Background(), "nullus", strings.NewReader(string(list))))

	retry := applied(t, dir, 2)
	assert.Equal(t, "10.96.45.47", retry[0]["spec"].(map[string]any)["clusterIP"], "거절되지 않은 게이트웨이는 백업 IP 그대로")
	assert.NotContains(t, retry[1]["spec"].(map[string]any), "clusterIP")
}

// ClusterIP 와 무관한 실패는 다시 시도하지 않고 그대로 알린다.
func TestApply_ClusterIP_와_무관한_실패는_그대로_돌려준다(t *testing.T) {
	dir := fakeKubectl(t, "gitlab", `error: no matches for kind "Gateway" in version "gateway.networking.k8s.io/v1"`)

	err := NewResourceDumper([]byte("kubeconfig")).Apply(context.Background(), "nullus", strings.NewReader(string(serviceList())))

	require.Error(t, err)
	_, statErr := os.Stat(filepath.Join(dir, "apply-2.json"))
	assert.True(t, os.IsNotExist(statErr))
}
