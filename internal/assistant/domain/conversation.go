// Package domain 은 AI 어시스턴트의 대화 규칙이다 — 화면이 보낸 대화의 검증, 모델과 주고받는 메시지,
// 외부 LLM 으로 나가는 도구 결과의 가리기와 자르기. 외부 의존이 없다.
package domain

import (
	"errors"
	"fmt"
	"strings"
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
)

var (
	// ErrInvalidConversation 은 화면이 보낸 대화가 규칙에 맞지 않는다는 뜻이다.
	ErrInvalidConversation = errors.New("assistant: invalid conversation")
	// ErrNotConfigured 는 LLM API 키가 없어 어시스턴트가 꺼져 있다는 뜻이다.
	ErrNotConfigured = errors.New("assistant: not configured")
	// ErrUpstream 은 LLM 호출이 실패했다는 뜻이다(키 · 크레딧 · 타임아웃).
	ErrUpstream = errors.New("assistant: llm request failed")
)

// Turn 은 화면이 보낸 대화 한 줄이다. 화면은 user · assistant 만 보낼 수 있다.
type Turn struct {
	Role    Role
	Content string
}

// ToolCall 은 모델이 고른 도구 호출이다. Arguments 는 모델이 만든 JSON 문자열 그대로다.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// Message 는 모델과 주고받는 메시지다. assistant 는 ToolCalls 를, tool 은 ToolCallID 를 가질 수 있다.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
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

// PageContext 는 사용자가 보고 있는 화면이다.
type PageContext struct {
	Path string
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

// TruncateToolOutput 은 도구 결과를 max 자로 자르고 생략한 길이를 적는다.
func TruncateToolOutput(s string, max int) string {
	n := utf8.RuneCountInString(s)
	if n <= max {
		return s
	}
	r := []rune(s)
	return fmt.Sprintf("%s\n…(%d자 생략)", string(r[:max]), n-max)
}
