package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	// WebSocketProtocolHeader 는 핸드셰이크에서 서브프로토콜을 협상하는 헤더다.
	WebSocketProtocolHeader = "Sec-WebSocket-Protocol"

	// WebSocketBearerProtocol 은 "다음 값이 Bearer 토큰" 이라는 표식이다.
	// 서버는 핸드셰이크 응답에 이 값을 그대로 돌려줘야 브라우저가 연결을 받아들인다
	// (gorilla Upgrader 의 Subprotocols 에 같은 값이 있어야 한다).
	WebSocketBearerProtocol = "bearer"
)

// WebSocketBearerSubprotocol lets browser WebSockets carry a bearer token.
//
// 브라우저의 WebSocket API 는 임의 헤더를 못 붙인다. 유일하게 통제할 수 있는 값이
// `new WebSocket(url, protocols)` 로 보내는 Sec-WebSocket-Protocol 이라, 여기에
// ["bearer", "<token>"] 을 실어 보내고 이 미들웨어가 표준 Authorization 헤더로 옮긴다.
// 그 뒤에 평소 쓰던 인증 미들웨어를 그대로 붙이면 HTTP 와 동일한 검증을 받는다.
//
// 쿼리 파라미터(`?token=`)를 쓰지 않는 이유는 토큰이 액세스 로그·프록시 로그·브라우저
// 히스토리에 남기 때문이다. 헤더로 오는 값은 그런 경로로 새지 않는다.
//
// 검증은 하지 않는다 — 자격증명을 옮기기만 하고, 거절 여부는 뒤의 인증 미들웨어가
// 정한다. 그래야 거절 논리가 한 곳에만 있다.
func WebSocketBearerSubprotocol() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			// 이미 헤더로 자격증명을 보낸 호출자(서버 간 호출, 테스트)를 덮지 않는다.
			if req.Header.Get(echo.HeaderAuthorization) != "" {
				return next(c)
			}

			if token := bearerFromSubprotocol(req.Header.Get(WebSocketProtocolHeader)); token != "" {
				req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
			}

			return next(c)
		}
	}
}

// bearerFromSubprotocol 은 "bearer, <token>" 형태에서 토큰만 뽑는다.
// 형태가 다르면 빈 문자열을 돌려 통과시킨다.
func bearerFromSubprotocol(header string) string {
	if header == "" {
		return ""
	}

	parts := strings.Split(header, ",")
	if len(parts) < 2 {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(parts[0]), WebSocketBearerProtocol) {
		return ""
	}

	return strings.TrimSpace(parts[1])
}
