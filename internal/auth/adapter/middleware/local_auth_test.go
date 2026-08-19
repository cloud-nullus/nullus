package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/auth/adapter/keycloak"
	"github.com/cloud-nullus/draft/internal/auth/adapter/token"
)

// DualAuthMiddleware 는 이름과 달리 authMode 로 하나만 골랐다. 그래서 OIDC 배포에서는
// ID/PW 로 들어갈 방법이 아예 없었고, IdP 가 죽으면 아무도 못 들어갔다.
// 두 경로가 동시에 서야 한다 — OIDC 토큰과 우리가 발급한 세션 토큰 둘 다 받는다.

const mwSecret = "middleware-test-secret-32bytes!!!"

func localBearer(t *testing.T, c token.Claims) string {
	t.Helper()
	raw, err := token.NewLocalIssuer(mwSecret, time.Hour).Issue(c)
	require.NoError(t, err)
	return raw
}

func dualHandler(t *testing.T, authMode, bearer string, headers map[string]string) (*httptest.ResponseRecorder, *admindomain.User) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var seen *admindomain.User
	mw := DualAuthMiddleware(authMode,
		AuthMiddleware(),
		JWTAuthMiddleware(JWTConfig{}, keycloak.NewOIDCProvider()),
		WithLocalTokens(token.NewLocalIssuer(mwSecret, time.Hour)))
	_ = mw(func(c echo.Context) error {
		seen, _ = c.Get(userContextKey).(*admindomain.User)
		return c.NoContent(http.StatusOK)
	})(c)
	return rec, seen
}

// oidc 모드에서도 우리가 발급한 토큰을 받아야 한다. 이게 break-glass 경로다.
func TestDualAuth_OIDCMode_AcceptsLocalSessionToken(t *testing.T) {
	rec, user := dualHandler(t, "oidc", localBearer(t, token.Claims{
		UserID: "u-9", Email: "admin@nullus.dev", Name: "Admin", Role: "admin", OrgID: "org-1",
	}), nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, user, "로컬 토큰으로 인증된 사용자가 컨텍스트에 없다")
	assert.Equal(t, "u-9", user.ID)
	assert.Equal(t, admindomain.Role("admin"), user.Role)
	assert.Equal(t, "org-1", user.OrgID)
}

// session 모드에서도 마찬가지다. 모드는 IdP 연동 여부일 뿐, ID/PW 경로를 막는
// 스위치가 아니다.
func TestDualAuth_SessionMode_AcceptsLocalSessionToken(t *testing.T) {
	rec, user := dualHandler(t, "session", localBearer(t, token.Claims{UserID: "u-9", Role: "devops"}), nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, user)
	assert.Equal(t, "u-9", user.ID)
}

// 우리 토큰이 아니면 기존 검증기로 넘어가야 한다. 여기서 가로채면 Keycloak
// 사용자가 전부 401 이 된다.
func TestDualAuth_ForeignTokenFallsThroughToOIDC(t *testing.T) {
	// issuer 가 우리 것이 아닌(그리고 서명도 우리 것이 아닌) 토큰.
	rec, user := dualHandler(t, "oidc", "eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJodHRwczovL2tjL3JlYWxtcy9uIn0.x", nil)
	assert.Nil(t, user)
	// OIDC 검증기가 처리해 401 을 낸다 — 로컬 검증기가 삼키지 않았다는 뜻이다.
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// 서명이 틀린 로컬 토큰은 통과하면 안 된다.
func TestDualAuth_RejectsForgedLocalToken(t *testing.T) {
	forged, err := token.NewLocalIssuer("a-different-secret-32-bytes-long!", time.Hour).
		Issue(token.Claims{UserID: "attacker", Role: "admin"})
	require.NoError(t, err)

	rec, user := dualHandler(t, "oidc", forged, nil)
	assert.Nil(t, user, "위조된 토큰이 사용자를 만들었다")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// 로컬 발급기를 주입하지 않으면 예전 동작 그대로여야 한다.
func TestDualAuth_WithoutLocalIssuerKeepsSessionBehaviour(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := DualAuthMiddleware("session", AuthMiddleware(),
		JWTAuthMiddleware(JWTConfig{}, keycloak.NewOIDCProvider()))(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}
