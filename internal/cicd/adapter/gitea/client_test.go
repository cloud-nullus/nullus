package gitea

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestEnsureGroup_ReturnsExistingWithoutCreating(t *testing.T) {
	var createCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/nullus":
			writeJSON(t, w, map[string]any{"username": "nullus", "full_name": "Nullus"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs":
			createCalls++
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("예상치 못한 요청: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	group, err := NewClient(srv.URL, "tok").EnsureGroup(context.Background(), port.GroupSpec{Path: "nullus"})

	require.NoError(t, err)
	assert.Equal(t, "nullus", group.FullPath)
	assert.Zero(t, createCalls, "이미 있는 조직을 다시 만들면 안 된다 (Ensure* 는 멱등)")
}

// GitHub 과 다른 지점이다 — Gitea 는 조직을 API 로 만들 수 있다.
func TestEnsureGroup_CreatesWhenMissing(t *testing.T) {
	var createdBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/nullus":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&createdBody))
			writeJSON(t, w, map[string]any{"username": "nullus"})
		default:
			t.Fatalf("예상치 못한 요청: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	group, err := NewClient(srv.URL, "tok").EnsureGroup(context.Background(), port.GroupSpec{Path: "nullus"})

	require.NoError(t, err)
	assert.Equal(t, "nullus", group.FullPath)
	assert.Equal(t, "private", createdBody["visibility"],
		"스캐폴딩에 배포 매니페스트가 들어가므로 public 이면 클러스터 구성이 노출된다")
}

func TestEnsureProject_CreatesAndMarksCreated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/nullus/api":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs/nullus/repos":
			writeJSON(t, w, map[string]any{
				"full_name": "nullus/api", "name": "api",
				"clone_url": srv2CloneURL, "default_branch": "main",
			})
		default:
			t.Fatalf("예상치 못한 요청: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	project, err := NewClient(srv.URL, "tok").EnsureProject(context.Background(), port.ProjectSpec{
		Path: "api", GroupPath: "nullus", InitReadme: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "nullus/api", project.ID)
	assert.True(t, project.Created, "새로 만든 리포는 Created 로 표시돼야 스캐폴딩이 덮어쓰기를 판단할 수 있다")
}

func TestEnsureProject_ExistingIsNotMarkedCreated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method, "이미 있는 리포에 POST 를 보내면 안 된다")
		writeJSON(t, w, map[string]any{"full_name": "nullus/api", "name": "api", "default_branch": "main"})
	}))
	defer srv.Close()

	project, err := NewClient(srv.URL, "tok").EnsureProject(context.Background(), port.ProjectSpec{
		Path: "api", GroupPath: "nullus",
	})

	require.NoError(t, err)
	assert.False(t, project.Created,
		"기존 리포를 Created 로 표시하면 CI 가 갱신한 이미지 태그가 스캐폴딩으로 되돌아간다")
}

// 스캐폴딩이 여러 커밋으로 쪼개지면 Argo CD 가 중간 상태를 동기화한다.
func TestCommitFiles_SendsAllFilesInOneRequest(t *testing.T) {
	var body map[string]any
	var commitCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			// 새 리포라 어떤 파일도 아직 없다.
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/nullus/api/contents":
			commitCalls++
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("예상치 못한 요청: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "tok").CommitFiles(context.Background(), "nullus/api", port.CommitSpec{
		Branch:  "main",
		Message: "chore: scaffold",
		Files: []port.CommitFile{
			{Path: "Jenkinsfile", Content: "pipeline {}"},
			{Path: "deploy/deployment.yaml", Content: "kind: Deployment"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, commitCalls, "파일마다 커밋하면 Argo CD 가 중간 상태를 배포한다")

	files, ok := body["files"].([]any)
	require.True(t, ok)
	require.Len(t, files, 2)

	first := files[0].(map[string]any)
	assert.Equal(t, "create", first["operation"])
	decoded, err := base64.StdEncoding.DecodeString(first["content"].(string))
	require.NoError(t, err, "Gitea contents API 는 base64 를 요구한다 — 평문을 보내면 파일이 깨진다")
	assert.Equal(t, "pipeline {}", string(decoded))
}

// 이미 있는 파일은 update + sha 로 보내야 한다. create 로 보내면 409 가 난다.
func TestCommitFiles_ExistingFileUsesUpdateWithSHA(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/nullus/api/contents/Jenkinsfile":
			writeJSON(t, w, map[string]any{"sha": "abc123"})
		case r.Method == http.MethodPost:
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("예상치 못한 요청: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "tok").CommitFiles(context.Background(), "nullus/api", port.CommitSpec{
		Branch: "main", Message: "update",
		Files: []port.CommitFile{{Path: "Jenkinsfile", Content: "pipeline { agent any }"}},
	})

	require.NoError(t, err)
	first := body["files"].([]any)[0].(map[string]any)
	assert.Equal(t, "update", first["operation"])
	assert.Equal(t, "abc123", first["sha"])
}

// 중첩 경로를 통째로 이스케이프하면 %2F 가 되어 404 가 난다.
func TestCommitFiles_NestedPathKeepsSlashes(t *testing.T) {
	var lookedUp string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			lookedUp = r.URL.Path
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "tok").CommitFiles(context.Background(), "nullus/api", port.CommitSpec{
		Files: []port.CommitFile{{Path: "deploy/deployment.yaml", Content: "x"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "/api/v1/repos/nullus/api/contents/deploy/deployment.yaml", lookedUp)
}

// 삭제의 목표는 "없는 상태" 다. 404 를 오류로 올리면 재시도가 영영 끝나지 않는다.
func TestDeleteProject_MissingIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	require.NoError(t, NewClient(srv.URL, "tok").DeleteProject(context.Background(), "nullus/api"))
}

func TestEnsureWebhook_SkipsWhenAlreadyPresent(t *testing.T) {
	var createCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, []map[string]any{
				{"id": 1, "config": map[string]string{"url": "http://jenkins/gitea-webhook/post"}},
			})
		case http.MethodPost:
			createCalls++
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "tok").EnsureWebhook(
		context.Background(), "nullus/api", "http://jenkins/gitea-webhook/post", "")

	require.NoError(t, err)
	assert.Zero(t, createCalls, "같은 주소의 webhook 을 중복 등록하면 빌드가 두 번 돈다")
}

const srv2CloneURL = "http://gitea-http.nullus.svc:3000/nullus/api.git"

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(v))
}
