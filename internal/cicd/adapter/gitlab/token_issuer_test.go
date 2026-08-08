package gitlab

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type fakeKubeconfig struct {
	data []byte
	err  error
}

func (f *fakeKubeconfig) GetKubeconfig(_ context.Context, _ string) ([]byte, error) {
	return f.data, f.err
}

type fakeSecretStore struct {
	values  map[string]string
	getErr  error
	putErr  error
	putLog  []string
	getCall int
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{values: map[string]string{}}
}

func (f *fakeSecretStore) GetTokenForStack(_ context.Context, provider, _, path string) (string, error) {
	f.getCall++
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.values[provider+":"+path], nil
}

func (f *fakeSecretStore) PutTokenForStack(_ context.Context, provider, _, path, value string) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.values[provider+":"+path] = value
	f.putLog = append(f.putLog, path)
	return nil
}

type kubectlCall struct{ Args []string }

func newIssuer(t *testing.T, store *fakeSecretStore, out string, execErr error) (*TokenIssuer, *[]kubectlCall) {
	t.Helper()
	calls := make([]kubectlCall, 0)
	runner := func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		calls = append(calls, kubectlCall{Args: args})
		if execErr != nil {
			return nil, execErr
		}
		return []byte(out), nil
	}
	issuer := NewTokenIssuer(&fakeKubeconfig{data: []byte("apiVersion: v1\nkind: Config\n")}, runner, store)
	return issuer, &calls
}

func TestTokenIssuer_ImplementsPort(t *testing.T) {
	var _ port.SCMTokenIssuer = (*TokenIssuer)(nil)
}

func TestEnsureToken_ReusesStoredTokenWithoutExec(t *testing.T) {
	store := newFakeSecretStore()
	store.values[SecretProvider+":"+TokenSecretPath("dev", "org-1")] = "glpat-existing"

	issuer, calls := newIssuer(t, store, "glpat-new", nil)

	token, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stk_1", ClusterID: "c1", Namespace: "devsecops", OrgID: "org-1", Env: "dev",
	})
	require.NoError(t, err)
	assert.Equal(t, "glpat-existing", token)
	assert.Empty(t, *calls, "저장된 토큰이 있으면 toolbox 에 들어가지 않는다")
}

func TestEnsureToken_IssuesAndPersistsWhenMissing(t *testing.T) {
	store := newFakeSecretStore()
	issuer, calls := newIssuer(t, store, "Defaulted container \"toolbox\" out of: toolbox\nglpat-issued123\n", nil)

	token, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stk_1", ClusterID: "c1", Namespace: "devsecops", OrgID: "org-1", Env: "dev",
	})
	require.NoError(t, err)

	// kubectl 출력의 안내 문구는 걸러내고 토큰만 남겨야 한다.
	assert.Equal(t, "glpat-issued123", token)
	assert.Equal(t, "glpat-issued123", store.values[SecretProvider+":"+TokenSecretPath("dev", "org-1")])
	require.Len(t, *calls, 1)

	args := strings.Join((*calls)[0].Args, " ")
	assert.Contains(t, args, "-n devsecops")
	assert.Contains(t, args, "deploy/gitlab-toolbox")
	assert.Contains(t, args, "gitlab-rails runner")
}

// 토큰이 만료·폐기된 경우 호출자가 Force 로 재발급을 요청한다.
func TestEnsureToken_ForceReissuesEvenWhenStored(t *testing.T) {
	store := newFakeSecretStore()
	store.values[SecretProvider+":"+TokenSecretPath("dev", "org-1")] = "glpat-stale"

	issuer, calls := newIssuer(t, store, "glpat-fresh\n", nil)

	token, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stk_1", ClusterID: "c1", Namespace: "devsecops", OrgID: "org-1", Env: "dev", Force: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "glpat-fresh", token)
	assert.Len(t, *calls, 1)
	assert.Equal(t, "glpat-fresh", store.values[SecretProvider+":"+TokenSecretPath("dev", "org-1")])
}

func TestEnsureToken_FailsWhenRailsOutputHasNoToken(t *testing.T) {
	store := newFakeSecretStore()
	issuer, _ := newIssuer(t, store, "Defaulted container \"toolbox\" out of: toolbox\n\n", nil)

	_, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stk_1", ClusterID: "c1", Namespace: "devsecops", OrgID: "org-1", Env: "dev",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "토큰")
}

func TestEnsureToken_PropagatesExecFailure(t *testing.T) {
	store := newFakeSecretStore()
	issuer, _ := newIssuer(t, store, "", errors.New(`deployments.apps "gitlab-toolbox" not found`))

	_, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{
		StackID: "stk_1", ClusterID: "c1", Namespace: "devsecops", OrgID: "org-1", Env: "dev",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gitlab-toolbox")
}

func TestEnsureToken_RequiresNamespaceAndOrg(t *testing.T) {
	store := newFakeSecretStore()
	issuer, _ := newIssuer(t, store, "glpat-x", nil)

	_, err := issuer.EnsureToken(context.Background(), port.SCMTokenSpec{StackID: "stk_1", ClusterID: "c1", OrgID: "o"})
	require.Error(t, err)

	_, err = issuer.EnsureToken(context.Background(), port.SCMTokenSpec{StackID: "stk_1", ClusterID: "c1", Namespace: "ns"})
	require.Error(t, err)
}

// 시크릿 경로는 스택 모듈의 규약(kv/nullus/{env}/{org}/{module}/{provider}/...)과 맞춘다.
func TestTokenSecretPath_FollowsSharedConvention(t *testing.T) {
	path := TokenSecretPath("dev", "11111111-1111-1111-1111-111111111111")
	assert.Equal(t, "kv/nullus/dev/11111111-1111-1111-1111-111111111111/cicd/gitlab/api-token", path)

	// env 가 비면 dev 로 떨어진다 — 로컬 실행에서 경로가 갈라지지 않게.
	assert.Contains(t, TokenSecretPath("", "org"), "/dev/")
}

func TestBuildTokenIssueScript_RevokesPriorTokensThenCreates(t *testing.T) {
	script := buildTokenIssueScript()

	assert.Contains(t, script, "find_by_username('root')")
	assert.Contains(t, script, AutomationTokenName)
	assert.Contains(t, script, "revoke", "이전 자동화 토큰을 정리해야 누적되지 않는다")
	assert.Contains(t, script, "scopes:")
	assert.Contains(t, script, "api")
	assert.Contains(t, script, "puts")
}

// 이 스크립트는 `bash -lc "gitlab-rails runner \"...\""` 로 두 겹의 셸 인용을
// 통과한다. 큰따옴표가 하나라도 섞이면 Ruby 구문이 깨져 실행 자체가 실패한다
// (실제로 where("name LIKE ...") 를 쓰다 겪었다).
func TestBuildTokenIssueScript_ContainsNoDoubleQuotes(t *testing.T) {
	script := buildTokenIssueScript()
	assert.NotContains(t, script, `"`,
		"셸 인용이 두 겹이라 큰따옴표가 있으면 Ruby 구문이 깨진다")
}
