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

type fakeConnectionReader struct {
	conn      *port.SCMConnection
	err       error
	platforms []port.SCMPlatform
}

func (f *fakeConnectionReader) GetConnection(
	_ context.Context,
	_ string,
	platform port.SCMPlatform,
) (*port.SCMConnection, error) {
	f.platforms = append(f.platforms, platform)
	return f.conn, f.err
}

func githubStack() *port.StackSummary {
	return &port.StackSummary{
		ID: "stk_gh", OrgID: "org-1", ClusterID: "c1", State: "completed",
		Namespace:        "devsecops",
		SourceRepository: "GitHub", ContainerRegistry: "GHCR",
		AccessDomain: "nullus.local",
	}
}

// stubGitHubAPI 는 인증 확인(GET /user)에 응답하는 최소 서버다.
func stubGitHubAPI(t *testing.T, acceptToken string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if acceptToken != "" && r.Header.Get("Authorization") != "Bearer "+acceptToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"login": "acme-bot"})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFor_BuildsGitHubBundle(t *testing.T) {
	apiURL := stubGitHubAPI(t, "ghp_valid")
	tokens := &fakeTokenIssuer{token: "ghp_valid"}
	conns := &fakeConnectionReader{conn: &port.SCMConnection{
		Platform: port.SCMPlatformGitHub, Owner: "acme", APIBaseURL: apiURL,
	}}

	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{Env: "dev"}).
		WithGitHub(tokens, conns)

	bundle, err := f.For(context.Background(), "stk_gh")
	require.NoError(t, err)

	assert.Equal(t, port.SCMPlatformGitHub, bundle.Platform)
	// 리포는 GitHub org 아래에 생긴다. nullus 그룹 경로를 쓰면 없는 네임스페이스를 가리킨다.
	assert.Equal(t, "acme", bundle.GroupPath)
	// Argo CD 는 여전히 클러스터 안에 있다.
	assert.Equal(t, "devsecops", bundle.CDNamespace)
	assert.Equal(t, "c1", bundle.ClusterID)
	// GitHub 은 리포 범위 토큰 API 가 없어 조직 PAT 를 재사용한다.
	assert.Equal(t, "ghp_valid", bundle.RepoAccessToken)
	assert.Equal(t, []port.SCMPlatform{port.SCMPlatformGitHub}, conns.platforms)
}

func TestFor_GitHubBundleResolvesGHCRTarget(t *testing.T) {
	apiURL := stubGitHubAPI(t, "")
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{}).
		WithGitHub(&fakeTokenIssuer{token: "ghp"}, &fakeConnectionReader{
			conn: &port.SCMConnection{Owner: "acme", APIBaseURL: apiURL},
		})

	bundle, err := f.For(context.Background(), "stk_gh")
	require.NoError(t, err)

	target, err := bundle.Registry.Resolve(context.Background(), port.ImageTargetSpec{AppName: "myapp"})
	require.NoError(t, err)
	assert.Equal(t, port.RegistryKindGHCR, target.Kind)
	assert.Equal(t, "ghcr.io/acme/myapp", target.Repository)
	assert.Empty(t, target.RequiredVariables, "GHCR 은 사용자 시크릿을 요구하지 않는다")
}

// GitLab 스택은 예전 동작을 그대로 유지해야 한다.
func TestFor_GitLabStackStillGetsGitLabPlatform(t *testing.T) {
	f := NewBundleFactory(
		&fakeStackReader{summary: gitlabStack()},
		&fakeTokenIssuer{token: "glpat"},
		Options{GitLabBaseURLOverride: stubGitLab(t)},
	)

	bundle, err := f.For(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, port.SCMPlatformGitLab, bundle.Platform)
	assert.Equal(t, "nullus", bundle.GroupPath)
	// GitLab 은 프로젝트마다 최소 권한 토큰을 따로 발급한다.
	assert.Empty(t, bundle.RepoAccessToken)
}

// 배선이 없는데 조용히 GitLab 으로 흘리면 엉뚱한 곳에 리포가 생긴다.
func TestFor_GitHubStackFailsWhenNotWired(t *testing.T) {
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{})

	_, err := f.For(context.Background(), "stk_gh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 연동이 배선되지 않아")
}

func TestFor_GitHubStackFailsWithoutConnection(t *testing.T) {
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{}).
		WithGitHub(&fakeTokenIssuer{token: "ghp"}, &fakeConnectionReader{conn: nil})

	_, err := f.For(context.Background(), "stk_gh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 연동 설정이 없습니다")
}

func TestFor_GitHubStackFailsWhenOwnerBlank(t *testing.T) {
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{}).
		WithGitHub(&fakeTokenIssuer{token: "ghp"}, &fakeConnectionReader{
			conn: &port.SCMConnection{Owner: "   "},
		})

	_, err := f.For(context.Background(), "stk_gh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 연동 설정이 없습니다")
}

// GitHub 에는 재발급 경로가 없다. 만료된 PAT 로 계속 가면 이후 모든 호출이
// 401 로 죽으므로, 여기서 끊고 다시 등록하라고 알려야 한다.
func TestFor_GitHubStackFailsOnExpiredToken(t *testing.T) {
	apiURL := stubGitHubAPI(t, "ghp_valid")
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{}).
		WithGitHub(&fakeTokenIssuer{token: "ghp_revoked"}, &fakeConnectionReader{
			conn: &port.SCMConnection{Owner: "acme", APIBaseURL: apiURL},
		})

	_, err := f.For(context.Background(), "stk_gh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "만료·폐기")
}

func TestFor_GitHubStackPropagatesTokenError(t *testing.T) {
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{}).
		WithGitHub(&fakeTokenIssuer{err: errors.New("no PAT registered")}, &fakeConnectionReader{
			conn: &port.SCMConnection{Owner: "acme"},
		})

	_, err := f.For(context.Background(), "stk_gh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no PAT registered")
}

// 알 수 없는 도구를 어느 쪽으로든 추측하면 엉뚱한 주소로 호출한다.
func TestFor_UnknownSourceRepositoryIsRejected(t *testing.T) {
	summary := githubStack()
	// Gitea 는 이제 지원되므로 "알 수 없는 도구" 예시로 쓸 수 없다.
	summary.SourceRepository = "Bitbucket"

	f := NewBundleFactory(&fakeStackReader{summary: summary}, &fakeTokenIssuer{}, Options{}).
		WithGitHub(&fakeTokenIssuer{token: "ghp"}, &fakeConnectionReader{
			conn: &port.SCMConnection{Owner: "acme"},
		})

	_, err := f.For(context.Background(), "stk_gh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bitbucket")
}

func TestPlatformFor_NormalizesToolNames(t *testing.T) {
	cases := map[string]port.SCMPlatform{
		"GitLab CE":                port.SCMPlatformGitLab,
		"gitlab_ee":                port.SCMPlatformGitLab,
		"GitHub":                   port.SCMPlatformGitHub,
		"github enterprise server": port.SCMPlatformGitHub,
		"Gitea":                    port.SCMPlatformGitea,
		"Bitbucket":                "",
		"":                         "",
	}
	for name, want := range cases {
		assert.Equal(t, want, platformFor(name), "tool=%q", name)
	}
}
