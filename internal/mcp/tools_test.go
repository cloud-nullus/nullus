package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedRequest 는 목 API 가 받은 요청 하나다.
type recordedRequest struct {
	Method string
	Path   string // 쿼리 포함
	Body   map[string]any
	Auth   string
}

// mockAPI 는 tool 이 두드릴 REST 표면을 흉내 내고 모든 요청을 기록한다
// (설계 §8 — httptest 목 API + in-memory transport).
type mockAPI struct {
	mu       sync.Mutex
	requests []recordedRequest
	// routes 는 "METHOD /path"(쿼리 제외) → 응답 본문이다. 없는 경로는 404
	// 에러 envelope 를 돌려준다.
	routes map[string]any
	server *httptest.Server
}

func newMockAPI(t *testing.T, routes map[string]any) *mockAPI {
	t.Helper()
	m := &mockAPI{routes: routes}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recordedRequest{
			Method: r.Method,
			Path:   r.URL.RequestURI(),
			Auth:   r.Header.Get("Authorization"),
		}
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &rec.Body)
		}
		m.mu.Lock()
		m.requests = append(m.requests, rec)
		m.mu.Unlock()

		resp, ok := m.routes[r.Method+" "+r.URL.Path]
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "NOT_FOUND", "message": "no route " + r.URL.Path, "trace_id": "trc-1"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(m.server.Close)
	return m
}

func (m *mockAPI) recorded() []recordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]recordedRequest(nil), m.requests...)
}

// callTool 은 in-memory 세션으로 tool 을 호출하고 text content 를 돌려준다.
func callTool(t *testing.T, cs *sdk.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err, "tool %s 프로토콜 오류", name)
	require.NotEmpty(t, res.Content, "tool %s 응답에 content 가 없다", name)
	text, ok := res.Content[0].(*sdk.TextContent)
	require.True(t, ok, "tool %s content 가 text 가 아니다", name)
	return text.Text, res.IsError
}

func newTestSession(t *testing.T, api *mockAPI, allowWrite bool) *sdk.ClientSession {
	t.Helper()
	server := NewServer(testClient(t, api.server.URL), Options{Version: "test", AllowWrite: allowWrite})
	return connect(t, server)
}

func TestReadTools_CallExpectedEndpoints(t *testing.T) {
	cases := []struct {
		tool       string
		args       map[string]any
		wantMethod string
		wantPath   string
	}{
		{"stack_list", nil, http.MethodGet, "/api/v1/stacks"},
		{"stack_status", map[string]any{"stack_id": "stk-1"}, http.MethodGet, "/api/v1/stacks/stk-1/status"},
		{"cluster_list", nil, http.MethodGet, "/api/v1/admin/clusters"},
		{"template_list", nil, http.MethodGet, "/api/v1/stacks/templates"},
		{"compat_check", map[string]any{"stack_id": "stk-1"}, http.MethodPost, "/api/v1/stacks/stk-1/validate"},
		// stack_logs_tail 은 REST 전용 tail 경로를 쓴다 — /deploy/logs 는 WS 다.
		{"stack_logs_tail", map[string]any{"stack_id": "stk-1", "lines": 50}, http.MethodGet, "/api/v1/stacks/stk-1/deploy/logs/tail?lines=50"},
		{"stack_logs_tail", map[string]any{"stack_id": "stk-1"}, http.MethodGet, "/api/v1/stacks/stk-1/deploy/logs/tail?lines=100"},
	}

	for _, tc := range cases {
		t.Run(tc.tool+" "+tc.wantPath, func(t *testing.T) {
			api := newMockAPI(t, map[string]any{
				tc.wantMethod + " " + strings.SplitN(tc.wantPath, "?", 2)[0]: map[string]any{"ok": true},
			})
			cs := newTestSession(t, api, false)

			text, isErr := callTool(t, cs, tc.tool, tc.args)
			assert.False(t, isErr, "tool 이 에러를 반환했다: %s", text)

			reqs := api.recorded()
			require.Len(t, reqs, 1)
			assert.Equal(t, tc.wantMethod, reqs[0].Method)
			assert.Equal(t, tc.wantPath, reqs[0].Path)
			assert.Equal(t, "Bearer test-token", reqs[0].Auth)
		})
	}
}

func TestCompatCheck_ToolsOverrideInBody(t *testing.T) {
	api := newMockAPI(t, map[string]any{
		"POST /api/v1/stacks/stk-1/validate": map[string]any{"overall": "pass"},
	})
	cs := newTestSession(t, api, false)

	_, isErr := callTool(t, cs, "compat_check", map[string]any{
		"stack_id": "stk-1",
		"tools":    map[string]any{"jenkins": "2.452.1"},
	})
	assert.False(t, isErr)

	reqs := api.recorded()
	require.Len(t, reqs, 1)
	require.NotNil(t, reqs[0].Body)
	assert.Equal(t, map[string]any{"jenkins": "2.452.1"}, reqs[0].Body["tools"])
}

func TestClusterList_RedactsSecrets(t *testing.T) {
	api := newMockAPI(t, map[string]any{
		"GET /api/v1/admin/clusters": map[string]any{
			"items": []map[string]any{{
				"id":         "cl-1",
				"name":       "prod",
				"kubeconfig": "apiVersion: v1\nclusters: ...",
				"metadata":   map[string]any{"token": "sekrit"},
			}},
		},
	})
	cs := newTestSession(t, api, false)

	text, isErr := callTool(t, cs, "cluster_list", nil)
	assert.False(t, isErr)
	assert.Contains(t, text, "prod")
	// 시크릿성 키는 값 마스킹이 아니라 키 자체가 사라져야 한다 (설계 §5).
	assert.NotContains(t, text, "kubeconfig")
	assert.NotContains(t, text, "sekrit")
}

func TestStackDeploy_CreatesThenDeploys_SCMTokenFromEnvOnly(t *testing.T) {
	t.Setenv(EnvSCMToken, "pat-from-env")

	api := newMockAPI(t, map[string]any{
		"POST /api/v1/stacks":                map[string]any{"id": "stk-new"},
		"POST /api/v1/stacks/stk-new/deploy": map[string]any{"status": "accepted"},
	})
	cs := newTestSession(t, api, true)

	text, isErr := callTool(t, cs, "stack_deploy", map[string]any{
		"stack": map[string]any{
			"name": "demo",
			"scm":  map[string]any{"url": "https://github.com/org/repo", "token": "leaked-arg"},
		},
		"acknowledge_warnings": true,
	})
	assert.False(t, isErr, "stack_deploy 실패: %s", text)
	assert.Contains(t, text, "stk-new")

	reqs := api.recorded()
	require.Len(t, reqs, 2)

	// 1) 생성 본문 — stacks.config 는 평문 JSONB 라 시크릿성 키는 제거돼야 한다.
	create := reqs[0]
	assert.Equal(t, "POST", create.Method)
	assert.Equal(t, "/api/v1/stacks", create.Path)
	scm, _ := create.Body["scm"].(map[string]any)
	assert.Equal(t, "https://github.com/org/repo", scm["url"])
	_, hasToken := scm["token"]
	assert.False(t, hasToken, "생성 본문에 토큰이 실렸다: %v", create.Body)

	// 2) deploy 본문 — PAT 는 env 값이 source_control 로만 나간다.
	deploy := reqs[1]
	assert.Equal(t, "/api/v1/stacks/stk-new/deploy", deploy.Path)
	assert.Equal(t, true, deploy.Body["acknowledge_warnings"])
	sc, _ := deploy.Body["source_control"].(map[string]any)
	assert.Equal(t, "pat-from-env", sc["personal_access_token"])
}

func TestStackDeploy_NoEnvToken_OmitsSourceControl(t *testing.T) {
	api := newMockAPI(t, map[string]any{
		"POST /api/v1/stacks":                map[string]any{"id": "stk-new"},
		"POST /api/v1/stacks/stk-new/deploy": map[string]any{"status": "accepted"},
	})
	cs := newTestSession(t, api, true)

	_, isErr := callTool(t, cs, "stack_deploy", map[string]any{
		"stack": map[string]any{"name": "demo"},
	})
	assert.False(t, isErr)

	reqs := api.recorded()
	require.Len(t, reqs, 2)
	_, has := reqs[1].Body["source_control"]
	assert.False(t, has, "env 없이 source_control 이 실렸다: %v", reqs[1].Body)
	_, hasAck := reqs[1].Body["acknowledge_warnings"]
	assert.False(t, hasAck, "명시하지 않은 acknowledge_warnings 가 실렸다")
}

func TestStackRollback_CamelCaseVersionID(t *testing.T) {
	api := newMockAPI(t, map[string]any{
		"POST /api/v1/stacks/stk-1/rollback": map[string]any{"id": "ver-3"},
	})
	cs := newTestSession(t, api, true)

	_, isErr := callTool(t, cs, "stack_rollback", map[string]any{
		"stack_id":   "stk-1",
		"version_id": "ver-2",
		"reason":     "bad config",
	})
	assert.False(t, isErr)

	reqs := api.recorded()
	require.Len(t, reqs, 1)
	// 서버 rollbackRequest 는 camelCase 다 — snake_case 는 조용히 무시돼
	// "versionId is required" 400 이 난다.
	assert.Equal(t, "ver-2", reqs[0].Body["versionId"])
	_, hasSnake := reqs[0].Body["version_id"]
	assert.False(t, hasSnake)
	assert.Equal(t, "bad config", reqs[0].Body["reason"])
}

func TestPipelineDeploy_UsesCICDGroupPath(t *testing.T) {
	api := newMockAPI(t, map[string]any{
		"POST /api/v1/cicd/pipelines/pl-1/deploy": map[string]any{"deploymentId": "dep-9"},
	})
	cs := newTestSession(t, api, true)

	text, isErr := callTool(t, cs, "pipeline_deploy", map[string]any{"pipeline_id": "pl-1"})
	assert.False(t, isErr, "pipeline_deploy 실패: %s", text)

	reqs := api.recorded()
	require.Len(t, reqs, 1)
	assert.Equal(t, "/api/v1/cicd/pipelines/pl-1/deploy", reqs[0].Path)
}

func TestToolCall_APIErrorSurfacesAsToolError(t *testing.T) {
	api := newMockAPI(t, map[string]any{}) // 모든 경로 404
	cs := newTestSession(t, api, false)

	text, isErr := callTool(t, cs, "stack_status", map[string]any{"stack_id": "no-such"})
	assert.True(t, isErr, "API 404 가 tool 에러로 표시되지 않았다")
	// APIError.Error() 가 상태·trace_id 를 담는다 — 모델이 원인을 추적할 단서다.
	assert.Contains(t, text, "404")
	assert.Contains(t, text, "trc-1")
}
