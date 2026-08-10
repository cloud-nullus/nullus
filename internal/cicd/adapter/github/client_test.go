package github

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

// newStubGitHub 은 요청을 기록하고 경로별 응답을 돌려주는 테스트 서버다.
func newStubGitHub(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	recorded := make([]recordedRequest, 0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		recorded = append(recorded, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		r.Body = io.NopCloser(bytes.NewReader(raw))
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &recorded
}

func TestClient_ImplementsPorts(t *testing.T) {
	var _ port.SCMProvisioner = (*Client)(nil)
	var _ port.PipelineConfigurator = (*Client)(nil)
}

func TestEnsureGroup_ReturnsExistingOrg(t *testing.T) {
	srv, recorded := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/orgs/acme" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"login": "acme", "name": "Acme Inc", "html_url": "https://github.com/acme",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	c := NewClient(srv.URL, "tok")
	group, err := c.EnsureGroup(context.Background(), port.GroupSpec{Path: "acme"})

	require.NoError(t, err)
	assert.Equal(t, "acme", group.ID)
	assert.Equal(t, "acme", group.FullPath)

	for _, req := range *recorded {
		assert.NotEqual(t, http.MethodPost, req.Method, "org 은 API 로 만들지 않는다")
	}
}

// GitHub org 는 API 로 만들 수 없다. 조용히 개인 네임스페이스로 흘리면
// 엉뚱한 곳에 리포가 생기므로, 사용자 계정이면 그 사실을 명확히 알려야 한다.
func TestEnsureGroup_FallsBackToUserAccount(t *testing.T) {
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/devlos":
			w.WriteHeader(http.StatusNotFound)
		case "/users/devlos":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"login": "devlos", "name": "devlos", "html_url": "https://github.com/devlos",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	c := NewClient(srv.URL, "tok")
	group, err := c.EnsureGroup(context.Background(), port.GroupSpec{Path: "devlos"})

	require.NoError(t, err)
	assert.Equal(t, "devlos", group.FullPath)
}

func TestEnsureGroup_ErrorsWhenOwnerMissing(t *testing.T) {
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	c := NewClient(srv.URL, "tok")
	_, err := c.EnsureGroup(context.Background(), port.GroupSpec{Path: "nope"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func TestEnsureProject_ReturnsExistingWithoutCreating(t *testing.T) {
	srv, recorded := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/myapp" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"full_name": "acme/myapp", "name": "myapp",
				"html_url":       "https://github.com/acme/myapp",
				"clone_url":      "https://github.com/acme/myapp.git",
				"default_branch": "main",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	c := NewClient(srv.URL, "tok")
	project, err := c.EnsureProject(context.Background(), port.ProjectSpec{
		Path: "myapp", GroupPath: "acme",
	})

	require.NoError(t, err)
	assert.Equal(t, "acme/myapp", project.ID)
	assert.Equal(t, "acme/myapp", project.FullPath)
	assert.Equal(t, "https://github.com/acme/myapp.git", project.HTTPCloneURL)
	assert.Equal(t, "main", project.DefaultBranch)

	for _, req := range *recorded {
		assert.NotEqual(t, http.MethodPost, req.Method, "이미 있으면 생성 요청을 보내면 안 된다")
	}
}

func TestEnsureProject_CreatesUnderOrg(t *testing.T) {
	srv, recorded := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/myapp":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/orgs/acme":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "acme"})
		case r.Method == http.MethodPost && r.URL.Path == "/orgs/acme/repos":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"full_name": "acme/myapp", "name": "myapp",
				"clone_url":      "https://github.com/acme/myapp.git",
				"default_branch": "main",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	c := NewClient(srv.URL, "tok")
	project, err := c.EnsureProject(context.Background(), port.ProjectSpec{
		Path: "myapp", GroupPath: "acme", InitReadme: true, Visibility: "private",
	})

	require.NoError(t, err)
	assert.Equal(t, "acme/myapp", project.ID)

	var create *recordedRequest
	for i := range *recorded {
		if (*recorded)[i].Method == http.MethodPost {
			create = &(*recorded)[i]
		}
	}
	require.NotNil(t, create, "생성 요청이 있어야 한다")
	assert.Equal(t, "/orgs/acme/repos", create.Path)
	assert.Equal(t, true, create.Body["auto_init"], "브랜치가 없으면 스캐폴딩 커밋이 실패한다")
	assert.Equal(t, true, create.Body["private"])
}

// org 가 아니라 개인 계정이면 POST /user/repos 로 가야 한다.
// /orgs/{owner}/repos 로 보내면 404 가 난다.
func TestEnsureProject_CreatesUnderUserAccount(t *testing.T) {
	srv, recorded := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/devlos/myapp", r.URL.Path == "/orgs/devlos":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/user/repos":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"full_name": "devlos/myapp", "name": "myapp",
				"clone_url":      "https://github.com/devlos/myapp.git",
				"default_branch": "main",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	c := NewClient(srv.URL, "tok")
	project, err := c.EnsureProject(context.Background(), port.ProjectSpec{
		Path: "myapp", GroupPath: "devlos", InitReadme: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "devlos/myapp", project.FullPath)

	var paths []string
	for _, req := range *recorded {
		if req.Method == http.MethodPost {
			paths = append(paths, req.Path)
		}
	}
	assert.Equal(t, []string{"/user/repos"}, paths)
}

// GHCR 경로는 소문자만 받는다. 대문자가 섞인 org/리포 이름을 그대로 쓰면
// docker push 가 거부된다.
func TestEnsureProject_RegistryURLIsLowercased(t *testing.T) {
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/Acme/MyApp" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"full_name": "Acme/MyApp", "name": "MyApp",
				"clone_url":      "https://github.com/Acme/MyApp.git",
				"default_branch": "main",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	c := NewClient(srv.URL, "tok")
	project, err := c.EnsureProject(context.Background(), port.ProjectSpec{
		Path: "MyApp", GroupPath: "Acme",
	})

	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/acme/myapp", project.RegistryURL)
}

// 파일 여러 개를 한 커밋으로 올려야 한다. Contents API 를 파일마다 부르면
// 커밋이 N 개 생기고 Argo CD 가 중간 상태를 동기화한다.
func TestCommitFiles_CreatesSingleCommitViaGitDataAPI(t *testing.T) {
	srv, recorded := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/myapp/git/ref/heads/main":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": map[string]any{"sha": "base-commit-sha"},
			})
		case r.URL.Path == "/repos/acme/myapp/git/commits/base-commit-sha":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha": "base-commit-sha", "tree": map[string]any{"sha": "base-tree-sha"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/myapp/git/trees":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-tree-sha"})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/myapp/git/commits":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-commit-sha"})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/acme/myapp/git/refs/heads/main":
			_ = json.NewEncoder(w).Encode(map[string]any{"ref": "refs/heads/main"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	c := NewClient(srv.URL, "tok")
	err := c.CommitFiles(context.Background(), "acme/myapp", port.CommitSpec{
		Branch:  "main",
		Message: "chore(nullus): scaffold",
		Files: []port.CommitFile{
			{Path: ".github/workflows/nullus-ci.yml", Content: "name: ci"},
			{Path: "deploy/deployment.yaml", Content: "kind: Deployment"},
		},
	})
	require.NoError(t, err)

	var tree, commit, patch *recordedRequest
	for i := range *recorded {
		switch req := &(*recorded)[i]; {
		case req.Path == "/repos/acme/myapp/git/trees":
			tree = req
		case req.Path == "/repos/acme/myapp/git/commits" && req.Method == http.MethodPost:
			commit = req
		case req.Method == http.MethodPatch:
			patch = req
		}
	}

	require.NotNil(t, tree)
	assert.Equal(t, "base-tree-sha", tree.Body["base_tree"],
		"base_tree 가 없으면 기존 파일이 전부 지워진다")
	entries, ok := tree.Body["tree"].([]any)
	require.True(t, ok)
	assert.Len(t, entries, 2, "파일 두 개가 한 트리에 들어가야 한다")

	require.NotNil(t, commit)
	assert.Equal(t, "chore(nullus): scaffold", commit.Body["message"])
	assert.Equal(t, "new-tree-sha", commit.Body["tree"])

	require.NotNil(t, patch)
	assert.Equal(t, "new-commit-sha", patch.Body["sha"])
}

func TestCommitFiles_NoFilesIsNoop(t *testing.T) {
	srv, recorded := newStubGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	c := NewClient(srv.URL, "tok")
	require.NoError(t, c.CommitFiles(context.Background(), "acme/myapp", port.CommitSpec{}))
	assert.Empty(t, *recorded)
}

func TestPing_FailsOnUnauthorized(t *testing.T) {
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	c := NewClient(srv.URL, "bad")
	require.Error(t, c.Ping(context.Background()))
}

func TestClient_SendsBearerAuthAndAPIVersion(t *testing.T) {
	var gotAuth, gotVersion string
	srv, _ := newStubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("X-GitHub-Api-Version")
		_ = json.NewEncoder(w).Encode(map[string]any{"login": "devlos"})
	})

	c := NewClient(srv.URL, "tok")
	require.NoError(t, c.Ping(context.Background()))
	assert.Equal(t, "Bearer tok", gotAuth)
	assert.Equal(t, apiVersion, gotVersion)
}
