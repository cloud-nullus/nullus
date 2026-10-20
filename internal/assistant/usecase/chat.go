// Package usecase 는 AI 어시스턴트의 에이전트 루프다 — 대화를 모델에 넘기고, 모델이 고른 도구를 질문한
// 사용자의 권한으로 실행해 결과를 되돌려 주며 답을 만든다.
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
)

// fallbackReply 는 모델이 빈 답을 돌려줬을 때 화면에 보일 말이다.
const fallbackReply = "답을 만들지 못했습니다. 질문을 조금 바꿔 다시 물어봐 주세요."

// ChatInput 은 질문 하나다. Turns 는 화면이 들고 있는 대화 전체(마지막이 이번 질문)다.
type ChatInput struct {
	Turns       []domain.Turn
	Page        domain.PageContext
	Credentials port.Credentials
}

// ChatOutput 은 답과 그 답을 위해 쓴 도구다.
type ChatOutput struct {
	Reply string
	Tools []domain.ToolTrace
}

// Chat 은 에이전트 루프다. llm 이 없으면 꺼져 있다.
type Chat struct {
	llm   port.LLM
	tools port.ToolSessionOpener
}

func NewChat(llm port.LLM, tools port.ToolSessionOpener) *Chat {
	return &Chat{llm: llm, tools: tools}
}

// Enabled 는 모델이 설정됐는지다.
func (uc *Chat) Enabled() bool { return uc != nil && uc.llm != nil }

// Execute 는 질문에 답한다. 도구 라운드는 domain.MaxToolRounds 까지 — 넘기면 도구 없이 한 번 더 물어
// 그때까지의 결과로 마무리하게 한다.
func (uc *Chat) Execute(ctx context.Context, in ChatInput) (*ChatOutput, error) {
	if !uc.Enabled() {
		return nil, domain.ErrNotConfigured
	}
	if err := domain.ValidateTurns(in.Turns); err != nil {
		return nil, err
	}
	session, err := uc.tools.Open(ctx, in.Credentials)
	if err != nil {
		return nil, fmt.Errorf("도구 세션을 열지 못했습니다: %w", err)
	}
	defer func() { _ = session.Close() }()

	specs, err := session.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("도구 목록을 읽지 못했습니다: %w", err)
	}

	messages := make([]domain.Message, 0, len(in.Turns)+1)
	messages = append(messages, domain.Message{Role: domain.RoleSystem, Content: domain.SystemPrompt(in.Page)})
	for _, t := range in.Turns {
		messages = append(messages, domain.Message{Role: t.Role, Content: t.Content})
	}

	started := time.Now()
	out := &ChatOutput{Tools: []domain.ToolTrace{}}
	for round := 0; ; round++ {
		tools := specs
		if round >= domain.MaxToolRounds {
			tools = nil
		}
		resp, err := uc.llm.Complete(ctx, port.CompletionRequest{Messages: messages, Tools: tools})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", domain.ErrUpstream, err)
		}
		if len(resp.ToolCalls) == 0 || tools == nil {
			out.Reply = strings.TrimSpace(resp.Content)
			if out.Reply == "" {
				out.Reply = fallbackReply
			}
			slog.Info("assistant answered", "rounds", round+1, "tools", len(out.Tools), "elapsed", time.Since(started).Round(time.Millisecond))
			return out, nil
		}
		messages = append(messages, domain.Message{Role: domain.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls})
		for _, call := range resp.ToolCalls {
			result, isError, err := session.CallTool(ctx, call.Name, call.Arguments)
			if err != nil {
				result, isError = "도구 실행 실패: "+err.Error(), true
			}
			out.Tools = append(out.Tools, domain.ToolTrace{Name: call.Name, OK: !isError})
			messages = append(messages, domain.Message{
				Role:       domain.RoleTool,
				ToolCallID: call.ID,
				// 가린 뒤 자른다 — 먼저 자르면 경계에 걸린 토큰 조각이 패턴에 안 걸려 그대로 나간다.
				Content: domain.TruncateToolOutput(domain.RedactSecrets(result), domain.MaxToolOutputChars),
			})
		}
	}
}
