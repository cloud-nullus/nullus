package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	admindomain "github.com/cloud-nullus/draft/internal/admin/domain"
	authport "github.com/cloud-nullus/draft/internal/auth/port"
)

type JWTConfig struct {
	IssuerURL string
	Audience  string
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

var (
	jwksCacheMu     sync.RWMutex
	jwksCache       = map[string]map[string]*rsa.PublicKey{}
	jwksRefreshOnce sync.Once
)

type JWTMiddleware struct {
	cfg      JWTConfig
	provider authport.OIDCProvider
}

func NewJWTMiddleware(cfg JWTConfig, provider authport.OIDCProvider) *JWTMiddleware {
	return &JWTMiddleware{cfg: cfg, provider: provider}
}

func JWTAuthMiddleware(cfg JWTConfig, provider authport.OIDCProvider) echo.MiddlewareFunc {
	return NewJWTMiddleware(cfg, provider).Handler()
}

func (m *JWTMiddleware) Handler() echo.MiddlewareFunc {
	ensureJWKSRefreshTimer()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			}

			tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if tokenString == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			}

			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
				if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
					return nil, errors.New("invalid signing method")
				}

				kid, _ := token.Header["kid"].(string)
				if kid == "" {
					return nil, errors.New("missing kid")
				}

				keys, err := getJWKS(c.Request().Context(), m.cfg.IssuerURL)
				if err != nil {
					return nil, err
				}

				key, ok := keys[kid]
				if !ok {
					return nil, errors.New("unknown key id")
				}

				return key, nil
			},
				jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
				jwt.WithIssuer(m.cfg.IssuerURL),
				jwt.WithAudience(m.cfg.Audience),
			)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			}

			user := &admindomain.User{
				ID:       claimString(claims, "sub"),
				Email:    claimString(claims, "email"),
				Name:     claimString(claims, "preferred_username"),
				Role:     m.extractRole(claims),
				IsActive: true,
			}

			c.Set(userContextKey, user)
			return next(c)
		}
	}
}

func ensureJWKSRefreshTimer() {
	jwksRefreshOnce.Do(func() {
		time.AfterFunc(time.Hour, func() {
			jwksCacheMu.Lock()
			jwksCache = map[string]map[string]*rsa.PublicKey{}
			jwksCacheMu.Unlock()
			jwksRefreshOnce = sync.Once{}
		})
	})
}

func getJWKS(ctx context.Context, issuerURL string) (map[string]*rsa.PublicKey, error) {
	jwksCacheMu.RLock()
	if keys, ok := jwksCache[issuerURL]; ok {
		jwksCacheMu.RUnlock()
		return keys, nil
	}
	jwksCacheMu.RUnlock()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimSuffix(issuerURL, "/")+"/protocol/openid-connect/certs",
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch jwks")
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" || key.Kid == "" || key.N == "" || key.E == "" {
			continue
		}

		pub, err := toRSAPublicKey(key.N, key.E)
		if err != nil {
			continue
		}

		keys[key.Kid] = pub
	}

	if len(keys) == 0 {
		return nil, errors.New("jwks has no usable keys")
	}

	jwksCacheMu.Lock()
	jwksCache[issuerURL] = keys
	jwksCacheMu.Unlock()

	return keys, nil
}

func toRSAPublicKey(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, err
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, err
	}
	if len(eBytes) == 0 {
		return nil, errors.New("empty exponent")
	}

	modulus := new(big.Int).SetBytes(nBytes)
	exponent := new(big.Int).SetBytes(eBytes).Int64()
	if exponent <= 0 {
		return nil, errors.New("invalid exponent")
	}

	return &rsa.PublicKey{
		N: modulus,
		E: int(exponent),
	}, nil
}

func claimString(claims jwt.MapClaims, key string) string {
	v, _ := claims[key].(string)
	return v
}

func (m *JWTMiddleware) extractRole(claims jwt.MapClaims) admindomain.Role {
	if m.provider == nil {
		return ""
	}

	roles := m.provider.ExtractRoles(claims)
	for _, role := range roles {
		switch role {
		case string(admindomain.RoleAdmin):
			return admindomain.RoleAdmin
		case string(admindomain.RoleDevOps):
			return admindomain.RoleDevOps
		case string(admindomain.RoleDeveloper):
			return admindomain.RoleDeveloper
		}
	}

	return ""
}
