package kubeconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const withServer = `apiVersion: v1
kind: Config
clusters:
- name: kind
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: kind
  context:
    cluster: kind
    user: kind
current-context: kind
users:
- name: kind
  user:
    token: t
`

func TestRequireServer_AcceptsKubeconfigWithServer(t *testing.T) {
	assert.NoError(t, RequireServer([]byte(withServer)))
}

func TestRequireServer_RejectsKubeconfigsThatFallBackToLocalhost(t *testing.T) {
	cases := map[string]string{
		"비어 있음":        "",
		"apiVersion 만": "apiVersion: v1\n",
		"클러스터 이름만":     "apiVersion: v1\nclusters:\n- name: kind\n",
		"current-context 없음": `apiVersion: v1
clusters:
- name: kind
  cluster: {server: https://127.0.0.1:6443}
contexts:
- name: kind
  context: {cluster: kind, user: kind}
`,
		"컨텍스트가 없는 클러스터를 가리킴": `apiVersion: v1
clusters:
- name: other
  cluster: {server: https://127.0.0.1:6443}
contexts:
- name: kind
  context: {cluster: kind, user: kind}
current-context: kind
`,
		"서버 주소가 빈 값": `apiVersion: v1
clusters:
- name: kind
  cluster: {server: ""}
contexts:
- name: kind
  context: {cluster: kind, user: kind}
current-context: kind
`,
		"YAML 아님": "::::",
	}
	for name, kc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.ErrorIs(t, RequireServer([]byte(kc)), ErrNoServer)
		})
	}
}
