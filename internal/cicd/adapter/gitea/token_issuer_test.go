package gitea

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type recordingKubectl struct {
	calls   [][]string
	podName string
	token   string
}

func (r *recordingKubectl) run(_ context.Context, _ []byte, args ...string) ([]byte, error) {
	r.calls = append(r.calls, args)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "get pod") {
		return []byte(r.podName), nil
	}
	return []byte(r.token), nil
}

type memSecrets struct{ m map[string]string }

func (s *memSecrets) GetTokenForStack(_ context.Context, _, _, path string) (string, error) {
	v, ok := s.m[path]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return v, nil
}

func (s *memSecrets) PutTokenForStack(_ context.Context, _, _, path, value string) error {
	s.m[path] = value
	return nil
}

func spec() port.SCMTokenSpec {
	return port.SCMTokenSpec{
		StackID: "stk_1", ClusterID: "c1", Namespace: "nullus-gj3", OrgID: "org-1", Env: "dev", Force: true,
	}
}

// Gitea 차트는 버전·설정에 따라 Deployment 로도 StatefulSet 으로도 배포된다.
// 실제로 차트 12.7.0 은 Deployment 를 만드는데, 워크로드 종류를 고정해 두면
// "statefulsets.apps \"gitea\" not found" 로 토큰 발급이 죽는다 — 스택은 정상
// 설치됐는데 파이프라인 생성만 실패하는, 원인이 멀리 떨어진 실패다.
//
// 레이블로 파드를 먼저 찾아 그 파드에 exec 한다.
func TestTokenIssuer_ResolvesPodByLabelNotWorkloadKind(t *testing.T) {
	kc := &recordingKubectl{podName: "gitea-fcdd599dd-rq28s", token: "tok-1"}
	issuer := NewTokenIssuer(nil, kc.run, &memSecrets{m: map[string]string{}})

	token, err := issuer.EnsureToken(context.Background(), spec())
	require.NoError(t, err)
	assert.Equal(t, "tok-1", token)

	require.Len(t, kc.calls, 2, "파드 조회 → exec 두 번이어야 한다")

	lookup := strings.Join(kc.calls[0], " ")
	assert.Contains(t, lookup, "get pod")
	assert.Contains(t, lookup, "app.kubernetes.io/name=gitea")

	exec := strings.Join(kc.calls[1], " ")
	assert.Contains(t, exec, "gitea-fcdd599dd-rq28s",
		"조회한 파드 이름으로 exec 해야 한다")
	assert.NotContains(t, exec, "statefulset/",
		"워크로드 종류를 고정하면 차트 구성이 바뀔 때 깨진다")
}

// 파드를 못 찾았는데 exec 로 넘어가면 빈 이름으로 호출해 엉뚱한 오류가 난다.
func TestTokenIssuer_NoPodFoundFailsClearly(t *testing.T) {
	kc := &recordingKubectl{podName: "  ", token: "tok"}
	issuer := NewTokenIssuer(nil, kc.run, &memSecrets{m: map[string]string{}})

	_, err := issuer.EnsureToken(context.Background(), spec())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Gitea 파드")
}

// 보관된 토큰이 있으면 재발급하지 않는다(Force 아닐 때).
func TestTokenIssuer_ReusesStoredToken(t *testing.T) {
	path := TokenSecretPath("dev", "org-1")
	kc := &recordingKubectl{podName: "p", token: "new"}
	issuer := NewTokenIssuer(nil, kc.run, &memSecrets{m: map[string]string{path: "stored"}})

	s := spec()
	s.Force = false
	token, err := issuer.EnsureToken(context.Background(), s)

	require.NoError(t, err)
	assert.Equal(t, "stored", token)
	assert.Empty(t, kc.calls, "보관된 토큰이 있으면 kubectl 을 부르지 않는다")
}

// Gitea 1.27 에는 delete-access-token 서브커맨드가 없다(create/list/delete/
// change-password/generate-access-token/must-change-password 뿐).
// 없는 명령을 부르면 삭제가 조용히 실패하고, 이어지는 생성이
// "access token name has been used already" 로 죽는다.
//
// 대신 파드 안에서 Gitea API 로 폐기한다 — 파드는 localhost:3000 에 닿는다.
func TestTokenIssuer_RevokesViaAPINotMissingCLICommand(t *testing.T) {
	kc := &recordingKubectl{podName: "gitea-abc", token: "tok-1"}
	secrets := &memSecrets{m: map[string]string{
		AdminPasswordPath("dev", "org-1"): "admin-pw",
	}}
	issuer := NewTokenIssuer(nil, kc.run, secrets)

	_, err := issuer.EnsureToken(context.Background(), spec())
	require.NoError(t, err)

	all := ""
	for _, c := range kc.calls {
		all += strings.Join(c, " ") + "\n"
	}
	assert.NotContains(t, all, "delete-access-token",
		"Gitea CLI 에 없는 명령이다")
	assert.Contains(t, all, "DELETE",
		"기존 토큰은 API 로 폐기해야 이름 충돌이 나지 않는다")
	assert.Contains(t, all, AutomationTokenName)
}
