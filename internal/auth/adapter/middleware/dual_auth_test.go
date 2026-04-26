package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/auth/adapter/keycloak"
)

func TestDualAuth_SessionMode_ValidSession(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("X-User-Email", "dev@nullus.io")
	req.Header.Set("X-User-Role", "developer")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := DualAuthMiddleware("session", AuthMiddleware(), JWTAuthMiddleware(JWTConfig{}, keycloak.NewOIDCProvider()))(func(c echo.Context) error {
		user, ok := c.Get(userContextKey).(*admindomain.User)
		require.True(t, ok)
		require.NotNil(t, user)
		assert.Equal(t, "user-1", user.ID)
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDualAuth_SessionMode_NoSession(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := DualAuthMiddleware("session", AuthMiddleware(), JWTAuthMiddleware(JWTConfig{}, keycloak.NewOIDCProvider()))(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"authentication required"}`, rec.Body.String())
}

func TestDualAuth_OIDCMode_ValidToken(t *testing.T) {
	signingKey := mustGenerateRSAKey(t)
	issuer := startJWKS(t, &signingKey.PublicKey, "dual-auth-kid")

	token := mustSignToken(t, signingKey, "dual-auth-kid", jwt.MapClaims{
		"sub":                "oidc-user-1",
		"email":              "oidc@nullus.io",
		"preferred_username": "oidc-user",
		"realm_access": map[string]any{
			"roles": []string{"devops"},
		},
		"iss": issuer,
		"aud": "nullus-app",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := DualAuthMiddleware("oidc", AuthMiddleware(), JWTAuthMiddleware(JWTConfig{IssuerURL: issuer, Audience: "nullus-app"}, keycloak.NewOIDCProvider()))(func(c echo.Context) error {
		user, ok := c.Get(userContextKey).(*admindomain.User)
		require.True(t, ok)
		require.NotNil(t, user)
		assert.Equal(t, "oidc-user-1", user.ID)
		assert.Equal(t, admindomain.RoleDevOps, user.Role)
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDualAuth_OIDCMode_InvalidToken(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer invalid.jwt.token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := DualAuthMiddleware("oidc", AuthMiddleware(), JWTAuthMiddleware(JWTConfig{IssuerURL: "https://issuer.local", Audience: "nullus-app"}, keycloak.NewOIDCProvider()))(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"authentication required"}`, rec.Body.String())
}
