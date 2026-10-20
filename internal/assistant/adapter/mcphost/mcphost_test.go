package mcphost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/assistant/port"
)

// fakeAPI 는 Nullus REST 를 흉내 낸다. 받은 인증 헤더를 남긴다.
type fakeAPI struct {
	mu      sync.Mutex
	headers []http.Header
}

func (f *fakeAPI) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/stacks", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.headers = append(f.headers, r.Header.Clone())
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"stk_1","name":"devsecops","status":"failed"}],"total":1}`))
	})
	mux.HandleFunc("/api/v1/admin/clusters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"FORBIDDEN","message":"admin only"}}`))
	})
	return mux
}

func openSession(t *testing.T, api *fakeAPI, creds port.Credentials) port.ToolSession {
	t.Helper()
	srv := httptest.NewServer(api.handler())
	t.Cleanup(srv.Close)
	s, err := NewOpener(srv.URL, "test").Open(context.Background(), creds)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// 어시스턴트는 조회 도구만 쓴다 — 변경 도구는 목록에 아예 없다.
func TestSession_ListsReadToolsOnly(t *testing.T) {
	s := openSession(t, &fakeAPI{}, nil)

	specs, err := s.ListTools(context.Background())
	require.NoError(t, err)
	names := map[string]bool{}
	for _, sp := range specs {
		names[sp.Name] = true
		assert.NotEmpty(t, sp.Description)
		assert.Equal(t, "object", sp.Parameters["type"], "%s 의 인자 스키마", sp.Name)
	}
	for _, want := range []string{"stack_list", "stack_status", "cluster_list", "template_list", "compat_check", "stack_logs_tail"} {
		assert.True(t, names[want], want)
	}
	for _, write := range []string{"stack_deploy", "stack_rollback", "pipeline_deploy"} {
		assert.False(t, names[write], write)
	}
}

// 도구는 질문한 사용자의 인증 헤더를 그대로 싣는다 — RBAC 는 서버가 판정한다.
func TestSession_CallsAPIWithRequesterCredentials(t *testing.T) {
	api := &fakeAPI{}
	s := openSession(t, api, port.Credentials{"Authorization": "Bearer user-token", "X-User-Role": "developer"})

	out, isError, err := s.CallTool(context.Background(), "stack_list", "")
	require.NoError(t, err)
	assert.False(t, isError)
	assert.Contains(t, out, "devsecops")

	require.Len(t, api.headers, 1)
	assert.Equal(t, "Bearer user-token", api.headers[0].Get("Authorization"))
	assert.Equal(t, "developer", api.headers[0].Get("X-User-Role"))
}

// 권한 없음은 도구의 실패 결과로 돌아온다 — 모델이 사정을 설명할 수 있다.
func TestSession_APIErrorIsAToolError(t *testing.T) {
	s := openSession(t, &fakeAPI{}, nil)

	out, isError, err := s.CallTool(context.Background(), "cluster_list", "{}")
	require.NoError(t, err)
	assert.True(t, isError)
	assert.Contains(t, out, "403")
}

// 모델이 JSON 이 아닌 인자를 만들면 도구 실패로 알려 다시 고치게 한다.
func TestSession_InvalidArgumentsAreAToolError(t *testing.T) {
	s := openSession(t, &fakeAPI{}, nil)

	out, isError, err := s.CallTool(context.Background(), "stack_status", "{stack_id: stk_1")
	require.NoError(t, err)
	assert.True(t, isError)
	assert.Contains(t, out, "JSON")
}
