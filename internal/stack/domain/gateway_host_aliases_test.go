package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pinned = []string{"gitlab.nullus.local", "registry.nullus.local"}

func TestGatewayPinnedHostnames(t *testing.T) {
	assert.Equal(t, pinned, GatewayPinnedHostnames(" nullus.local "))
	assert.Nil(t, GatewayPinnedHostnames(""))
}

func argoValues(ip string) map[string]any {
	return map[string]any{
		"global": map[string]any{
			"hostAliases": []any{
				map[string]any{"ip": ip, "hostnames": []any{"gitlab.nullus.local", "registry.nullus.local"}},
				// 사용자가 더한 것은 건드리지 않는다.
				map[string]any{"ip": "192.0.2.10", "hostnames": []any{"vault.example.com"}},
			},
		},
		"server": map[string]any{"replicas": 1},
	}
}

func TestRepinArgoCDHostAliases(t *testing.T) {
	values := argoValues("10.96.45.47")

	changed, old := RepinArgoCDHostAliases(values, pinned, "10.96.200.9")

	assert.True(t, changed)
	assert.Equal(t, []string{"10.96.45.47"}, old)
	aliases := values["global"].(map[string]any)["hostAliases"].([]any)
	assert.Equal(t, "10.96.200.9", aliases[0].(map[string]any)["ip"])
	assert.Equal(t, "192.0.2.10", aliases[1].(map[string]any)["ip"], "스택 도구가 아닌 항목은 그대로")
	assert.Equal(t, map[string]any{"replicas": 1}, values["server"])
}

// 주소가 같으면 바꾸지 않는다 — 같은 클러스터로 되돌려 IP 가 그대로인 흔한 경우다.
func TestRepinArgoCDHostAliases_SameIPIsNoop(t *testing.T) {
	values := argoValues("10.96.45.47")
	changed, old := RepinArgoCDHostAliases(values, pinned, "10.96.45.47")
	assert.False(t, changed)
	assert.Empty(t, old)
}

func TestRepinArgoCDHostAliases_NothingPinned(t *testing.T) {
	changed, _ := RepinArgoCDHostAliases(map[string]any{"server": map[string]any{}}, pinned, "10.96.200.9")
	assert.False(t, changed)
	changed, _ = RepinArgoCDHostAliases(nil, pinned, "10.96.200.9")
	assert.False(t, changed)
}

// 러너 설정은 TOML 문자열이다(설치가 helper 이미지·CA 볼륨과 함께 runners.config 에 넣는다).
const runnerConfig = `[[runners]]
  [runners.kubernetes]
    helper_image = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:arm64-v18.4.0"
    [[runners.kubernetes.volumes.secret]]
      name = "nullus-internal-ca"
      mount_path = "/etc/nullus/ca"
      read_only = true
    [[runners.kubernetes.host_aliases]]
      ip = "10.96.45.47"
      hostnames = ["gitlab.nullus.local", "registry.nullus.local"]
    [[runners.kubernetes.host_aliases]]
      ip = "192.0.2.10"
      hostnames = ["vault.example.com"]
`

func TestRepinRunnerHostAliases(t *testing.T) {
	values := map[string]any{"runners": map[string]any{"config": runnerConfig}, "concurrent": 4}

	changed, old := RepinRunnerHostAliases(values, pinned, "10.96.200.9")

	assert.True(t, changed)
	assert.Equal(t, []string{"10.96.45.47"}, old)
	got := values["runners"].(map[string]any)["config"].(string)
	assert.Equal(t, strings.Replace(runnerConfig, `ip = "10.96.45.47"`, `ip = "10.96.200.9"`, 1), got,
		"바뀌는 것은 스택 도구 블록의 ip 한 줄뿐이다")
	assert.Equal(t, 4, values["concurrent"])
}

func TestRepinRunnerHostAliases_SameIPIsNoop(t *testing.T) {
	values := map[string]any{"runners": map[string]any{"config": runnerConfig}}
	changed, old := RepinRunnerHostAliases(values, pinned, "10.96.45.47")
	assert.False(t, changed)
	assert.Empty(t, old)
	assert.Equal(t, runnerConfig, values["runners"].(map[string]any)["config"])
}

func TestRepinRunnerHostAliases_NothingPinned(t *testing.T) {
	values := map[string]any{"runners": map[string]any{"config": "[[runners]]\n"}}
	changed, _ := RepinRunnerHostAliases(values, pinned, "10.96.200.9")
	assert.False(t, changed)

	changed, _ = RepinRunnerHostAliases(map[string]any{}, pinned, "10.96.200.9")
	assert.False(t, changed)
}

// 대상 호스트를 모르면(접근 도메인 없음) 아무것도 바꾸지 않는다.
func TestRepinHostAliases_NoHostnames(t *testing.T) {
	values := argoValues("10.96.45.47")
	changed, _ := RepinArgoCDHostAliases(values, nil, "10.96.200.9")
	require.False(t, changed)
}
