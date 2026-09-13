package provisioning

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// pathRecorder 는 받은 요청 경로를 기록하고, 목록 API 에는 빈 배열로 답하는 서버다.
type pathRecorder struct {
	mu    sync.Mutex
	paths []string
}

func (p *pathRecorder) server(t *testing.T, listBody any) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.paths = append(p.paths, r.URL.EscapedPath())
		p.mu.Unlock()
		switch r.URL.Path {
		case "/api/v4/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"username": "root"})
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "acme-bot"})
		default:
			_ = json.NewEncoder(w).Encode(listBody)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (p *pathRecorder) has(path string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, got := range p.paths {
		if got == path {
			return true
		}
	}
	return false
}

// GitLab CI 스택도 실행 기록과 스캔 리포트를 읽을 수 있어야 한다. 비어 있으면
// ForPipeline 이 조용히 건너뛰어, 스캔 게이트가 돌아도 판정이 기록되지 않는다.
func TestFor_GitLabBundleReadsBuildsFromGroupProject(t *testing.T) {
	rec := &pathRecorder{}
	f := NewBundleFactory(&fakeStackReader{summary: gitlabStack()}, &fakeTokenIssuer{token: "t"}, Options{
		Env: "dev", GroupPath: "acme", GitLabBaseURLOverride: rec.server(t, []any{}),
	})

	bundle, err := f.For(context.Background(), "stk_1")
	require.NoError(t, err)
	require.NotNil(t, bundle.CIBuilds, "GitLab 스택에 실행 기록 경로가 없다")
	assert.NotNil(t, bundle.CIArtifacts, "GitLab 스택에 스캔 리포트 경로가 없다")

	_, err = bundle.CIBuilds.ListBuilds(context.Background(), "shop", "main", 1)
	require.NoError(t, err)
	assert.True(t, rec.has("/api/v4/projects/acme%2Fshop/pipelines"),
		"파이프라인은 스택 그룹 아래 앱 프로젝트에서 돈다")
}

func TestFor_GitHubBundleReadsBuildsFromOwnerRepo(t *testing.T) {
	rec := &pathRecorder{}
	conns := &fakeConnectionReader{conn: &port.SCMConnection{
		Platform: port.SCMPlatformGitHub, Owner: "acme",
		APIBaseURL: rec.server(t, map[string]any{"workflow_runs": []any{}}),
	}}
	f := NewBundleFactory(&fakeStackReader{summary: githubStack()}, &fakeTokenIssuer{}, Options{Env: "dev"}).
		WithGitHub(&fakeTokenIssuer{token: "ghp"}, conns)

	bundle, err := f.For(context.Background(), "stk_gh")
	require.NoError(t, err)
	require.NotNil(t, bundle.CIBuilds, "GitHub 스택에 실행 기록 경로가 없다")
	assert.NotNil(t, bundle.CIArtifacts, "GitHub 스택에 스캔 리포트 경로가 없다")

	_, err = bundle.CIBuilds.ListBuilds(context.Background(), "shop", "main", 1)
	require.NoError(t, err)
	assert.True(t, rec.has("/repos/acme/shop/actions/runs"),
		"리포는 연동에 등록된 organization 아래에 있다")
}
