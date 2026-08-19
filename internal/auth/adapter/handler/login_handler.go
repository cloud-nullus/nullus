// Package handler 는 인증 컨텍스트의 HTTP 어댑터다.
package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/auth/domain"
	"github.com/cloud-nullus/draft/internal/auth/usecase"
)

type LoginHandler struct {
	login *usecase.Login
}

func NewLoginHandler(login *usecase.Login) *LoginHandler {
	return &LoginHandler{login: login}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginUser 는 응답에 실을 사용자 정보다.
//
// domain.Credential 을 그대로 직렬화하지 않는다 — PasswordHash 가 함께 나가면
// 오프라인 크래킹 대상이 된다.
type loginUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	OrgID string `json:"orgId"`
}

type loginResponse struct {
	Token string    `json:"token"`
	User  loginUser `json:"user"`
}

// Login 은 ID/PW 로 인증하고 세션 토큰을 돌려준다.
//
// OIDC 와 나란히 서는 두 번째 경로다 — IdP 가 죽어도 들어갈 수단이 있어야 한다.
func (h *LoginHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	out, err := h.login.Execute(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			// 어느 쪽이 틀렸는지 알리지 않는다 — 알리면 어떤 이메일이 가입돼
			// 있는지 알아낼 수 있다.
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
		}
		// 저장소 장애를 401 로 답하면 DB 가 죽었을 때 전 사용자가 "비밀번호가
		// 틀렸다" 는 답을 받고 원인을 못 찾는다.
		return echo.NewHTTPError(http.StatusInternalServerError, "login failed")
	}

	return c.JSON(http.StatusOK, loginResponse{
		Token: out.Token,
		User: loginUser{
			ID:    out.User.UserID,
			Email: out.User.Email,
			Name:  out.User.Name,
			Role:  out.User.Role,
			OrgID: out.User.OrgID,
		},
	})
}

// RegisterRoutes 는 로그인 라우트를 붙인다.
//
// 인증 미들웨어 앞에 있어야 한다 — 로그인하려면 먼저 통과해야 하기 때문이다.
func (h *LoginHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/auth/login", h.Login)
}
