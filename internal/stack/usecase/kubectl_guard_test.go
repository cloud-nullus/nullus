package usecase

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeKubectlOnPath 는 호출 인자를 기록하고 바로 끝나는 kubectl 을 PATH 앞에 둔다.
// 실제 kubectl 이 불렸는지를 환경과 무관하게 확인하기 위해서다.
func fakeKubectlOnPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\necho \"$@\" >> '" + logPath + "'\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func kubectlCalled(t *testing.T, logPath string) bool {
	t.Helper()
	_, err := os.Stat(logPath)
	return err == nil
}

// 서버 주소가 없는 kubeconfig 로 kubectl 을 실행하면, kubectl 은 조용히
// http://localhost:8080 으로 폴백한다. 그 자리에 무엇이 떠 있든 삭제 요청이 간다.
//
// 테스트에서는 이것이 무한 대기로 드러났다 — 이 머신의 8080 을 다른 서비스가
// 점유하고 응답하지 않아 kubectl 이 32초 타임아웃 × 재시도로 매달렸다.
// CI 는 8080 이 비어 즉시 실패해 드러나지 않았다.
func TestRunKubectlWithKubeconfig_RefusesKubeconfigWithoutServer(t *testing.T) {
	logPath := fakeKubectlOnPath(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, kc := range [][]byte{
		[]byte("apiVersion: v1\n"),
		[]byte("apiVersion: v1\nclusters:\n- name: kind\n"),
		nil,
	} {
		_, err := runKubectlWithKubeconfig(ctx, kc, "delete", "-n", "nullus", "deploy/x")
		assert.Error(t, err, "서버 없는 kubeconfig %q 는 거부해야 한다", string(kc))
	}
	assert.False(t, kubectlCalled(t, logPath), "서버 없는 kubeconfig 로 kubectl 을 실행했다")
}

func TestDeleteManifest_RefusesKubeconfigWithoutServer(t *testing.T) {
	logPath := fakeKubectlOnPath(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := deleteManifest(ctx, []byte("apiVersion: v1\nclusters:\n- name: kind\n"), "nullus",
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n")
	assert.Error(t, err)
	assert.False(t, kubectlCalled(t, logPath))
}

// 가드가 과하면 정상 클러스터에서 삭제가 조용히 멈춘다. 서버가 있으면 실행한다.
func TestRunKubectlWithKubeconfig_RunsWithRealKubeconfig(t *testing.T) {
	logPath := fakeKubectlOnPath(t)
	// 넉넉히 둔다. 막 만든 스크립트의 첫 실행은 부하가 높은 macOS 에서 수 초 걸린다.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := runKubectlWithKubeconfig(ctx, []byte(validKubeconfigForGuard), "get", "ns")
	require.NoError(t, err)
	assert.True(t, kubectlCalled(t, logPath), "서버가 있는 kubeconfig 인데 kubectl 을 실행하지 않았다")
}

const validKubeconfigForGuard = `apiVersion: v1
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
