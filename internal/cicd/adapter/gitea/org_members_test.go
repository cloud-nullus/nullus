package gitea

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 플랫폼이 만든 저장소는 자동화 계정 소유의 private 조직 안에 있다. SSO 로
// 들어온 사람은 그 조직의 멤버가 아니라, 로그인은 되는데 화면이 텅 비어 보인다.
// 파이프라인을 만든 사람은 자기 저장소를 볼 수 있어야 한다.
func TestEnsureOrgMember_AddsUserToWriteTeam(t *testing.T) {
	var addedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/users/search"):
			assert.Equal(t, "dev@acme.io", r.URL.Query().Get("q"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"login": "devuser", "email": "dev@acme.io"}},
			})
		case r.URL.Path == "/api/v1/orgs/nullus/teams":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 7, "name": "Owners", "permission": "owner"},
				{"id": 9, "name": "developers", "permission": "write"},
			})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/members/"):
			addedPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "token").EnsureOrgMember(context.Background(), "nullus", "dev@acme.io")

	require.NoError(t, err)
	// Owners 가 아니라 write 팀에 넣는다. 저장소를 보고 밀 수 있으면 충분하고,
	// 조직 소유권까지 주면 되돌리기 어려운 권한이 조용히 퍼진다.
	assert.Equal(t, "/api/v1/teams/9/members/devuser", addedPath)
}

// 팀이 없으면 만든다. 새 스택의 조직에는 Owners 밖에 없다.
func TestEnsureOrgMember_CreatesTeamWhenMissing(t *testing.T) {
	created := false
	var addedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/users/search"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"login": "devuser", "email": "dev@acme.io"}},
			})
		case r.URL.Path == "/api/v1/orgs/nullus/teams" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 7, "name": "Owners", "permission": "owner"}})
		case r.URL.Path == "/api/v1/orgs/nullus/teams" && r.Method == http.MethodPost:
			created = true
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 11, "name": teamName, "permission": "write"})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/members/"):
			addedPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	require.NoError(t, NewClient(srv.URL, "token").
		EnsureOrgMember(context.Background(), "nullus", "dev@acme.io"))

	assert.True(t, created)
	assert.Equal(t, "/api/v1/teams/11/members/devuser", addedPath)
}

// 아직 한 번도 로그인하지 않았으면 Gitea 계정이 없다. 그것은 실패가 아니라
// "아직" 이다 — 호출부가 경고로 옮겨 담아 사용자에게 무엇을 하면 되는지 알린다.
func TestEnsureOrgMember_ReportsMissingUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/users/search") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "token").EnsureOrgMember(context.Background(), "nullus", "ghost@acme.io")

	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrSCMUserNotFound)
}

// 이메일이 없으면 누구를 넣을지 정해지지 않는다. 조용히 넘긴다 —
// 사용자를 특정할 수 없는 경로(자동화 호출 등)까지 실패로 만들 이유가 없다.
func TestEnsureOrgMember_SkipsWithoutEmail(t *testing.T) {
	require.NoError(t, NewClient("http://gitea.local", "token").
		EnsureOrgMember(context.Background(), "nullus", "  "))
}
