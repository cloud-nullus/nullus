package sastcreds

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	values map[string]string
	err    error
	seen   []string
}

func (f *fakeStore) GetTokenForStack(_ context.Context, provider, stackID, path string) (string, error) {
	f.seen = append(f.seen, provider+"|"+stackID+"|"+path)
	if f.err != nil {
		return "", f.err
	}
	return f.values[path], nil
}

// 분석 토큰은 스택 설치가 발급해 OpenBao 에 둔다. 플랫폼이 이미 갖고 있는 값을
// 사용자에게 다시 받지 않는다.
func TestAnalysisToken_FromStackSecretStore(t *testing.T) {
	store := &fakeStore{values: map[string]string{
		"kv/nullus/dev/org-1/security/sonarqube/analysis-token": "sqa_token",
	}}

	got, err := New(store, "dev", "org-1", "stk-1").AnalysisToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "sqa_token", got)
	assert.Equal(t, []string{"openbao|stk-1|kv/nullus/dev/org-1/security/sonarqube/analysis-token"}, store.seen,
		"스택의 OpenBao 에서 읽어야 한다 — 전역 금고에는 그 값이 없다")
}

func TestAnalysisToken_DefaultEnv(t *testing.T) {
	store := &fakeStore{values: map[string]string{
		"kv/nullus/dev/org-1/security/sonarqube/analysis-token": "sqa_token",
	}}
	got, err := New(store, "", "org-1", "stk-1").AnalysisToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "sqa_token", got)
}

func TestAnalysisToken_PropagatesStoreError(t *testing.T) {
	_, err := New(&fakeStore{err: errors.New("sealed")}, "dev", "org-1", "stk-1").AnalysisToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sealed")
}

// 조직·스택을 모르면 읽지 않는다. 엉뚱한 경로의 값을 토큰으로 쓰면 원인이 먼 인증 실패가 된다.
func TestAnalysisToken_RequiresScope(t *testing.T) {
	store := &fakeStore{}
	got, err := New(store, "dev", "", "stk-1").AnalysisToken(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, store.seen)
}
