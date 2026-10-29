package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
)

func TestClient_SendsToolsAndHistoryInOpenAIShape(t *testing.T) {
	var got map[string]any
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":null,
		  "tool_calls":[{"id":"call_1","type":"function","function":{"name":"stack_status","arguments":"{\"stack_id\":\"stk_1\"}"}}]}}]}`)
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL + "/v1/", APIKey: "up_key", Model: "solar-pro4"})

	msg, err := c.Complete(context.Background(), port.CompletionRequest{
		Messages: []domain.Message{
			{Role: domain.RoleSystem, Content: "규칙"},
			{Role: domain.RoleUser, Content: "왜 실패?"},
			{Role: domain.RoleAssistant, ToolCalls: []domain.ToolCall{{ID: "c0", Name: "stack_list", Arguments: "{}"}}},
			{Role: domain.RoleTool, ToolCallID: "c0", Content: "[]"},
		},
		Tools: []domain.ToolSpec{{Name: "stack_status", Description: "상태", Parameters: map[string]any{"type": "object"}}},
	})
	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", path)
	assert.Equal(t, "Bearer up_key", auth)
	assert.Equal(t, "solar-pro4", got["model"])
	assert.Equal(t, "auto", got["tool_choice"])

	msgs := got["messages"].([]any)
	require.Len(t, msgs, 4)
	call := msgs[2].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)
	assert.Equal(t, "function", call["type"])
	assert.Equal(t, "stack_list", call["function"].(map[string]any)["name"])
	assert.Equal(t, "c0", msgs[3].(map[string]any)["tool_call_id"])
	tool := got["tools"].([]any)[0].(map[string]any)
	assert.Equal(t, "function", tool["type"])
	assert.Equal(t, "stack_status", tool["function"].(map[string]any)["name"])

	assert.Equal(t, domain.RoleAssistant, msg.Role)
	require.Len(t, msg.ToolCalls, 1)
	assert.Equal(t, domain.ToolCall{ID: "call_1", Name: "stack_status", Arguments: `{"stack_id":"stk_1"}`}, msg.ToolCalls[0])
}

// 도구 없이 물을 때는 tools · tool_choice 를 아예 보내지 않는다(빈 배열을 거부하는 서버가 있다).
func TestClient_OmitsToolsWhenNone(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"답"}}]}`)
	}))
	defer srv.Close()

	msg, err := New(Config{BaseURL: srv.URL, APIKey: "k", Model: "m"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.NoError(t, err)
	assert.Equal(t, "답", msg.Content)
	assert.NotContains(t, got, "tools")
	assert.NotContains(t, got, "tool_choice")
}

// 실패는 상태 코드와 서버의 오류 메시지로 알린다 — 키는 싣지 않는다.
func TestClient_UpstreamErrorHidesKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"Invalid API key","type":"invalid_request_error"}}`)
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, APIKey: "up_secret_key", Model: "m"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "Invalid API key")
	assert.NotContains(t, err.Error(), "up_secret_key")
}

func TestClient_NoChoicesIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[]}`)
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, APIKey: "k", Model: "m"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	assert.Error(t, err)
}

// Gemini 는 도구 호출에 사고 서명(extra_content)을 붙이고, 다음 요청에 그대로 돌려받지 못하면 400 이다.
func TestClient_EchoesProviderDataOnToolCalls(t *testing.T) {
	var second map[string]any
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if calls == 1 {
			_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function",
			  "function":{"name":"stack_list","arguments":"{}"},"extra_content":{"google":{"thought_signature":"sig=="}}}]}}]}`)
			return
		}
		_ = json.Unmarshal(body, &second)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL, APIKey: "k", Model: "gemini-x"})

	first, err := c.Complete(context.Background(), port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.NoError(t, err)
	require.Len(t, first.ToolCalls, 1)
	_, err = c.Complete(context.Background(), port.CompletionRequest{Messages: []domain.Message{
		{Role: domain.RoleUser, Content: "a"}, first, {Role: domain.RoleTool, ToolCallID: "c1", Content: "[]"},
	}})
	require.NoError(t, err)
	call := second["messages"].([]any)[1].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)
	assert.Equal(t, map[string]any{"google": map[string]any{"thought_signature": "sig=="}}, call["extra_content"])
}

// 키가 없는 사내 서버에는 Authorization 을 보내지 않는다.
func TestClient_NoAuthorizationWithoutKey(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, Model: "m"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.NoError(t, err)
	assert.Empty(t, auth)
}

// 마무리 라운드: 도구 정의는 보내고 tool_choice 를 none 으로.
func TestClient_ForbidToolUseSetsNone(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"끝"}}]}`)
	}))
	defer srv.Close()
	_, err := New(Config{BaseURL: srv.URL, APIKey: "k", Model: "m"}).Complete(context.Background(), port.CompletionRequest{
		Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}},
		Tools:    []domain.ToolSpec{{Name: "stack_list", Parameters: map[string]any{"type": "object"}}}, ForbidToolUse: true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, got["tools"])
	assert.Equal(t, "none", got["tool_choice"])
}

// 리다이렉트를 따라가지 않는다 — 관리자가 넣은 주소가 내부의 다른 곳으로 튀지 않게.
func TestClient_DoesNotFollowRedirects(t *testing.T) {
	hit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, APIKey: "k", Model: "m"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.Error(t, err)
	assert.False(t, hit)
	assert.Contains(t, err.Error(), "302")
}

// JSON 오류가 아닌 응답 본문은 싣지 않는다 — 연결 테스트로 내부 서버의 내용을 읽어 내지 못하게.
func TestClient_NonJSONErrorBodyIsNotEchoed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "<html>internal admin console secret-token-123</html>")
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, APIKey: "k", Model: "m"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.NotContains(t, err.Error(), "secret-token-123")
}

// 링크로컬(클라우드 메타데이터 169.254.169.254) 대역으로는 연결하지 않는다.
func TestClient_RefusesLinkLocalAddresses(t *testing.T) {
	_, err := New(Config{BaseURL: "http://169.254.169.254/latest", APIKey: "k", Model: "m"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "링크로컬")
}

// 조회 질문의 첫 라운드: 모델이 도구를 건너뛰고 지어내지 못하게 tool_choice 를 required 로 보낸다.
func TestClient_RequireToolUseSendsRequired(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":null,
		  "tool_calls":[{"id":"call_1","type":"function","function":{"name":"cluster_list","arguments":"{}"}}]}}]}`)
	}))
	defer srv.Close()

	msg, err := New(Config{BaseURL: srv.URL, APIKey: "k", Model: "m"}).Complete(context.Background(), port.CompletionRequest{
		Messages:       []domain.Message{{Role: domain.RoleUser, Content: "등록된 클러스터 목록"}},
		Tools:          []domain.ToolSpec{{Name: "cluster_list", Description: "목록", Parameters: map[string]any{"type": "object"}}},
		RequireToolUse: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "required", got["tool_choice"])
	require.Len(t, msg.ToolCalls, 1)
	assert.Equal(t, "cluster_list", msg.ToolCalls[0].Name)
}

// 상한 라운드가 강제보다 앞선다 — 둘 다 켜져 오면 none 이다.
func TestClient_ForbidWinsOverRequire(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"답"}}]}`)
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, APIKey: "k", Model: "m"}).Complete(context.Background(), port.CompletionRequest{
		Messages:       []domain.Message{{Role: domain.RoleUser, Content: "a"}},
		Tools:          []domain.ToolSpec{{Name: "cluster_list", Parameters: map[string]any{"type": "object"}}},
		RequireToolUse: true, ForbidToolUse: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "none", got["tool_choice"])
}
