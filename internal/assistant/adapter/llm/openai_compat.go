// Package llm 은 모델 API 클라이언트다 — OpenAI 호환 Chat Completions(Upstage · OpenAI · Gemini · 사내 vLLM 등)와
// Anthropic Messages. 어느 것을 쓸지는 Factory 가 설정의 공급자로 고른다.
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

// DefaultBaseURL · DefaultModel 은 Upstage 의 Chat Completions 와 기본 모델이다.
const (
	DefaultBaseURL = "https://api.upstage.ai/v1"
	DefaultModel   = "solar-pro4"
)

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	// Timeout 은 한 번 묻는 상한이다. 0 이면 90초.
	Timeout time.Duration
}

type Client struct {
	cfg Config
	hc  *http.Client
}

var _ port.LLM = (*Client)(nil)

func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 90 * time.Second
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{cfg: cfg, hc: &http.Client{Timeout: cfg.Timeout}}
}

// Model 은 쓰는 모델 이름이다(상태 API 가 보인다).
func (c *Client) Model() string { return c.cfg.Model }

type wireFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Arguments   *string        `json:"arguments,omitempty"`
}

type wireToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
	// ExtraContent 는 Gemini 의 OpenAI 호환 엔드포인트가 붙이는 공급자 전용 데이터(사고 서명)다. 그대로 되돌려 준다.
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

type wireMessage struct {
	Role       string         `json:"role"`
	Content    *string        `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireRequest struct {
	Model      string        `json:"model"`
	Messages   []wireMessage `json:"messages"`
	Tools      []wireTool    `json:"tools,omitempty"`
	ToolChoice string        `json:"tool_choice,omitempty"`
}

type wireResponse struct {
	Choices []struct {
		Message wireMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete 는 한 번 묻는다. 도구가 있으면 모델이 고르게(tool_choice=auto), 없으면 도구 필드를 보내지 않는다.
func (c *Client) Complete(ctx context.Context, req port.CompletionRequest) (domain.Message, error) {
	body := wireRequest{Model: c.cfg.Model, Messages: make([]wireMessage, 0, len(req.Messages))}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, toWire(m))
	}
	for _, t := range req.Tools {
		body.Tools = append(body.Tools, wireTool{Type: "function", Function: wireFunction{
			Name: t.Name, Description: t.Description, Parameters: t.Parameters,
		}})
	}
	if len(body.Tools) > 0 {
		body.ToolChoice = "auto"
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return domain.Message{}, fmt.Errorf("요청 직렬화: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return domain.Message{}, fmt.Errorf("요청 생성: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return domain.Message{}, fmt.Errorf("모델 호출: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return domain.Message{}, fmt.Errorf("모델 응답 읽기: %w", err)
	}

	var out wireResponse
	decodeErr := json.Unmarshal(data, &out)
	if resp.StatusCode/100 != 2 {
		detail := strings.TrimSpace(string(data))
		if decodeErr == nil && out.Error != nil && out.Error.Message != "" {
			detail = out.Error.Message
		}
		if len(detail) > 300 {
			detail = detail[:300]
		}
		return domain.Message{}, fmt.Errorf("모델 응답 %d: %s", resp.StatusCode, detail)
	}
	if decodeErr != nil {
		return domain.Message{}, fmt.Errorf("모델 응답 해석: %w", decodeErr)
	}
	if len(out.Choices) == 0 {
		return domain.Message{}, fmt.Errorf("모델 응답에 choices 가 없다")
	}
	return fromWire(out.Choices[0].Message), nil
}

func toWire(m domain.Message) wireMessage {
	w := wireMessage{Role: string(m.Role), ToolCallID: m.ToolCallID}
	// 도구 호출만 있는 assistant 메시지는 content 가 null 이어도 된다. 그 밖에는 문자열로 보낸다.
	if m.Content != "" || len(m.ToolCalls) == 0 {
		content := m.Content
		w.Content = &content
	}
	for _, tc := range m.ToolCalls {
		args := tc.Arguments
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		w.ToolCalls = append(w.ToolCalls, wireToolCall{ID: tc.ID, Type: "function",
			Function: wireFunction{Name: tc.Name, Arguments: &args}, ExtraContent: tc.ProviderData})
	}
	return w
}

func fromWire(w wireMessage) domain.Message {
	m := domain.Message{Role: domain.RoleAssistant}
	if w.Content != nil {
		m.Content = *w.Content
	}
	for _, tc := range w.ToolCalls {
		args := ""
		if tc.Function.Arguments != nil {
			args = *tc.Function.Arguments
		}
		m.ToolCalls = append(m.ToolCalls, domain.ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: args,
			ProviderData: []byte(tc.ExtraContent)})
	}
	return m
}
