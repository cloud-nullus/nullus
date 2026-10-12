package helm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const adoptionTestKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: kind
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: kind
  context: {cluster: kind, user: kind}
current-context: kind
users:
- name: kind
  user: {token: t}
`

// fakeAdoptionKubectl 은 kube-system Service 목록과 옛 릴리스 Secret 조회에 답하는 kubectl 이다.
// oldReleaseSecret 이 비어 있지 않으면 옛 릴리스가 살아 있는 것이다.
func fakeAdoptionKubectl(t *testing.T, kubeSystemServices, oldReleaseSecret string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> '" + logPath + "'\n" +
		"case \" $* \" in\n" +
		"  *\" get service -n kube-system \"*) printf '%s' '" + kubeSystemServices + "'; exit 0 ;;\n" +
		"  *\" get secret \"*) printf '%s' '" + oldReleaseSecret + "'; exit 0 ;;\n" +
		"esac\n" +
		"exit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func readCalls(t *testing.T, logPath string) []string {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// kube-prometheus-stack 은 컨트롤 플레인을 긁는 Service 5개(coredns·kube-controller-manager·kube-etcd·
// kube-proxy·kube-scheduler)를 스택 네임스페이스가 아니라 kube-system 에 만든다. 스택 삭제가 그 릴리스를
// 지우지 못하면 이것들이 남는데, 네임스페이스 리소스라 cluster-scoped 인수가 보지 않았다 — 2026-10-12
// 재설치가 installing_prometheus 에서 "kube-prometheus-stack-coredns ... cannot be imported" 로 실패했다.
func TestAdoptClusterScopedResources_AdoptsLeftoverKubeSystemServices(t *testing.T) {
	logPath := fakeAdoptionKubectl(t,
		"kube-dns <none> <none>\n"+
			"kube-prometheus-stack-coredns kube-prometheus-stack nullus-gitlab-argocd-sonarqube\n"+
			"kube-prometheus-stack-kube-proxy kube-prometheus-stack nullus-gitlab-argocd-sonarqube\n"+
			"kube-prometheus-stack-kubelet <none> <none>\n",
		"")
	o := NewOrchestrator(nil, []byte(adoptionTestKubeconfig), "nullus-gitlab-sca")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	o.adoptClusterScopedResources(ctx, "kube-prometheus-stack", "nullus-gitlab-sca")

	var patched []string
	for _, call := range readCalls(t, logPath) {
		if strings.Contains(call, " patch service ") {
			assert.Contains(t, call, "-n kube-system")
			assert.Contains(t, call, `"meta.helm.sh/release-namespace":"nullus-gitlab-sca"`)
			patched = append(patched, call)
		}
	}
	require.Len(t, patched, 2, "옛 릴리스가 남긴 Service 둘만 인수한다")
	assert.Contains(t, patched[0], "kube-prometheus-stack-coredns")
	assert.Contains(t, patched[1], "kube-prometheus-stack-kube-proxy")
}

// 옛 릴리스가 살아 있으면 그 Service 는 다른 스택의 것이다 — 인수하지 않는다.
func TestAdoptClusterScopedResources_KeepsKubeSystemServicesOfLiveRelease(t *testing.T) {
	logPath := fakeAdoptionKubectl(t,
		"kube-prometheus-stack-coredns kube-prometheus-stack team-a\n",
		"sh.helm.release.v1.kube-prometheus-stack.v1")
	o := NewOrchestrator(nil, []byte(adoptionTestKubeconfig), "nullus-gitlab-sca")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	o.adoptClusterScopedResources(ctx, "kube-prometheus-stack", "nullus-gitlab-sca")

	for _, call := range readCalls(t, logPath) {
		assert.NotContains(t, call, " patch service ", "살아 있는 릴리스의 Service 를 탈취했다")
	}
}
