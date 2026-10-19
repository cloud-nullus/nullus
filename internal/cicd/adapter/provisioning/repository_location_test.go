package provisioning

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 앱 배포 화면은 스택 모드에서 저장소 경로를 입력받았지만 서버는 그 값을 쓰지 않았다 — 저장소는 언제나
// <그룹>/<앱 이름> 으로 만들어진다. 화면이 실제로 만들어질 주소를 보여 주려면 그룹과 웹 주소가 필요하다.
// 토큰을 발급하거나 SCM 에 붙지 않는다 — 입력 화면을 여는 것만으로 OpenBao · GitLab 을 두드리지 않게.
func TestRepositoryLocation_GitLab(t *testing.T) {
	issuer := &fakeTokenIssuer{token: "glpat-x"}
	f := NewBundleFactory(&fakeStackReader{summary: gitlabStack()}, issuer, Options{Env: "dev", GroupPath: "acme"})

	loc, err := f.RepositoryLocation(context.Background(), "stk_1")
	require.NoError(t, err)

	assert.Equal(t, &port.RepositoryLocation{
		Platform: port.SCMPlatformGitLab, GroupPath: "acme", WebBaseURL: "https://gitlab.nullus.local",
	}, loc)
	assert.Empty(t, issuer.specs, "토큰을 발급하지 않는다")
}

// GitHub 은 조직(연결의 Owner) 아래에 리포가 생긴다 — 서버의 그룹 설정을 쓰면 없는 자리를 가리킨다.
func TestRepositoryLocation_GitHub(t *testing.T) {
	conns := &fakeConnectionReader{conn: &port.SCMConnection{Platform: port.SCMPlatformGitHub, Owner: "acme-org"}}
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{Env: "dev", GroupPath: "nullus"}).
		WithGitHub(&fakeTokenIssuer{}, conns)

	loc, err := f.RepositoryLocation(context.Background(), "stk_gh")
	require.NoError(t, err)

	assert.Equal(t, port.SCMPlatformGitHub, loc.Platform)
	assert.Equal(t, "acme-org", loc.GroupPath)
	assert.Equal(t, "https://github.com", loc.WebBaseURL)
}

func TestRepositoryLocation_Gitea(t *testing.T) {
	stack := gitlabStack()
	stack.SourceRepository = "Gitea"
	f := NewBundleFactory(&fakeStackReader{summary: stack}, &fakeTokenIssuer{}, Options{Env: "dev", GroupPath: "acme"})

	loc, err := f.RepositoryLocation(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Equal(t, &port.RepositoryLocation{
		Platform: port.SCMPlatformGitea, GroupPath: "acme", WebBaseURL: "https://gitea.nullus.local",
	}, loc)
}

func TestRepositoryLocation_MissingStack(t *testing.T) {
	f := NewBundleFactory(&fakeStackReader{summary: nil}, &fakeTokenIssuer{}, Options{GroupPath: "acme"})
	_, err := f.RepositoryLocation(context.Background(), "stk_missing")
	assert.True(t, errors.Is(err, port.ErrStackToolsUnavailable), "got %v", err)
}

func TestRepositoryLocation_GitHubWithoutConnection(t *testing.T) {
	conns := &fakeConnectionReader{conn: nil}
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{Env: "dev"}).
		WithGitHub(&fakeTokenIssuer{}, conns)
	_, err := f.RepositoryLocation(context.Background(), "stk_gh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub 연동")
}
