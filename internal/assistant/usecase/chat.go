// Package usecase 는 AI 어시스턴트의 에이전트 루프다 — 대화를 모델에 넘기고, 모델이 고른 도구를 질문한
// 사용자의 권한으로 실행해 결과를 되돌려 주며 답을 만든다.
package usecase

import (
	"context"
	"errors"
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

// DefaultDeadline 은 질문 하나에 쓰는 시간이다. 웹 앞단 프록시(nginx 기본 60초)보다 먼저 끝내야 화면이
// 504 대신 시간 초과 안내를 받는다.
const DefaultDeadline = 55 * time.Second

// Chat 은 에이전트 루프다. llm 이 없으면 꺼져 있다.
type Chat struct {
	llm      port.LLM
	tools    port.ToolSessionOpener
	deadline time.Duration
}

func NewChat(llm port.LLM, tools port.ToolSessionOpener) *Chat {
	return &Chat{llm: llm, tools: tools, deadline: DefaultDeadline}
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
	ctx, cancel := context.WithTimeout(ctx, uc.deadline)
	defer cancel()
	out, err := uc.run(ctx, in)
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("%w: %v", domain.ErrTimeout, err)
	}
	return out, err
}

func (uc *Chat) run(ctx context.Context, in ChatInput) (*ChatOutput, error) {
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
	executed := 0
	for round := 0; ; round++ {
		tools := specs
		if round >= domain.MaxToolRounds || executed >= domain.MaxToolCallsTotal {
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
			slog.Info("assistant answered", "rounds", round+1, "tools", toolNames(out.Tools), "elapsed", time.Since(started).Round(time.Millisecond))
			return out, nil
		}
		messages = append(messages, domain.Message{Role: domain.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls})
		for i, call := range resp.ToolCalls {
			// 상한을 넘긴 호출에도 결과 메시지는 준다 — 호출 ID 마다 결과가 있어야 다음 요청이 받아들여진다.
			if i >= domain.MaxToolCallsPerRound || executed >= domain.MaxToolCallsTotal {
				messages = append(messages, domain.Message{Role: domain.RoleTool, ToolCallID: call.ID,
					Content: "실행하지 않음: 도구 호출 상한을 넘었습니다. 지금까지의 결과로 답하세요."})
				continue
			}
			executed++
			result, isError, err := session.CallTool(ctx, call.Name, call.Arguments)
			if err != nil {
				if ctx.Err() != nil {
					return nil, err
				}
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

// toolNames 는 로그에 남길 도구 이름이다 — 인자 · 결과는 남기지 않는다.
func toolNames(traces []domain.ToolTrace) []string {
	names := make([]string, 0, len(traces))
	for _, t := range traces {
		names = append(names, t.Name)
	}
	return names
}
