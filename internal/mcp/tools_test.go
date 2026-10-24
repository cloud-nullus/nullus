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

const testClusterID = "6f1c2a9e-3b7d-4c1e-9a52-0d8e7f6a5b41"

// clustersRoute 는 stack_deploy 가 클러스터 이름을 ID 로 해석할 때 부르는 목록이다.
func clustersRoute(items ...map[string]any) map[string]any {
	return map[string]any{"items": items, "total": len(items)}
}

func TestStackDeploy_CreatesThenDeploys_SCMTokenFromEnvOnly(t *testing.T) {
	t.Setenv(EnvSCMToken, "pat-from-env")

	api := newMockAPI(t, map[string]any{
		"GET /api/v1/admin/clusters":         clustersRoute(map[string]any{"id": testClusterID, "name": "prod-01"}),
		"POST /api/v1/stacks":                map[string]any{"id": "stk-new"},
		"POST /api/v1/stacks/stk-new/deploy": map[string]any{"status": "accepted"},
	})
	cs := newTestSession(t, api, true)

	text, isErr := callTool(t, cs, "stack_deploy", map[string]any{
		"stack": map[string]any{
			"apiVersion": "nullus.io/v1alpha1",
			"kind":       "Stack",
			"name":       "demo",
			"cluster":    "prod-01",
			"config": map[string]any{
				"source_control": map[string]any{"owner": "org", "personal_access_token": "leaked-arg"},
			},
		},
		"acknowledge_warnings": true,
	})
	assert.False(t, isErr, "stack_deploy 실패: %s", text)
	assert.Contains(t, text, "stk-new")

	reqs := api.recorded()
	require.Len(t, reqs, 3)

	// 1) 생성 본문 — stacks.config 는 평문 JSONB 라 시크릿성 키는 제거돼야 한다.
	create := reqs[1]
	assert.Equal(t, "POST", create.Method)
	assert.Equal(t, "/api/v1/stacks", create.Path)
	cfg, _ := create.Body["config"].(map[string]any)
	sc, _ := cfg["source_control"].(map[string]any)
	assert.Equal(t, "org", sc["owner"])
	_, hasToken := sc["personal_access_token"]
	assert.False(t, hasToken, "생성 본문에 토큰이 실렸다: %v", create.Body)

	// 2) deploy 본문 — PAT 는 env 값이 source_control 로만 나간다.
	deploy := reqs[2]
	assert.Equal(t, "/api/v1/stacks/stk-new/deploy", deploy.Path)
	assert.Equal(t, true, deploy.Body["acknowledge_warnings"])
	dsc, _ := deploy.Body["source_control"].(map[string]any)
	assert.Equal(t, "pat-from-env", dsc["personal_access_token"])
}

// stack.yaml top-level 은 생성 API payload 로 매핑돼야 한다 (스키마 §1.2) —
// 그대로 보내면 서버는 cluster_id·golden_path_id 를 빈 값으로 받는다.
func TestStackDeploy_MapsStackYAMLToCreatePayload(t *testing.T) {
	api := newMockAPI(t, map[string]any{
		"GET /api/v1/admin/clusters": clustersRoute(
			map[string]any{"id": "c0a8f3d2-0000-4000-8000-000000000001", "name": "dev"},
			map[string]any{"id": testClusterID, "name": "prod-01"},
		),
		"POST /api/v1/stacks":                map[string]any{"id": "stk-new"},
		"POST /api/v1/stacks/stk-new/deploy": map[string]any{"status": "accepted"},
	})
	cs := newTestSession(t, api, true)

	text, isErr := callTool(t, cs, "stack_deploy", map[string]any{
		"stack": map[string]any{
			"apiVersion": "nullus.io/v1alpha1",
			"kind":       "Stack",
			"name":       "team-alpha",
			"cluster":    "prod-01",
			"namespace":  "nullus-team-alpha",
			"template":   "gitlab-argocd-v1",
			"config":     map[string]any{"access_domain": "team-alpha.internal"},
		},
	})
	require.False(t, isErr, "stack_deploy 실패: %s", text)

	reqs := api.recorded()
	require.Len(t, reqs, 3)
	assert.Equal(t, "/api/v1/admin/clusters", reqs[0].Path)
	assert.Equal(t, map[string]any{
		"name":           "team-alpha",
		"cluster_id":     testClusterID,
		"namespace":      "nullus-team-alpha",
		"golden_path_id": "gitlab-argocd-v1",
		"config":         map[string]any{"access_domain": "team-alpha.internal"},
	}, reqs[1].Body)
}

// cluster 가 이미 ID(UUID)면 조회하지 않는다 — /admin/clusters 는 admin 전용이라
// devops 사용자도 ID 로는 배포할 수 있어야 한다.
func TestStackDeploy_ClusterIDUsedWithoutLookup(t *testing.T) {
	api := newMockAPI(t, map[string]any{
		"POST /api/v1/stacks":                map[string]any{"id": "stk-new"},
		"POST /api/v1/stacks/stk-new/deploy": map[string]any{"status": "accepted"},
	})
	cs := newTestSession(t, api, true)

	text, isErr := callTool(t, cs, "stack_deploy", map[string]any{
		"stack": map[string]any{"name": "demo", "cluster": testClusterID},
	})
	require.False(t, isErr, "stack_deploy 실패: %s", text)

	reqs := api.recorded()
	require.Len(t, reqs, 2)
	assert.Equal(t, "/api/v1/stacks", reqs[0].Path)
	assert.Equal(t, testClusterID, reqs[0].Body["cluster_id"])
	_, hasSC := reqs[1].Body["source_control"]
	assert.False(t, hasSC, "env 없이 source_control 이 실렸다: %v", reqs[1].Body)
	_, hasAck := reqs[1].Body["acknowledge_warnings"]
	assert.False(t, hasAck, "명시하지 않은 acknowledge_warnings 가 실렸다")
}

func TestStackDeploy_InvalidStackRejectedBeforeCreate(t *testing.T) {
	cases := []struct {
		name     string
		clusters map[string]any
		stack    map[string]any
		wantMsg  string
	}{
		{"name 누락", nil, map[string]any{"cluster": testClusterID}, "name"},
		{"cluster 누락", nil, map[string]any{"name": "demo"}, "cluster"},
		{"없는 클러스터 이름", clustersRoute(map[string]any{"id": testClusterID, "name": "prod-01"}),
			map[string]any{"name": "demo", "cluster": "nope"}, "nope"},
		{"중복 클러스터 이름", clustersRoute(
			map[string]any{"id": testClusterID, "name": "prod-01"},
			map[string]any{"id": "c0a8f3d2-0000-4000-8000-000000000001", "name": "prod-01"},
		), map[string]any{"name": "demo", "cluster": "prod-01"}, "prod-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			routes := map[string]any{"POST /api/v1/stacks": map[string]any{"id": "stk-new"}}
			if tc.clusters != nil {
				routes["GET /api/v1/admin/clusters"] = tc.clusters
			}
			api := newMockAPI(t, routes)
			cs := newTestSession(t, api, true)

			text, isErr := callTool(t, cs, "stack_deploy", map[string]any{"stack": tc.stack})
			assert.True(t, isErr, "잘못된 입력이 통과했다: %s", text)
			assert.Contains(t, text, tc.wantMsg)
			for _, r := range api.recorded() {
				assert.NotEqual(t, "POST", r.Method, "검증 실패인데 생성 요청이 나갔다: %s", r.Path)
			}
		})
	}
}

// 생성 후 배포만 실패하면 스택은 이미 있다 — 모델이 재시도로 중복 스택을 만들지
// 않도록 에러에 stack_id 를 담아야 한다.
func TestStackDeploy_DeployFailureReportsCreatedStackID(t *testing.T) {
	api := newMockAPI(t, map[string]any{
		"POST /api/v1/stacks": map[string]any{"id": "stk-new"},
		// deploy 경로 없음 → 404
	})
	cs := newTestSession(t, api, true)

	text, isErr := callTool(t, cs, "stack_deploy", map[string]any{
		"stack": map[string]any{"name": "demo", "cluster": testClusterID},
	})
	assert.True(t, isErr)
	// 경로가 아니라 "생성됐다"는 사실로 stack_id 를 알려야 한다.
	assert.Contains(t, text, "스택 stk-new 은 생성됐")
	assert.Contains(t, text, "404")
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
