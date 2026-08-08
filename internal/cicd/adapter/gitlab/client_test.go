package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
}

// newStubGitLab 은 요청을 기록하고 경로별 응답을 돌려주는 테스트 서버다.
func newStubGitLab(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body map[string]any)) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	recorded := make([]recordedRequest, 0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		// GitLab 은 프로젝트 경로를 %2F 로 인코딩해 받는다. r.URL.Path 는 디코드된
		// 값이라 매칭에 쓸 수 없으므로 EscapedPath 를 기록한다.
		recorded = append(recorded, recordedRequest{
			Method: r.Method, Path: r.URL.EscapedPath(), Query: r.URL.RawQuery, Body: body,
		})
		handler(w, r, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &recorded
}

func TestClient_ImplementsSCMProvisioner(t *testing.T) {
	var _ port.SCMProvisioner = (*Client)(nil)
}

func TestEnsureGroup_ReturnsExistingWithoutCreating(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/acme" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 7, "name": "acme", "full_path": "acme", "web_url": "https://gl.test/acme",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	c := NewClient(srv.URL, "tok")
	group, err := c.EnsureGroup(context.Background(), port.GroupSpec{Name: "acme", Path: "acme"})

	require.NoError(t, err)
	assert.Equal(t, "7", group.ID)
	assert.Equal(t, "acme", group.FullPath)

	for _, req := range *recorded {
		assert.NotEqual(t, http.MethodPost, req.Method, "이미 있으면 생성 요청을 보내면 안 된다")
	}
}

func TestEnsureGroup_CreatesWhenMissing(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 9, "name": "acme", "full_path": "acme", "web_url": "https://gl.test/acme",
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	c := NewClient(srv.URL, "tok")
	group, err := c.EnsureGroup(context.Background(), port.GroupSpec{Name: "acme", Path: "acme"})

	require.NoError(t, err)
	assert.Equal(t, "9", group.ID)

	var created *recordedRequest
	for i := range *recorded {
		if (*recorded)[i].Method == http.MethodPost {
			created = &(*recorded)[i]
		}
	}
	require.NotNil(t, created)
	assert.Equal(t, "acme", created.Body["path"])
}

func TestEnsureProject_ReturnsExistingAndExposesRegistryURL(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/projects/acme%2Fcommon" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 42, "name": "common", "path_with_namespace": "acme/common",
				"web_url": "https://gl.test/acme/common", "default_branch": "main",
				"http_url_to_repo":       "https://gl.test/acme/common.git",
				"container_registry_url": "registry.gl.test/acme/common",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	c := NewClient(srv.URL, "tok")
	p, err := c.EnsureProject(context.Background(), port.ProjectSpec{
		Name: "common", Path: "common", GroupID: "7", GroupPath: "acme",
	})

	require.NoError(t, err)
	assert.Equal(t, "42", p.ID)
	assert.Equal(t, "acme/common", p.FullPath)
	assert.Equal(t, "registry.gl.test/acme/common", p.RegistryURL)
	assert.Equal(t, "main", p.DefaultBranch)
	assert.Equal(t, "https://gl.test/acme/common.git", p.HTTPCloneURL)
}

// GitLab 응답에 container_registry_url 이 없는 버전이 있다. 그 경우
// 레지스트리 경로를 알 수 없으면 CI 가 이미지를 어디에 올릴지 정할 수 없으므로
// 프로젝트 경로로 폴백한다.
func TestEnsureProject_FallsBackToDerivedRegistryURL(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 42, "name": "common", "path_with_namespace": "acme/common",
				"web_url": "https://gl.test/acme/common", "default_branch": "main",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	c := NewClient(srv.URL, "tok").WithRegistryHost("registry.gl.test")
	p, err := c.EnsureProject(context.Background(), port.ProjectSpec{Name: "common", Path: "common"})

	require.NoError(t, err)
	assert.Equal(t, "registry.gl.test/acme/common", p.RegistryURL)
}

func TestEnsureProject_CreatesWithGroupAndReadme(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 51, "name": "myapp", "path_with_namespace": "acme/myapp",
				"default_branch": "main",
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	c := NewClient(srv.URL, "tok")
	_, err := c.EnsureProject(context.Background(), port.ProjectSpec{
		Name: "myapp", Path: "myapp", GroupID: "7", InitReadme: true, Visibility: "private",
	})
	require.NoError(t, err)

	var created *recordedRequest
	for i := range *recorded {
		if (*recorded)[i].Method == http.MethodPost {
			created = &(*recorded)[i]
		}
	}
	require.NotNil(t, created)
	assert.Equal(t, float64(7), created.Body["namespace_id"])
	assert.Equal(t, true, created.Body["initialize_with_readme"])
	assert.Equal(t, "private", created.Body["visibility"])
}

func TestCommitFiles_UsesCreateActionsFirst(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc123"})
	})

	c := NewClient(srv.URL, "tok")
	err := c.CommitFiles(context.Background(), "51", port.CommitSpec{
		Branch:  "main",
		Message: "chore: scaffold",
		Files: []port.CommitFile{
			{Path: ".gitlab-ci.yml", Content: "stages: [build]"},
			{Path: "deploy/deployment.yaml", Content: "kind: Deployment"},
		},
	})
	require.NoError(t, err)

	// 트리 조회(GET) 후 커밋(POST) 순이다.
	var req *recordedRequest
	for i := range *recorded {
		if (*recorded)[i].Method == http.MethodPost {
			req = &(*recorded)[i]
		}
	}
	require.NotNil(t, req)

	assert.Equal(t, "/api/v4/projects/51/repository/commits", req.Path)
	actions, ok := req.Body["actions"].([]any)
	require.True(t, ok)
	require.Len(t, actions, 2)

	first := actions[0].(map[string]any)
	assert.Equal(t, "create", first["action"])
	assert.Equal(t, ".gitlab-ci.yml", first["file_path"])
}

// 저장소에 이미 있는 파일과 없는 파일이 섞여 있는 것이 정상이다
// (initialize_with_readme 로 README.md 만 있는 새 프로젝트).
// 커밋 전체에 하나의 action 을 쓰면 어느 쪽으로 보내도 실패한다 —
// create 는 "already exists", update 는 "doesn't exist" 로 죽는다.
func TestCommitFiles_ChoosesActionPerFile(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.EscapedPath(), "/repository/tree") {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"path": "README.md", "type": "blob"},
				{"path": "deploy", "type": "tree"},
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc"})
	})

	c := NewClient(srv.URL, "tok")
	err := c.CommitFiles(context.Background(), "51", port.CommitSpec{
		Branch: "main", Message: "chore: scaffold",
		Files: []port.CommitFile{
			{Path: "README.md", Content: "updated"},
			{Path: ".gitlab-ci.yml", Content: "stages: [build]"},
		},
	})
	require.NoError(t, err)

	var commit *recordedRequest
	for i := range *recorded {
		if (*recorded)[i].Method == http.MethodPost {
			commit = &(*recorded)[i]
		}
	}
	require.NotNil(t, commit)

	actions := commit.Body["actions"].([]any)
	byPath := map[string]string{}
	for _, a := range actions {
		m := a.(map[string]any)
		byPath[m["file_path"].(string)] = m["action"].(string)
	}
	assert.Equal(t, "update", byPath["README.md"], "이미 있는 파일은 update")
	assert.Equal(t, "create", byPath[".gitlab-ci.yml"], "없는 파일은 create")
}

// 트리 조회가 실패해도(빈 저장소 등) 커밋은 시도해야 한다.
func TestCommitFiles_FallsBackToCreateWhenTreeUnavailable(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc"})
	})

	c := NewClient(srv.URL, "tok")
	err := c.CommitFiles(context.Background(), "51", port.CommitSpec{
		Branch: "main", Message: "m",
		Files: []port.CommitFile{{Path: "a.txt", Content: "b"}},
	})
	require.NoError(t, err)

	var commit *recordedRequest
	for i := range *recorded {
		if (*recorded)[i].Method == http.MethodPost {
			commit = &(*recorded)[i]
		}
	}
	require.NotNil(t, commit)
	actions := commit.Body["actions"].([]any)
	assert.Equal(t, "create", actions[0].(map[string]any)["action"])
}

func TestCommitFiles_PropagatesNonExistenceErrors(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"message":"403 Forbidden"}`)
	})

	c := NewClient(srv.URL, "tok")
	err := c.CommitFiles(context.Background(), "51", port.CommitSpec{
		Branch: "main", Message: "m", Files: []port.CommitFile{{Path: "a", Content: "b"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestClient_SendsPrivateTokenHeader(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("PRIVATE-TOKEN")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "full_path": "x"})
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "super-secret")
	_, err := c.EnsureGroup(context.Background(), port.GroupSpec{Name: "x", Path: "x"})
	require.NoError(t, err)
	assert.Equal(t, "super-secret", gotToken)
}
