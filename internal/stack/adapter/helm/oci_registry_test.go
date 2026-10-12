package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OCI 차트(envoy gateway)를 받으려면 action.Configuration 에 레지스트리
// 클라이언트가 있어야 한다. 없으면 Helm 이 "missing registry client" 로 죽고,
// 설치는 helm CLI 폴백에 의존하게 된다 — 그 폴백은 CLI 버전에 묶여 있어
// helm v4 가 PATH 에 있으면 함께 실패한다(2026-09-10 실측).
func TestNewActionConfig_HasRegistryClient(t *testing.T) {
	kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user: {}
`)

	cfg, err := newActionConfig(kubeconfig, "nullus")
	require.NoError(t, err)

	assert.NotNil(t, cfg.RegistryClient,
		"레지스트리 클라이언트가 없으면 oci:// 차트를 해석하지 못한다")
}
