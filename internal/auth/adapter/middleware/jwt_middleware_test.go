package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestJWTAuthMiddleware_ValidJWT_SetsUserInContext(t *testing.T) {
	signingKey := mustGenerateRSAKey(t)
	issuer := startJWKS(t, &signingKey.PublicKey, "test-kid")

	token := mustSignToken(t, signingKey, "test-kid", jwt.MapClaims{
		"sub":                "user-1",
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
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	h := JWTAuthMiddleware(JWTConfig{IssuerURL: issuer, Audience: "nullus-app"})(func(c echo.Context) error {
		called = true
		user, ok := c.Get(userContextKey).(*admindomain.User)
		require.True(t, ok)
		require.NotNil(t, user)
		require.Equal(t, "user-1", user.ID)
		require.Equal(t, "admin@nullus.io", user.Email)
		require.Equal(t, "admin", user.Name)
		require.Equal(t, admindomain.RoleAdmin, user.Role)
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestJWTAuthMiddleware_ExpiredJWT_ReturnsUnauthorized(t *testing.T) {
	signingKey := mustGenerateRSAKey(t)
	issuer := startJWKS(t, &signingKey.PublicKey, "test-kid")

	token := mustSignToken(t, signingKey, "test-kid", jwt.MapClaims{
		"sub": "user-1",
		"iss": issuer,
		"aud": "nullus-app",
		"exp": time.Now().Add(-1 * time.Minute).Unix(),
		"iat": time.Now().Add(-2 * time.Minute).Unix(),
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := JWTAuthMiddleware(JWTConfig{IssuerURL: issuer, Audience: "nullus-app"})(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTAuthMiddleware_MissingAuthorizationHeader_ReturnsUnauthorized(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := JWTAuthMiddleware(JWTConfig{IssuerURL: "http://issuer.local", Audience: "nullus-app"})(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWTAuthMiddleware_InvalidSignature_ReturnsUnauthorized(t *testing.T) {
	jwksKey := mustGenerateRSAKey(t)
	issuer := startJWKS(t, &jwksKey.PublicKey, "test-kid")

	otherKey := mustGenerateRSAKey(t)
	token := mustSignToken(t, otherKey, "test-kid", jwt.MapClaims{
		"sub": "user-1",
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

	h := JWTAuthMiddleware(JWTConfig{IssuerURL: issuer, Audience: "nullus-app"})(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDualAuthMiddleware_UsesSessionWhenModeSession(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	usedSession := false
	usedOIDC := false

	sessionMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			usedSession = true
			return next(c)
		}
	}
	oidcMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			usedOIDC = true
			return next(c)
		}
	}

	h := DualAuthMiddleware("session", sessionMW, oidcMW)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	require.True(t, usedSession)
	require.False(t, usedOIDC)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestDualAuthMiddleware_UsesOIDCWhenModeOIDC(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	usedSession := false
	usedOIDC := false

	sessionMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			usedSession = true
			return next(c)
		}
	}
	oidcMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			usedOIDC = true
			return next(c)
		}
	}

	h := DualAuthMiddleware("oidc", sessionMW, oidcMW)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	require.False(t, usedSession)
	require.True(t, usedOIDC)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestDualAuthMiddleware_DefaultsToSessionForUnknownMode(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	usedSession := false
	usedOIDC := false

	sessionMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			usedSession = true
			return next(c)
		}
	}
	oidcMW := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			usedOIDC = true
			return next(c)
		}
	}

	h := DualAuthMiddleware("unknown", sessionMW, oidcMW)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	err := h(c)
	require.NoError(t, err)
	require.True(t, usedSession)
	require.False(t, usedOIDC)
	require.Equal(t, http.StatusOK, rec.Code)
}

func mustGenerateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func startJWKS(t *testing.T, pub *rsa.PublicKey, kid string) string {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/protocol/openid-connect/certs" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"` + kid + `","use":"sig","alg":"RS256","n":"` + n + `","e":"` + e + `"}]}`))
	}))
	t.Cleanup(server.Close)

	return server.URL
}

func mustSignToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}
