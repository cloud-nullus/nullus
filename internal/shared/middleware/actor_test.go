package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
)

type ctxUser struct {
	ID    string
	Email string
	Name  string
}

func newActorContext(t *testing.T, setup func(c echo.Context)) echo.Context {
	t.Helper()
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	if setup != nil {
		setup(c)
	}
	return c
}

// OIDC 모드에서는 헤더가 아니라 인증 미들웨어가 심은 컨텍스트에만 사용자가 있다.
// 여기서 못 읽으면 감사 로그의 user_id 가 통째로 빈다.
func TestActorFromContext_ReadsAuthenticatedUser(t *testing.T) {
	c := newActorContext(t, func(c echo.Context) {
		c.Set("current_user", &ctxUser{ID: "usr_1", Email: "devops@nullus.dev"})
	})

	actor := middleware.ActorFromContext(c)
	if actor.ID != "usr_1" || actor.Email != "devops@nullus.dev" {
		t.Fatalf("컨텍스트의 사용자를 못 읽었다: %+v", actor)
	}
}

// session 모드는 헤더로 신원이 온다.
func TestActorFromContext_FallsBackToHeaders(t *testing.T) {
	c := newActorContext(t, func(c echo.Context) {
		c.Request().Header.Set("X-User-ID", "usr_2")
		c.Request().Header.Set("X-User-Email", "admin@nullus.dev")
	})

	actor := middleware.ActorFromContext(c)
	if actor.ID != "usr_2" || actor.Email != "admin@nullus.dev" {
		t.Fatalf("헤더에서 신원을 못 읽었다: %+v", actor)
	}
}

// 신원을 못 찾아도 감사 로그는 남아야 한다. 빈 문자열보다 "누군지 모름" 이
// 낫다 — 나중에 로그를 읽는 사람이 버그인지 익명 호출인지 구분할 수 있다.
func TestActorFromContext_UnknownActorHasLabel(t *testing.T) {
	actor := middleware.ActorFromContext(newActorContext(t, nil))

	if actor.ID != "" || actor.Email != "" {
		t.Fatalf("없는 신원을 지어내면 안 된다: %+v", actor)
	}
	if actor.Label() != "unknown" {
		t.Fatalf("알 수 없는 주체의 표기가 틀렸다: %q", actor.Label())
	}
}

// 이력의 changed_by 는 사람이 읽는 값이라 이메일을 먼저 쓴다.
func TestActor_LabelPrefersEmail(t *testing.T) {
	actor := middleware.Actor{ID: "usr_3", Email: "sre@nullus.dev"}
	if actor.Label() != "sre@nullus.dev" {
		t.Fatalf("이메일이 우선이어야 한다: %q", actor.Label())
	}

	idOnly := middleware.Actor{ID: "usr_3"}
	if idOnly.Label() != "usr_3" {
		t.Fatalf("이메일이 없으면 ID 를 쓴다: %q", idOnly.Label())
	}
}
