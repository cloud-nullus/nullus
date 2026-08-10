package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func runWSSubprotocol(t *testing.T, headerValue string) *http.Request {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/ws/deployments/d-1/logs", nil)
	if headerValue != "" {
		req.Header.Set(WebSocketProtocolHeader, headerValue)
	}
	c := e.NewContext(req, httptest.NewRecorder())

	h := WebSocketBearerSubprotocol()(func(c echo.Context) error { return nil })
	if err := h(c); err != nil {
		t.Fatalf("middleware returned error: %v", err)
	}
	return c.Request()
}

// 브라우저는 WebSocket 에 Authorization 헤더를 못 붙인다. 대신 서브프로토콜로
// 토큰을 실어 보내면 이 미들웨어가 평범한 Bearer 헤더로 옮겨 준다 — 그래야 뒤에
// 오는 기존 인증 미들웨어가 HTTP 와 똑같은 검증을 할 수 있다.
func TestWebSocketBearerSubprotocol_MovesTokenIntoAuthorizationHeader(t *testing.T) {
	req := runWSSubprotocol(t, "bearer, eyJhbGciOi.payload.sig")

	if got := req.Header.Get(echo.HeaderAuthorization); got != "Bearer eyJhbGciOi.payload.sig" {
		t.Fatalf("expected the token to become a Bearer header, got %q", got)
	}
}

func TestWebSocketBearerSubprotocol_ToleratesMissingSpaceAfterComma(t *testing.T) {
	req := runWSSubprotocol(t, "bearer,tok-123")

	if got := req.Header.Get(echo.HeaderAuthorization); got != "Bearer tok-123" {
		t.Fatalf("expected %q, got %q", "Bearer tok-123", got)
	}
}

// 이미 헤더로 인증을 보낸 호출자(테스트·서버 간 호출)를 덮어쓰면 안 된다.
func TestWebSocketBearerSubprotocol_KeepsExistingAuthorizationHeader(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/ws/deployments/d-1/logs", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer original")
	req.Header.Set(WebSocketProtocolHeader, "bearer, from-subprotocol")
	c := e.NewContext(req, httptest.NewRecorder())

	h := WebSocketBearerSubprotocol()(func(c echo.Context) error { return nil })
	if err := h(c); err != nil {
		t.Fatalf("middleware returned error: %v", err)
	}

	if got := c.Request().Header.Get(echo.HeaderAuthorization); got != "Bearer original" {
		t.Fatalf("existing credentials must win, got %q", got)
	}
}

// 인증은 뒤따르는 미들웨어의 몫이다. 여기서 401 을 내면 두 곳에서 거절 논리가
// 갈라지므로, 못 알아본 값은 그냥 통과시킨다.
func TestWebSocketBearerSubprotocol_PassesThroughWhenNoUsableToken(t *testing.T) {
	for _, header := range []string{"", "bearer", "graphql-ws", "bearer, "} {
		req := runWSSubprotocol(t, header)
		if got := req.Header.Get(echo.HeaderAuthorization); got != "" {
			t.Fatalf("header %q: expected no Authorization header, got %q", header, got)
		}
	}
}
