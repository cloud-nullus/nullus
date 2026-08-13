package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
)

// 요청을 일으킨 사람이 누구인지 한 곳에서 푼다.
//
// 신원이 오는 경로가 인증 모드마다 다르다. session 모드는 X-User-* 헤더로,
// OIDC 모드는 인증 미들웨어가 컨텍스트에 심어 둔 사용자 객체로 온다. 핸들러가
// 헤더만 보면 OIDC 배포에서 감사 로그의 user_id 가 통째로 비어, "누가 바꿨나"
// 에 답할 수 없게 된다.
//
// 사용자 타입을 직접 참조하지 않고 필드 이름으로 읽는다. stack 모듈이 admin
// 모듈의 타입을 import 하지 않게 하려는 것이고, 이 패키지의 다른 코드
// (rate limiter, org context)도 같은 이유로 같은 방식을 쓴다.

// Actor 는 요청을 일으킨 주체다.
type Actor struct {
	ID    string
	Email string
	Name  string
}

// Label 은 사람이 읽을 표기다. 이력의 changed_by 처럼 화면에 그대로 나가는
// 자리에 쓴다 — 이메일이 가장 알아보기 쉽고, 없으면 ID, 그마저 없으면
// "unknown" 이다. 빈 문자열을 돌려주지 않는 이유는 뒤에서 "기록 누락" 과
// "익명 호출" 이 구분되지 않기 때문이다.
func (a Actor) Label() string {
	if email := strings.TrimSpace(a.Email); email != "" {
		return email
	}
	if id := strings.TrimSpace(a.ID); id != "" {
		return id
	}
	if name := strings.TrimSpace(a.Name); name != "" {
		return name
	}
	return "unknown"
}

// ActorFromContext 는 인증 컨텍스트를 먼저 보고, 없으면 헤더로 떨어진다.
func ActorFromContext(c echo.Context) Actor {
	actor := Actor{
		ID:    userField(c, "ID", "id", "sub"),
		Email: userField(c, "Email", "email"),
		Name:  userField(c, "Name", "name"),
	}

	req := c.Request()
	if req == nil {
		return actor
	}
	if actor.ID == "" {
		actor.ID = strings.TrimSpace(req.Header.Get("X-User-ID"))
	}
	if actor.Email == "" {
		actor.Email = strings.TrimSpace(req.Header.Get("X-User-Email"))
	}
	if actor.Name == "" {
		actor.Name = strings.TrimSpace(req.Header.Get("X-User-Name"))
	}
	return actor
}
