package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func TestClient_ImplementsPipelineConfigurator(t *testing.T) {
	var _ port.PipelineConfigurator = (*Client)(nil)
}

func TestSetProjectVariable_CreatesWhenAbsent(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "HARBOR_USERNAME"})
	})

	c := NewClient(srv.URL, "tok")
	err := c.SetProjectVariable(context.Background(), "51", port.ProjectVariable{
		Key: "HARBOR_USERNAME", Value: "robot", Masked: true,
	})
	require.NoError(t, err)
	require.Len(t, *recorded, 1)

	req := (*recorded)[0]
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "/api/v4/projects/51/variables", req.Path)
	assert.Equal(t, "HARBOR_USERNAME", req.Body["key"])
	assert.Equal(t, true, req.Body["masked"])
}

// 재프로비저닝 시 변수가 이미 있으면 갱신해야 한다. 그러지 않으면
// 자격증명이 바뀌어도 예전 값이 남아 빌드가 계속 실패한다.
func TestSetProjectVariable_UpdatesWhenAlreadyExists(t *testing.T) {
	attempt := 0
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		attempt++
		if attempt == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"message":{"key":["has already been taken"]}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "HARBOR_USERNAME"})
	})

	c := NewClient(srv.URL, "tok")
	err := c.SetProjectVariable(context.Background(), "51", port.ProjectVariable{
		Key: "HARBOR_USERNAME", Value: "robot2", Masked: true,
	})
	require.NoError(t, err)
	require.Len(t, *recorded, 2)

	update := (*recorded)[1]
	assert.Equal(t, http.MethodPut, update.Method)
	assert.Equal(t, "/api/v4/projects/51/variables/HARBOR_USERNAME", update.Path)
	assert.Equal(t, "robot2", update.Body["value"])
}

func TestSetProjectVariable_RequiresKey(t *testing.T) {
	c := NewClient("http://unused", "tok")
	err := c.SetProjectVariable(context.Background(), "51", port.ProjectVariable{Value: "v"})
	require.Error(t, err)
}

func TestCreateProjectAccessToken_ReturnsIssuedValue(t *testing.T) {
	srv, recorded := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 3, "token": "glpat-deploy"})
	})

	c := NewClient(srv.URL, "tok")
	token, err := c.CreateProjectAccessToken(context.Background(), "51", port.AccessTokenSpec{
		Name: "nullus-deploy", Scopes: []string{"write_repository"},
	})
	require.NoError(t, err)
	assert.Equal(t, "glpat-deploy", token)

	req := (*recorded)[0]
	assert.Equal(t, "/api/v4/projects/51/access_tokens", req.Path)
	assert.Equal(t, "nullus-deploy", req.Body["name"])
	scopes, ok := req.Body["scopes"].([]any)
	require.True(t, ok)
	assert.Equal(t, "write_repository", scopes[0])
	assert.NotEmpty(t, req.Body["expires_at"], "만료 없는 토큰은 회수 경로가 없다")

	// GitLab 은 만료일이 1년 뒤보다 "이전" 이어야 한다(경계 배타적).
	// 정확히 365일을 쓰면 400 으로 거부된다 — 실제로 겪었다.
	expires, err := time.Parse("2006-01-02", req.Body["expires_at"].(string))
	require.NoError(t, err)
	assert.True(t, expires.Before(time.Now().AddDate(1, 0, 0).AddDate(0, 0, -1)),
		"만료일이 1년 경계에 너무 붙으면 GitLab 이 거부한다: %s", expires)
}

func TestCreateProjectAccessToken_FailsWhenTokenMissingInResponse(t *testing.T) {
	srv, _ := newStubGitLab(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 3})
	})

	c := NewClient(srv.URL, "tok")
	_, err := c.CreateProjectAccessToken(context.Background(), "51", port.AccessTokenSpec{Name: "n"})
	require.Error(t, err)
}
