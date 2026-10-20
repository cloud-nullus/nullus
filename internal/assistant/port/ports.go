// Package port 는 AI 어시스턴트가 바깥과 만나는 자리다 — 모델(LLM)과 도구(MCP).
package port

import (
	"context"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
)

// CompletionRequest 는 모델에 한 번 묻는 내용이다. Tools 가 비면 도구 없이 답하게 한다.
type CompletionRequest struct {
	Messages []domain.Message
	Tools    []domain.ToolSpec
}

// LLM 은 OpenAI 호환 Chat Completions 모델이다. 응답 메시지는 답(Content)이거나 도구 호출(ToolCalls)이다.
type LLM interface {
	Complete(ctx context.Context, req CompletionRequest) (domain.Message, error)
}

// Credentials 는 도구가 API 를 부를 때 실을 요청자의 인증 헤더다(Authorization · X-User-*). 도구는 질문한
// 사용자의 권한으로만 돈다.
type Credentials map[string]string

// ToolSession 은 한 질문 동안 쓰는 도구 묶음이다.
type ToolSession interface {
	ListTools(ctx context.Context) ([]domain.ToolSpec, error)
	// CallTool 은 도구를 부른다. isError 는 도구가 실패를 결과로 돌려준 것(권한 없음 · 없는 스택)이고,
	// err 는 도구를 부르지 못한 것이다.
	CallTool(ctx context.Context, name, arguments string) (output string, isError bool, err error)
	Close() error
}

// ToolSessionOpener 는 요청자의 인증으로 도구 세션을 연다.
type ToolSessionOpener interface {
	Open(ctx context.Context, creds Credentials) (ToolSession, error)
}
