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

// Anthropic Messages 는 시스템 프롬프트가 따로 있고, 도구 호출은 tool_use 블록, 도구 결과는 user 메시지의
// tool_result 블록이다. 연달은 도구 결과는 user 메시지 하나로 묶어야 한다.
func TestAnthropic_TranslatesConversationAndTools(t *testing.T) {
	var got map[string]any
	var headers http.Header
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, headers = r.URL.Path, r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"상태를 보겠습니다."},
		  {"type":"tool_use","id":"toolu_1","name":"stack_status","input":{"stack_id":"stk_1"}}],"stop_reason":"tool_use"}`)
	}))
	defer srv.Close()
	c := NewAnthropic(Config{BaseURL: srv.URL, APIKey: "sk-ant-x", Model: "claude-sonnet-5-5"})

	msg, err := c.Complete(context.Background(), port.CompletionRequest{
		Messages: []domain.Message{
			{Role: domain.RoleSystem, Content: "규칙"},
			{Role: domain.RoleUser, Content: "실패한 스택?"},
			{Role: domain.RoleAssistant, ToolCalls: []domain.ToolCall{
				{ID: "t1", Name: "stack_list", Arguments: "{}"}, {ID: "t2", Name: "cluster_list", Arguments: ""}}},
			{Role: domain.RoleTool, ToolCallID: "t1", Content: `{"items":[]}`},
			{Role: domain.RoleTool, ToolCallID: "t2", Content: "403"},
		},
		Tools: []domain.ToolSpec{{Name: "stack_status", Description: "상태", Parameters: map[string]any{"type": "object"}}},
	})
	require.NoError(t, err)

	assert.Equal(t, "/v1/messages", path)
	assert.Equal(t, "sk-ant-x", headers.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", headers.Get("anthropic-version"))
	assert.Empty(t, headers.Get("Authorization"))
	assert.Equal(t, "claude-sonnet-5-5", got["model"])
	assert.Equal(t, "규칙", got["system"])
	assert.NotZero(t, got["max_tokens"])
	assert.Equal(t, map[string]any{"type": "auto"}, got["tool_choice"])
	tool := got["tools"].([]any)[0].(map[string]any)
	assert.Equal(t, "stack_status", tool["name"])
	assert.Equal(t, map[string]any{"type": "object"}, tool["input_schema"])

	msgs := got["messages"].([]any)
	require.Len(t, msgs, 3, "system 은 빠지고, 연달은 도구 결과는 user 하나로 묶인다")
	assistant := msgs[1].(map[string]any)
	blocks := assistant["content"].([]any)
	assert.Equal(t, "tool_use", blocks[0].(map[string]any)["type"])
	assert.Equal(t, map[string]any{}, blocks[1].(map[string]any)["input"], "빈 인자는 빈 객체다")
	results := msgs[2].(map[string]any)
	assert.Equal(t, "user", results["role"])
	rb := results["content"].([]any)
	require.Len(t, rb, 2)
	assert.Equal(t, "tool_result", rb[0].(map[string]any)["type"])
	assert.Equal(t, "t1", rb[0].(map[string]any)["tool_use_id"])

	assert.Equal(t, "상태를 보겠습니다.", msg.Content)
	require.Len(t, msg.ToolCalls, 1)
	assert.Equal(t, "toolu_1", msg.ToolCalls[0].ID)
	assert.Equal(t, "stack_status", msg.ToolCalls[0].Name)
	assert.JSONEq(t, `{"stack_id":"stk_1"}`, msg.ToolCalls[0].Arguments)
}

func TestAnthropic_OmitsToolsWhenNoneAndReportsErrors(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		if got["model"] == "bad" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"답"}]}`)
	}))
	defer srv.Close()

	msg, err := NewAnthropic(Config{BaseURL: srv.URL, APIKey: "k", Model: "m"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.NoError(t, err)
	assert.Equal(t, "답", msg.Content)
	assert.NotContains(t, got, "tools")
	assert.NotContains(t, got, "tool_choice")

	_, err = NewAnthropic(Config{BaseURL: srv.URL, APIKey: "sk-ant-secret", Model: "bad"}).Complete(context.Background(),
		port.CompletionRequest{Messages: []domain.Message{{Role: domain.RoleUser, Content: "a"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "invalid x-api-key")
	assert.NotContains(t, err.Error(), "sk-ant-secret")
}
