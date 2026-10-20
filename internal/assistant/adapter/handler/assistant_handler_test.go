package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/usecase"
)

type stubChat struct {
	enabled bool
	out     *usecase.ChatOutput
	err     error
	got     usecase.ChatInput
}

func (s *stubChat) Enabled() bool { return s.enabled }
func (s *stubChat) Execute(_ context.Context, in usecase.ChatInput) (*usecase.ChatOutput, error) {
	s.got = in
	return s.out, s.err
}

func serve(t *testing.T, chat *stubChat, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	NewAssistantHandler(chat, Status{Provider: "upstage", Model: "solar-pro4"}).RegisterRoutes(e.Group("/api/v1/assistant"))
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestStatus(t *testing.T) {
	rec := serve(t, &stubChat{enabled: true}, http.MethodGet, "/api/v1/assistant/status", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"enabled":true,"provider":"upstage","model":"solar-pro4"}`, rec.Body.String())

	// 꺼져 있으면 모델 이름도 알리지 않는다 — 화면은 버튼을 숨긴다.
	rec = serve(t, &stubChat{}, http.MethodGet, "/api/v1/assistant/status", "", nil)
	assert.JSONEq(t, `{"enabled":false}`, rec.Body.String())
}

func TestChat_AnswersWithToolsAndForwardsCredentials(t *testing.T) {
	chat := &stubChat{enabled: true, out: &usecase.ChatOutput{Reply: "실패한 스택은 없습니다.",
		Tools: []domain.ToolTrace{{Name: "stack_list", OK: true}}}}
	rec := serve(t, chat, http.MethodPost, "/api/v1/assistant/chat",
		`{"messages":[{"role":"user","content":"실패한 스택 있어?"}],"context":{"path":"/stack/list"}}`,
		map[string]string{"Authorization": "Bearer t", "X-User-Role": "devops", "X-User-OrgID": "org-1", "Cookie": "a=b"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"reply":"실패한 스택은 없습니다.","tools":[{"name":"stack_list","ok":true}]}`, rec.Body.String())

	assert.Equal(t, []domain.Turn{{Role: domain.RoleUser, Content: "실패한 스택 있어?"}}, chat.got.Turns)
	assert.Equal(t, "/stack/list", chat.got.Page.Path)
	assert.Equal(t, "Bearer t", chat.got.Credentials["Authorization"])
	assert.Equal(t, "devops", chat.got.Credentials["X-User-Role"])
	assert.Equal(t, "org-1", chat.got.Credentials["X-User-OrgID"])
	assert.NotContains(t, chat.got.Credentials, "Cookie", "인증 헤더만 넘긴다")
}

func TestChat_Errors(t *testing.T) {
	cases := []struct {
		name   string
		chat   *stubChat
		body   string
		status int
		code   string
	}{
		{"꺼져 있다", &stubChat{}, `{"messages":[{"role":"user","content":"a"}]}`, http.StatusServiceUnavailable, "ASSISTANT_NOT_CONFIGURED"},
		{"JSON 이 아니다", &stubChat{enabled: true}, `{`, http.StatusBadRequest, "ASSISTANT_INVALID"},
		{"대화 규칙 위반", &stubChat{enabled: true, err: fmt.Errorf("%w: 마지막은 사용자 질문", domain.ErrInvalidConversation)},
			`{"messages":[{"role":"assistant","content":"a"}]}`, http.StatusBadRequest, "ASSISTANT_INVALID"},
		{"모델 실패", &stubChat{enabled: true, err: fmt.Errorf("%w: 401", domain.ErrUpstream)},
			`{"messages":[{"role":"user","content":"a"}]}`, http.StatusBadGateway, "ASSISTANT_UPSTREAM"},
		{"그 밖", &stubChat{enabled: true, err: errors.New("도구 세션을 열지 못했습니다")},
			`{"messages":[{"role":"user","content":"a"}]}`, http.StatusInternalServerError, "ASSISTANT_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(t, tc.chat, http.MethodPost, "/api/v1/assistant/chat", tc.body, nil)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.code, body.Error.Code)
		})
	}
}
