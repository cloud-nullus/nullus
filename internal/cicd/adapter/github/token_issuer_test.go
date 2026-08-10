package github

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type stubSecrets struct {
	token string
	err   error
	calls []string
}

func (s *stubSecrets) GetTokenForStack(_ context.Context, provider, stackID, path string) (string, error) {
	s.calls = append(s.calls, provider+"|"+stackID+"|"+path)
	return s.token, s.err
}

func TestTokenIssuer_ImplementsPort(t *testing.T) {
	var _ port.SCMTokenIssuer = (*TokenIssuer)(nil)
}

func TestEnsureToken_ReturnsStoredPAT(t *testing.T) {
	secrets := &stubSecrets{token: "ghp_stored"}
	issuer := NewTokenIssuer(secrets)

	token, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stack-1", OrgID: "org-1", Env: "dev",
	})

	require.NoError(t, err)
	assert.Equal(t, "ghp_stored", token)
	assert.Equal(t, []string{"openbao|stack-1|kv/nullus/dev/org-1/cicd/github/api-token"}, secrets.calls)
}

// GitHub 은 SaaS 라 우리가 토큰을 만들 수 없다. 없는데 빈 값으로 진행하면
// 리포 생성이 401 로 죽고 원인이 프로비저닝 버그처럼 보인다.
func TestEnsureToken_FailsWithActionableMessageWhenMissing(t *testing.T) {
	issuer := NewTokenIssuer(&stubSecrets{token: "  "})

	_, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stack-1", OrgID: "org-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "등록된 GitHub PAT 가 없습니다")
	assert.Contains(t, err.Error(), "read:packages")
}

func TestEnsureToken_WrapsStoreError(t *testing.T) {
	issuer := NewTokenIssuer(&stubSecrets{err: errors.New("vault sealed")})

	_, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stack-1", OrgID: "org-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault sealed")
}

// Force 는 GitLab 에서 재발급을 뜻하지만 GitHub 에는 재발급 경로가 없다.
// 그렇다고 오류를 내면 401 폴백 경로가 통째로 막힌다 — 그대로 읽어야 한다.
func TestEnsureToken_ForceStillReadsStoredToken(t *testing.T) {
	issuer := NewTokenIssuer(&stubSecrets{token: "ghp_stored"})

	token, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stack-1", OrgID: "org-1", Force: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "ghp_stored", token)
}

func TestEnsureToken_RequiresOrgAndStack(t *testing.T) {
	issuer := NewTokenIssuer(&stubSecrets{token: "ghp_stored"})

	_, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{StackID: "stack-1"})
	require.Error(t, err)

	_, err = issuer.EnsureToken(context.Background(), port.SCMTokenSpec{OrgID: "org-1"})
	require.Error(t, err)
}
