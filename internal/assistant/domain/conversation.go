// Package domain 은 AI 어시스턴트의 대화 규칙이다 — 화면이 보낸 대화의 검증, 모델과 주고받는 메시지,
// 외부 LLM 으로 나가는 도구 결과의 가리기와 자르기. 외부 의존이 없다.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Role 은 메시지를 누가 말했는지다(OpenAI 호환 Chat Completions 의 역할 이름).
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

const (
	// MaxTurns · MaxTurnChars 는 화면이 한 번에 보낼 수 있는 대화의 크기다 — 크레딧과 컨텍스트를 묶는다.
	MaxTurns     = 20
	MaxTurnChars = 4000
	// MaxToolRounds 는 한 질문에서 도구를 부르는 라운드 상한이다. 넘기면 도구 없이 한 번 더 물어 마무리한다.
	MaxToolRounds = 6
	// MaxToolOutputChars 는 도구 결과 하나를 모델에 넘기는 길이 상한이다(GitLab 설치 로그는 수만 자다).
	MaxToolOutputChars = 12000
	// MaxToolCallsPerRound · MaxToolCallsTotal 은 한 라운드 · 한 질문에서 실행하는 도구 호출 수다. 도구는 API 를
	// 루프백으로 부르므로 IP 상한(127.0.0.1 하나)과 질문한 사용자의 레이트리밋을 함께 쓴다.
	MaxToolCallsPerRound = 8
	MaxToolCallsTotal    = 20
	// MaxPathChars 는 시스템 프롬프트에 넣는 화면 경로의 길이 상한이다.
	MaxPathChars = 200
)

var (
	// ErrInvalidConversation 은 화면이 보낸 대화가 규칙에 맞지 않는다는 뜻이다.
	ErrInvalidConversation = errors.New("assistant: invalid conversation")
	// ErrNotConfigured 는 LLM API 키가 없어 어시스턴트가 꺼져 있다는 뜻이다.
	ErrNotConfigured = errors.New("assistant: not configured")
	// ErrUpstream 은 LLM 호출이 실패했다는 뜻이다(키 · 크레딧 · 응답 오류).
	ErrUpstream = errors.New("assistant: llm request failed")
	// ErrTimeout 은 질문 하나에 쓸 시간을 넘겼다는 뜻이다.
	ErrTimeout = errors.New("assistant: timed out")
)

// Turn 은 화면이 보낸 대화 한 줄이다. 화면은 user · assistant 만 보낼 수 있다.
type Turn struct {
	Role    Role
	Content string
}

// ToolCall 은 모델이 고른 도구 호출이다. Arguments 는 모델이 만든 JSON 문자열 그대로다. ProviderData 는
// 공급자가 호출에 붙여 보낸 것을 다음 요청에 그대로 되돌려 주는 자리다(Gemini 의 사고 서명 — 빠지면 400).
type ToolCall struct {
	ID           string
	Name         string
	Arguments    string
	ProviderData []byte
}

// Message 는 모델과 주고받는 메시지다. assistant 는 ToolCalls 를, tool 은 ToolCallID 를 가질 수 있다.
// ProviderData 는 공급자가 응답에 담아 보낸 원본을 다음 라운드에 그대로 돌려주는 자리다(Anthropic 의
// thinking 블록과 서명 — 빼거나 바꾸면 거부된다).
type Message struct {
	Role         Role
	Content      string
	ToolCalls    []ToolCall
	ToolCallID   string
	ProviderData []byte
}

// ToolSpec 은 모델에 알려 주는 도구 하나다. Parameters 는 JSON Schema 다.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// ToolTrace 는 답에 함께 싣는 "사용한 도구" 한 줄이다 — 인자 · 결과는 싣지 않는다.
type ToolTrace struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

// PageContext 는 사용자가 보고 있는 화면이다. NewPageContext 로 만든다.
type PageContext struct {
	Path string
}

// NewPageContext 는 화면 경로를 시스템 프롬프트에 넣을 수 있게 다듬는다 — 줄바꿈 · 제어 문자를 지우고
// 길이를 묶어, 경로에 지시문을 끼워 넣거나 비용 상한을 우회하지 못하게 한다.
func NewPageContext(path string) PageContext {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(path))
	if r := []rune(cleaned); len(r) > MaxPathChars {
		cleaned = string(r[:MaxPathChars])
	}
	return PageContext{Path: cleaned}
}

// ValidateTurns 는 화면이 보낸 대화를 검증한다. system · tool 역할을 꾸며 넣어 지시를 바꾸지 못하게 한다.
func ValidateTurns(turns []Turn) error {
	if len(turns) == 0 {
		return fmt.Errorf("%w: 대화가 비었다", ErrInvalidConversation)
	}
	if len(turns) > MaxTurns {
		return fmt.Errorf("%w: 대화는 %d턴까지다", ErrInvalidConversation, MaxTurns)
	}
	for i, t := range turns {
		if t.Role != RoleUser && t.Role != RoleAssistant {
			return fmt.Errorf("%w: %d번째 역할 %q 는 받지 않는다", ErrInvalidConversation, i+1, t.Role)
		}
		if strings.TrimSpace(t.Content) == "" {
			return fmt.Errorf("%w: %d번째 내용이 비었다", ErrInvalidConversation, i+1)
		}
		if utf8.RuneCountInString(t.Content) > MaxTurnChars {
			return fmt.Errorf("%w: 한 턴은 %d자까지다", ErrInvalidConversation, MaxTurnChars)
		}
	}
	if turns[len(turns)-1].Role != RoleUser {
		return fmt.Errorf("%w: 마지막은 사용자 질문이어야 한다", ErrInvalidConversation)
	}
	return nil
}

// TruncateToolOutput 은 도구 결과를 max 자 안팎으로 줄인다. 앞 1/3 과 뒤 2/3 를 남기고 가운데를 생략한다 —
// 설치 로그는 오래된 줄부터 와서 실패 원인이 끝에 있다.
func TruncateToolOutput(s string, max int) string {
	n := utf8.RuneCountInString(s)
	if n <= max {
		return s
	}
	r := []rune(s)
	head := max / 3
	tail := max - head
	return fmt.Sprintf("%s\n…(%d자 생략)…\n%s", string(r[:head]), n-max, string(r[n-tail:]))
}
