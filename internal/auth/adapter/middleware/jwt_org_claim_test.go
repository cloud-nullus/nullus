package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/auth/adapter/keycloak"
)

// OIDC 로그인으로 들어온 요청은 org_id 를 principal 까지 전달해야 한다.
// 전달되지 않으면 핸들러의 resolveOrgID 가 존재하지 않는 기본 조직으로
// 폴백해 스택 생성이 FK 위반(stacks_org_id_fkey)으로 실패한다.
func TestJWTAuthMiddleware_PropagatesOrgIDClaim(t *testing.T) {
	signingKey := mustGenerateRSAKey(t)
	issuer := startJWKS(t, &signingKey.PublicKey, "org-kid")

	const orgID = "11111111-1111-1111-1111-111111111111"

	token := mustSignToken(t, signingKey, "org-kid", jwt.MapClaims{
		"sub":                "user-org",
		"email":              "admin@nullus.io",
		"preferred_username": "admin",
		"org_id":             orgID,
		"realm_access": map[string]any{
			"roles": []string{"admin"},
		},
		"iss": issuer,
		"aud": "nullus-app",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	c := e.NewContext(req, httptest.NewRecorder())

	called := false
	h := JWTAuthMiddleware(JWTConfig{IssuerURL: issuer, Audience: "nullus-app"}, keycloak.NewOIDCProvider())(func(c echo.Context) error {
		called = true
		user, ok := c.Get(userContextKey).(*admindomain.User)
		require.True(t, ok)
		require.Equal(t, orgID, user.OrgID)
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
	require.True(t, called)
}

// org_id 클레임이 없으면 빈 값이어야 한다 — 임의의 기본 조직을 지어내지 않는다.
func TestJWTAuthMiddleware_OrgIDEmptyWhenClaimMissing(t *testing.T) {
	signingKey := mustGenerateRSAKey(t)
	issuer := startJWKS(t, &signingKey.PublicKey, "noorg-kid")

	token := mustSignToken(t, signingKey, "noorg-kid", jwt.MapClaims{
		"sub":                "user-noorg",
		"email":              "admin@nullus.io",
		"preferred_username": "admin",
		"realm_access": map[string]any{
			"roles": []string{"admin"},
		},
		"iss": issuer,
		"aud": "nullus-app",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	c := e.NewContext(req, httptest.NewRecorder())

	h := JWTAuthMiddleware(JWTConfig{IssuerURL: issuer, Audience: "nullus-app"}, keycloak.NewOIDCProvider())(func(c echo.Context) error {
		user, ok := c.Get(userContextKey).(*admindomain.User)
		require.True(t, ok)
		require.Empty(t, user.OrgID)
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
}

// Keycloak 은 사용자 속성을 organization/org 등 다른 이름으로 실어 보낼 수도 있다.
func TestJWTAuthMiddleware_AcceptsOrganizationIDClaimAlias(t *testing.T) {
	signingKey := mustGenerateRSAKey(t)
	issuer := startJWKS(t, &signingKey.PublicKey, "alias-kid")

	const orgID = "22222222-2222-2222-2222-222222222222"

	token := mustSignToken(t, signingKey, "alias-kid", jwt.MapClaims{
		"sub":                "user-alias",
		"email":              "admin@nullus.io",
		"preferred_username": "admin",
		"organization_id":    orgID,
		"realm_access": map[string]any{
			"roles": []string{"admin"},
		},
		"iss": issuer,
		"aud": "nullus-app",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	c := e.NewContext(req, httptest.NewRecorder())

	h := JWTAuthMiddleware(JWTConfig{IssuerURL: issuer, Audience: "nullus-app"}, keycloak.NewOIDCProvider())(func(c echo.Context) error {
		user, ok := c.Get(userContextKey).(*admindomain.User)
		require.True(t, ok)
		require.Equal(t, orgID, user.OrgID)
		return c.NoContent(http.StatusOK)
	})

	require.NoError(t, h(c))
}
