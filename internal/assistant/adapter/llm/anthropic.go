package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
)

// anthropicVersion 은 Messages API 버전 헤더다.
const anthropicVersion = "2023-06-01"

// anthropicMaxTokens 는 답 하나의 토큰 상한이다(Messages API 는 필수다). thinking 토큰도 이 안에 들어간다 —
// 작으면 도구 입력이 잘리거나 답이 빈다. 스트리밍을 쓰지 않는 요청의 시간 안에 드는 값으로 둔다.
const anthropicMaxTokens = 16000

// AnthropicClient 는 Anthropic Messages API 클라이언트다. 시스템 프롬프트는 별도 필드, 도구 호출은 tool_use 블록,
// 도구 결과는 user 메시지의 tool_result 블록이다.
type AnthropicClient struct {
	cfg Config
	hc  *http.Client
}

var _ port.LLM = (*AnthropicClient)(nil)

func NewAnthropic(cfg Config) *AnthropicClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 90 * time.Second
	}
	// 주소는 /v1 을 빼고 받지만, 넣어도 /v1/v1 이 되지 않게 한다(OpenAI 계열은 /v1 까지 넣는다).
	cfg.BaseURL = strings.TrimSuffix(strings.TrimRight(cfg.BaseURL, "/"), "/v1")
	return &AnthropicClient{cfg: cfg, hc: newHTTPClient(cfg.Timeout)}
}

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

// anthropicMessage 는 Raw 가 있으면 그 content 를 그대로 보낸다 — 이전 응답의 원본(thinking 블록 · 서명)을
// 바꾸지 않고 돌려줘야 한다.
type anthropicMessage struct {
	Role    string
	Content []anthropicBlock
	Raw     json.RawMessage
}

func (m anthropicMessage) MarshalJSON() ([]byte, error) {
	if len(m.Raw) > 0 {
		return json.Marshal(struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}{m.Role, m.Raw})
	}
	return json.Marshal(struct {
		Role    string           `json:"role"`
		Content []anthropicBlock `json:"content"`
	}{m.Role, m.Content})
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicRequest struct {
	Model      string             `json:"model"`
	MaxTokens  int                `json:"max_tokens"`
	System     string             `json:"system,omitempty"`
	Messages   []anthropicMessage `json:"messages"`
	Tools      []anthropicTool    `json:"tools,omitempty"`
	ToolChoice map[string]string  `json:"tool_choice,omitempty"`
}

type anthropicResponse struct {
	Content    json.RawMessage `json:"content"`
	StopReason string          `json:"stop_reason"`
}

func (c *AnthropicClient) Complete(ctx context.Context, req port.CompletionRequest) (domain.Message, error) {
	body := anthropicRequest{Model: c.cfg.Model, MaxTokens: anthropicMaxTokens}
	var system []string
	for _, m := range req.Messages {
		switch m.Role {
		case domain.RoleSystem:
			system = append(system, m.Content)
		case domain.RoleUser:
			body.Messages = append(body.Messages, anthropicMessage{Role: "user", Content: []anthropicBlock{{Type: "text", Text: m.Content}}})
		case domain.RoleAssistant:
			if len(m.ProviderData) > 0 && json.Valid(m.ProviderData) {
				body.Messages = append(body.Messages, anthropicMessage{Role: "assistant", Raw: m.ProviderData})
				continue
			}
			var blocks []anthropicBlock
			if strings.TrimSpace(m.Content) != "" {
				blocks = append(blocks, anthropicBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				input := json.RawMessage(tc.Arguments)
				if strings.TrimSpace(tc.Arguments) == "" || !json.Valid(input) {
					input = json.RawMessage("{}")
				}
				blocks = append(blocks, anthropicBlock{Type: "tool_use", ID: tc.ID, Name: tc.Name, Input: input})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropicBlock{Type: "text", Text: " "})
			}
			body.Messages = append(body.Messages, anthropicMessage{Role: "assistant", Content: blocks})
		case domain.RoleTool:
			// 연달은 도구 결과는 user 메시지 하나로 묶는다 — 따로 보내면 user 가 연달아 와서 거부된다.
			result := anthropicBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content}
			if n := len(body.Messages); n > 0 && body.Messages[n-1].Role == "user" && isToolResults(body.Messages[n-1]) {
				body.Messages[n-1].Content = append(body.Messages[n-1].Content, result)
			} else {
				body.Messages = append(body.Messages, anthropicMessage{Role: "user", Content: []anthropicBlock{result}})
			}
		}
	}
	body.System = strings.Join(system, "\n\n")
	for _, t := range req.Tools {
		schema := t.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object"}
		}
		body.Tools = append(body.Tools, anthropicTool{Name: t.Name, Description: t.Description, InputSchema: schema})
	}
	if len(body.Tools) > 0 {
		body.ToolChoice = map[string]string{"type": "auto"}
		if req.RequireToolUse {
			body.ToolChoice = map[string]string{"type": "any"}
		}
		if req.ForbidToolUse {
			body.ToolChoice = map[string]string{"type": "none"}
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return domain.Message{}, fmt.Errorf("요청 직렬화: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return domain.Message{}, fmt.Errorf("요청 생성: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return domain.Message{}, fmt.Errorf("모델 호출: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return domain.Message{}, fmt.Errorf("모델 응답 읽기: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return domain.Message{}, upstreamError(resp.StatusCode, data)
	}
	var out anthropicResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return domain.Message{}, fmt.Errorf("모델 응답 해석: %w", err)
	}
	var blocks []anthropicBlock
	if len(out.Content) > 0 {
		if err := json.Unmarshal(out.Content, &blocks); err != nil {
			return domain.Message{}, fmt.Errorf("모델 응답 해석: %w", err)
		}
	}

	// 원본 content 를 다음 라운드에 그대로 돌려준다(thinking 블록 · 서명 포함).
	msg := domain.Message{Role: domain.RoleAssistant, ProviderData: []byte(out.Content)}
	var text []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			text = append(text, b.Text)
		case "tool_use":
			args := string(b.Input)
			if args == "" {
				args = "{}"
			}
			msg.ToolCalls = append(msg.ToolCalls, domain.ToolCall{ID: b.ID, Name: b.Name, Arguments: args})
		}
	}
	msg.Content = strings.Join(text, "\n")
	switch {
	case out.StopReason == "refusal":
		return domain.Message{}, fmt.Errorf("모델이 답을 거절했습니다")
	case out.StopReason == "max_tokens" && strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0:
		return domain.Message{}, fmt.Errorf("모델 출력이 토큰 상한(%d)에 닿아 답을 만들지 못했습니다", anthropicMaxTokens)
	}
	return msg, nil
}

func isToolResults(m anthropicMessage) bool {
	return len(m.Content) > 0 && m.Content[0].Type == "tool_result"
}
