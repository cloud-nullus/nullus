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

const runnerTestKubeconfig = `apiVersion: v1
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

// fakeGitLabKubectl 은 호출 인자를 순서대로 남기고, toolbox exec 에는 러너 토큰
// 조사 결과를, wait 에는 waitOutput/waitExit 을 돌려주는 kubectl 이다.
func fakeGitLabKubectl(t *testing.T, waitOutput string, waitExit int) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\n" +
		"echo \"$*\" >> '" + logPath + "'\n" +
		"case \" $* \" in\n" +
		"  *\" wait \"*) printf '%s\\n' '" + waitOutput + "'; exit " + string(rune('0'+waitExit)) + " ;;\n" +
		"  *\" exec \"*) printf 'registration_allowed=true\\nregistration_token=REG123\\nauth_token=\\n'; exit 0 ;;\n" +
		"esac\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func callLines(t *testing.T, logPath string) []string {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// GitLab 마이그레이션은 끝날 때 러너 등록 토큰을 차트 Secret 값으로 다시 넣는다.
// 그 전에 토큰을 읽으면 곧 무효가 될 값을 러너에 넘긴다 — kind 스택에서 러너가
// "403 invalid token" 으로 등록에 실패해 CI 가 한 건도 돌지 않았다(읽은 시각
// 10:26:4x, 마이그레이션 완료 10:26:41).
func TestDiscoverRunnerToken_WaitsForGitLabMigrationsFirst(t *testing.T) {
	logPath := fakeGitLabKubectl(t, "job.batch/gitlab-migrations-8976b56 condition met", 0)
	o := NewOrchestrator(nil, []byte(runnerTestKubeconfig), "scan-e2e")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := o.discoverGitLabRunnerRegistrationToken(ctx, "scan-e2e")
	require.NoError(t, err)
	assert.Equal(t, "REG123", token.Value)

	waitIdx, execIdx := -1, -1
	for i, line := range callLines(t, logPath) {
		if strings.Contains(line, " wait ") && strings.Contains(line, "--for=condition=complete") &&
			strings.Contains(line, gitLabMigrationsJobSelector) && strings.Contains(line, "-n scan-e2e") {
			waitIdx = i
		}
		if strings.Contains(line, "exec deploy/gitlab-toolbox") && execIdx < 0 {
			execIdx = i
		}
	}
	require.GreaterOrEqual(t, waitIdx, 0, "마이그레이션 완료를 기다리지 않았다")
	require.GreaterOrEqual(t, execIdx, 0, "토큰을 읽지 않았다")
	assert.Less(t, waitIdx, execIdx, "마이그레이션이 끝나기 전에 토큰을 읽었다")
}

// 기다릴 마이그레이션 잡이 없으면(이미 정리됨, 외부 GitLab) 그대로 진행한다 —
// 없는 잡을 기다리다 설치가 멈추면 안 된다.
func TestDiscoverRunnerToken_ProceedsWhenNoMigrationsJob(t *testing.T) {
	fakeGitLabKubectl(t, "error: no matching resources found", 1)
	o := NewOrchestrator(nil, []byte(runnerTestKubeconfig), "scan-e2e")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := o.discoverGitLabRunnerRegistrationToken(ctx, "scan-e2e")
	require.NoError(t, err)
	assert.Equal(t, "REG123", token.Value)
}
