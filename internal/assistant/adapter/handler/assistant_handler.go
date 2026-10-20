// Package handler 는 AI 어시스턴트의 HTTP 어댑터다 — 상태 조회와 대화.
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	sharedmw "github.com/cloud-nullus/draft/internal/shared/middleware"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
	"github.com/cloud-nullus/draft/internal/assistant/usecase"
)

// credentialHeaders 는 도구가 API 를 부를 때 그대로 실을 요청자의 인증 헤더다 — OIDC · 로컬 토큰과 세션
// 모드의 X-User-*(internal/auth/adapter/middleware). 그 밖의 헤더(쿠키 등)는 넘기지 않는다.
// X-Org-ID 는 조직 주장이 없는 토큰 · 개발 모드에서 API 가 조직을 고르는 헤더다 — 빠지면 화면과 다른 조직을 본다.
var credentialHeaders = []string{"Authorization", "X-User-ID", "X-User-Email", "X-User-Name", "X-User-Role", "X-User-OrgID", "X-Org-ID"}

type chatter interface {
	Status(ctx context.Context, orgID string) (usecase.Status, error)
	Execute(ctx context.Context, in usecase.ChatInput) (*usecase.ChatOutput, error)
}

type AssistantHandler struct {
	chat chatter
}

func NewAssistantHandler(chat chatter) *AssistantHandler {
	return &AssistantHandler{chat: chat}
}

// RegisterRoutes 는 /api/v1/assistant 그룹에 붙는다. 그룹은 인증만 요구한다 — 무엇을 볼 수 있는지는
// 도구가 부르는 API 가 정한다.
func (h *AssistantHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/status", h.GetStatus)
	g.POST("/chat", h.Chat)
}

// GetStatus 는 요청자 조직의 어시스턴트가 켜져 있는지다. 꺼져 있으면 모델 이름도 알리지 않는다.
func (h *AssistantHandler) GetStatus(c echo.Context) error {
	st, err := h.chat.Status(c.Request().Context(), resolveOrgID(c))
	if err != nil {
		slog.Warn("assistant: 상태를 읽지 못했습니다", "error", err)
		return c.JSON(http.StatusOK, map[string]any{"enabled": false})
	}
	if !st.Enabled {
		return c.JSON(http.StatusOK, map[string]any{"enabled": false})
	}
	return c.JSON(http.StatusOK, map[string]any{"enabled": true, "provider": st.Provider, "model": st.Model})
}

type chatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Context struct {
		Path string `json:"path"`
	} `json:"context"`
}

func (h *AssistantHandler) Chat(c echo.Context) error {
	var req chatRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "ASSISTANT_INVALID", "요청 본문을 해석하지 못했습니다")
	}
	turns := make([]domain.Turn, 0, len(req.Messages))
	for _, m := range req.Messages {
		turns = append(turns, domain.Turn{Role: domain.Role(m.Role), Content: m.Content})
	}
	creds := port.Credentials{}
	for _, name := range credentialHeaders {
		if v := c.Request().Header.Get(name); v != "" {
			creds[name] = v
		}
	}

	out, err := h.chat.Execute(c.Request().Context(), usecase.ChatInput{
		OrgID: resolveOrgID(c), Turns: turns, Page: domain.NewPageContext(req.Context.Path), Credentials: creds,
	})
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, map[string]any{"reply": out.Reply, "tools": out.Tools})
	case errors.Is(err, domain.ErrInvalidConversation):
		return errorResponse(c, http.StatusBadRequest, "ASSISTANT_INVALID", err.Error())
	case errors.Is(err, domain.ErrNotConfigured):
		return errorResponse(c, http.StatusServiceUnavailable, "ASSISTANT_NOT_CONFIGURED", "AI 어시스턴트가 설정되지 않았습니다 — 관리 › AI 어시스턴트에서 모델과 키를 등록하세요")
	case errors.Is(err, domain.ErrTimeout):
		slog.Warn("assistant: 시간 초과", "error", err)
		return errorResponse(c, http.StatusGatewayTimeout, "ASSISTANT_TIMEOUT", "답을 만드는 데 너무 오래 걸렸습니다. 질문을 좁혀 다시 물어보세요")
	case errors.Is(err, domain.ErrUpstream):
		slog.Warn("assistant: 모델 호출 실패", "error", err)
		return errorResponse(c, http.StatusBadGateway, "ASSISTANT_UPSTREAM", "AI 모델 호출에 실패했습니다. 잠시 뒤 다시 시도하세요")
	default:
		slog.Warn("assistant: 대화 실패", "error", err)
		return errorResponse(c, http.StatusInternalServerError, "ASSISTANT_FAILED", "답을 만들지 못했습니다")
	}
}

func errorResponse(c echo.Context, httpStatus int, code, message string) error {
	return c.JSON(httpStatus, map[string]any{
		"error": map[string]any{"code": code, "http_status": httpStatus, "message": message},
	})
}

// resolveOrgID 는 로그인한 사용자의 조직, 없으면 X-Org-ID, 그것도 없으면 기본 조직이다(알림 채널 · 백업 API 와 같다).
func resolveOrgID(c echo.Context) string {
	if orgID := sharedmw.OrgIDFromEchoContext(c); orgID != "" {
		return orgID
	}
	if orgID := strings.TrimSpace(c.Request().Header.Get("X-Org-ID")); orgID != "" {
		return orgID
	}
	return shareddomain.DefaultOrgID()
}
