package helm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeKubectlOnPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + logPath + "'\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// looksLikeKubeconfig 는 apiVersion·clusters 만 본다. 그걸 통과해도 서버 주소가
// 없으면 kubectl 은 localhost:8080 으로 폴백한다 — 설치 경로가 엉뚱한 곳에
// apply 를 날리게 된다. 실행 지점에서 막는다.
func TestOrchestrator_KubectlRefusesKubeconfigWithoutServer(t *testing.T) {
	logPath := fakeKubectlOnPath(t)
	o := NewOrchestrator(nil, []byte("apiVersion: v1\nclusters:\n- name: test\n"), "nullus")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := o.runKubectl(ctx, "get", "crd")
	assert.Error(t, err)
	_, err = o.runKubectlWithStdin(ctx, "apiVersion: v1\n", "apply", "-f", "-")
	assert.Error(t, err)
	assert.Error(t, o.applyManifest(ctx, "nullus", "apiVersion: v1\nkind: ConfigMap\n"))

	_, statErr := os.Stat(logPath)
	assert.True(t, os.IsNotExist(statErr), "서버 없는 kubeconfig 로 kubectl 을 실행했다")
}

func TestOrchestrator_KubectlRunsWithRealKubeconfig(t *testing.T) {
	logPath := fakeKubectlOnPath(t)
	o := NewOrchestrator(nil, []byte(`apiVersion: v1
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
`), "nullus")
	// 넉넉히 둔다. 막 만든 스크립트의 첫 실행은 부하가 높은 macOS 에서 수 초 걸린다.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := o.runKubectl(ctx, "get", "ns")
	require.NoError(t, err)
	_, statErr := os.Stat(logPath)
	assert.NoError(t, statErr, "서버가 있는 kubeconfig 인데 kubectl 을 실행하지 않았다")
}
