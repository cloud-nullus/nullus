package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type fakeStackReader struct {
	summary *port.StackSummary
	err     error
}

func (f *fakeStackReader) GetStackSummary(_ context.Context, _ string) (*port.StackSummary, error) {
	return f.summary, f.err
}

type fakeTokenIssuer struct {
	token string
	err   error
	specs []port.SCMTokenSpec
}

func (f *fakeTokenIssuer) EnsureToken(_ context.Context, spec port.SCMTokenSpec) (string, error) {
	f.specs = append(f.specs, spec)
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

func gitlabStack() *port.StackSummary {
	return &port.StackSummary{
		ID: "stk_1", OrgID: "org-1", ClusterID: "c1", State: "completed",
		Namespace:        "devsecops",
		SourceRepository: "GitLab CE", ContainerRegistry: "GitLab Registry",
		AccessDomain: "nullus.local",
	}
}

// stubGitLab 은 인증 확인(Ping)에 응답하는 최소 서버다.
// acceptTokens 가 비면 아무 토큰이나 받아들인다.
func stubGitLab(t *testing.T, acceptTokens ...string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(acceptTokens) > 0 {
			ok := false
			for _, tok := range acceptTokens {
				if r.Header.Get("PRIVATE-TOKEN") == tok {
					ok = true
					break
				}
			}
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"username": "root"})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func newFactory(t *testing.T, stack *port.StackSummary, issuer *fakeTokenIssuer) *BundleFactory {
	t.Helper()
	return NewBundleFactory(&fakeStackReader{summary: stack}, issuer, Options{
		Env: "dev", GroupPath: "acme", GitLabBaseURLOverride: stubGitLab(t),
	})
}

func TestBundleFactory_ImplementsPort(t *testing.T) {
	var _ port.SCMBundleFactory = (*BundleFactory)(nil)
}

func TestFor_BuildsBundleFromStack(t *testing.T) {
	issuer := &fakeTokenIssuer{token: "glpat-x"}
	bundle, err := newFactory(t, gitlabStack(), issuer).For(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.NotNil(t, bundle.Provisioner)
	assert.NotNil(t, bundle.Pipeline)
	assert.NotNil(t, bundle.Registry)
	assert.Equal(t, "acme", bundle.GroupPath)
	// Argo CD 는 스택 네임스페이스에 설치되므로 Application 도 거기 만든다.
	assert.Equal(t, "devsecops", bundle.CDNamespace)
	assert.Equal(t, "c1", bundle.ClusterID)
}

// 토큰은 스택의 클러스터·네임스페이스를 기준으로 발급되어야 한다.
func TestFor_RequestsTokenForStackLocation(t *testing.T) {
	issuer := &fakeTokenIssuer{token: "glpat-x"}
	_, err := newFactory(t, gitlabStack(), issuer).For(context.Background(), "stk_1")
	require.NoError(t, err)

	require.Len(t, issuer.specs, 1)
	assert.Equal(t, "c1", issuer.specs[0].ClusterID)
	assert.Equal(t, "devsecops", issuer.specs[0].Namespace)
	assert.Equal(t, "org-1", issuer.specs[0].OrgID)
	assert.Equal(t, "dev", issuer.specs[0].Env)
}

// GitLab Registry 스택이면 앱 프로젝트 자신의 레지스트리를 쓴다.
func TestFor_SelectsSCMProjectRegistryForGitLabStack(t *testing.T) {
	bundle, err := newFactory(t, gitlabStack(), &fakeTokenIssuer{token: "t"}).For(context.Background(), "stk_1")
	require.NoError(t, err)

	target, err := bundle.Registry.Resolve(context.Background(), port.ImageTargetSpec{
		AppName: "myapp", SCMProjectPath: "acme/myapp",
		SCMRegistryURL: "registry.nullus.local/acme/myapp",
	})
	require.NoError(t, err)
	assert.Equal(t, port.RegistryKindSCMProject, target.Kind)
}

// Harbor 스택이면 같은 코드 경로가 Harbor 를 골라야 한다 — 이것이 추상화의 목적이다.
func TestFor_SelectsHarborForHarborStack(t *testing.T) {
	stack := gitlabStack()
	stack.ContainerRegistry = "Harbor"

	bundle, err := newFactory(t, stack, &fakeTokenIssuer{token: "t"}).For(context.Background(), "stk_1")
	require.NoError(t, err)

	target, err := bundle.Registry.Resolve(context.Background(), port.ImageTargetSpec{
		AppName: "myapp", OrgPath: "acme",
		SCMRegistryURL: "registry.nullus.local/acme/myapp",
	})
	require.NoError(t, err)
	assert.Equal(t, port.RegistryKindHarbor, target.Kind)
	// 접근 도메인에서 harbor 호스트를 유도한다.
	assert.Equal(t, "harbor.nullus.local", target.Host)
}

func TestFor_FailsWhenStackMissing(t *testing.T) {
	f := NewBundleFactory(&fakeStackReader{summary: nil}, &fakeTokenIssuer{}, Options{GroupPath: "acme"})
	_, err := f.For(context.Background(), "stk_missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stk_missing")
}

// GitLab 이 설치되지 않은 스택(GitHub 조합)에서는 저장소를 만들 수 없다.
func TestFor_FailsWhenSourceRepositoryIsNotSelfHosted(t *testing.T) {
	stack := gitlabStack()
	stack.SourceRepository = "GitHub"

	_, err := newFactory(t, stack, &fakeTokenIssuer{token: "t"}).For(context.Background(), "stk_1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub")
}

func TestFor_PropagatesTokenFailure(t *testing.T) {
	issuer := &fakeTokenIssuer{err: errors.New("toolbox unavailable")}
	_, err := newFactory(t, gitlabStack(), issuer).For(context.Background(), "stk_1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "toolbox unavailable")
}

// 스택이 아직 설치 중이면 GitLab 이 준비되지 않았을 수 있다.
func TestFor_FailsWhenStackNotCompleted(t *testing.T) {
	stack := gitlabStack()
	stack.State = "installing"

	_, err := newFactory(t, stack, &fakeTokenIssuer{token: "t"}).For(context.Background(), "stk_1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "installing")
}

// 클러스터 내부 DNS 는 API 서버가 클러스터 안에서 돌 때만 해석된다.
// 로컬 실행이나 외부 GitLab 을 위해 override 가 우선해야 한다.
func TestFor_HonorsGitLabBaseURLOverride(t *testing.T) {
	custom := stubGitLab(t)
	f := NewBundleFactory(
		&fakeStackReader{summary: gitlabStack()},
		&fakeTokenIssuer{token: "t"},
		Options{Env: "dev", GroupPath: "acme", GitLabBaseURLOverride: custom},
	)

	bundle, err := f.For(context.Background(), "stk_1")
	require.NoError(t, err)

	client, ok := bundle.Provisioner.(interface{ BaseURL() string })
	require.True(t, ok, "클라이언트가 주소를 노출해야 검증할 수 있다")
	assert.Equal(t, custom, client.BaseURL())
}

// 보관된 토큰은 폐기·만료될 수 있다. 그대로 쓰면 이후 모든 호출이 401 로 죽고
// 복구 경로가 없으므로, 인증에 실패하면 강제 재발급해야 한다.
type reissuingTokenIssuer struct {
	stale, fresh string
	specs        []port.SCMTokenSpec
}

func (r *reissuingTokenIssuer) EnsureToken(_ context.Context, spec port.SCMTokenSpec) (string, error) {
	r.specs = append(r.specs, spec)
	if spec.Force {
		return r.fresh, nil
	}
	return r.stale, nil
}

func TestFor_ReissuesTokenWhenStoredOneIsRevoked(t *testing.T) {
	issuer := &reissuingTokenIssuer{stale: "revoked", fresh: "valid"}
	f := NewBundleFactory(
		&fakeStackReader{summary: gitlabStack()},
		issuer,
		Options{Env: "dev", GroupPath: "acme", GitLabBaseURLOverride: stubGitLab(t, "valid")},
	)

	bundle, err := f.For(context.Background(), "stk_1")
	require.NoError(t, err)
	require.NotNil(t, bundle)

	require.Len(t, issuer.specs, 2, "폐기된 토큰이면 재발급을 시도해야 한다")
	assert.False(t, issuer.specs[0].Force)
	assert.True(t, issuer.specs[1].Force)
}

// 재발급해도 인증되지 않으면 조용히 넘어가지 않는다.
func TestFor_FailsWhenReissuedTokenStillUnauthorized(t *testing.T) {
	issuer := &reissuingTokenIssuer{stale: "bad", fresh: "also-bad"}
	f := NewBundleFactory(
		&fakeStackReader{summary: gitlabStack()},
		issuer,
		Options{Env: "dev", GroupPath: "acme", GitLabBaseURLOverride: stubGitLab(t, "only-this")},
	)

	_, err := f.For(context.Background(), "stk_1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "인증 실패")
}

func TestGitLabBaseURL_UsesInClusterService(t *testing.T) {
	assert.Equal(t,
		"http://gitlab-webservice-default.devsecops.svc:8181",
		gitLabBaseURL("devsecops"))
}

func TestRegistryHostFor_DerivesFromAccessDomain(t *testing.T) {
	assert.Equal(t, "registry.nullus.local", registryHostFor("nullus.local"))
	// 도메인을 모르면 빈 값 — 클라이언트가 API 응답의 경로를 그대로 쓴다.
	assert.Equal(t, "", registryHostFor(""))
}
