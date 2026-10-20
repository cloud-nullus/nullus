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
