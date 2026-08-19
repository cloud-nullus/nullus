package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/auth/adapter/token"
)

// localTokenVerifier 는 우리가 발급한 세션 토큰을 검증한다.
type localTokenVerifier interface {
	Enabled() bool
	Verify(raw string) (token.Claims, error)
}

// DualAuthOption 은 DualAuthMiddleware 의 선택 동작이다.
type DualAuthOption func(*dualAuthConfig)

type dualAuthConfig struct {
	local localTokenVerifier
}

// WithLocalTokens 는 ID/PW 로그인이 발급한 세션 토큰도 받게 한다.
func WithLocalTokens(v localTokenVerifier) DualAuthOption {
	return func(c *dualAuthConfig) { c.local = v }
}

// DualAuthMiddleware 는 두 인증 경로를 함께 세운다.
//
//	사용자 ──[OIDC 로그인]──▶ IdP(Keycloak) ──ID Token──┐
//	     └──[ID/PW 로그인]──▶ Nullus DB ──세션 토큰──────┴──▶ API
//
// 예전에는 authMode 로 둘 중 하나만 골랐다. 그래서 OIDC 배포에는 ID/PW 로 들어갈
// 방법이 아예 없었고, IdP 가 죽으면 아무도 들어갈 수 없었다.
//
// 토큰의 issuer 로 갈래를 정한다. 우리가 찍은 issuer 면 우리가 검증하고, 아니면
// 기존 검증기(authMode 가 고른 것)로 넘긴다 — 여기서 가로채면 IdP 사용자가 전부
// 401 이 되므로 넘기는 쪽이 기본이다.
func DualAuthMiddleware(authMode string, sessionMW, oidcMW echo.MiddlewareFunc, opts ...DualAuthOption) echo.MiddlewareFunc {
	cfg := &dualAuthConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	var fallback echo.MiddlewareFunc
	switch authMode {
	case "oidc":
		fallback = oidcMW
	default:
		fallback = sessionMW
	}

	if cfg.local == nil || !cfg.local.Enabled() {
		return fallback
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		fallbackNext := fallback(next)
		return func(c echo.Context) error {
			raw := bearerToken(c)
			if raw == "" || token.IssuerOf(raw) != token.LocalIssuer {
				return fallbackNext(c)
			}

			claims, err := cfg.local.Verify(raw)
			if err != nil {
				// 우리 issuer 를 자칭했으나 서명이 맞지 않는다. 다른 검증기로
				// 넘기지 않는다 — 넘기면 위조 토큰이 두 번째 기회를 얻는다.
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid session token",
				})
			}

			c.Set(userContextKey, &admindomain.User{
				ID:       claims.UserID,
				Email:    claims.Email,
				Name:     claims.Name,
				Role:     admindomain.Role(claims.Role),
				OrgID:    claims.OrgID,
				IsActive: true,
			})
			return next(c)
		}
	}
}

func bearerToken(c echo.Context) string {
	header := c.Request().Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
