package port

import "github.com/golang-jwt/jwt/v5"

// OIDCProvider abstracts OIDC provider-specific behavior.
type OIDCProvider interface {
	// ExtractRoles extracts application roles from JWT claims.
	// Keycloak: realm_access.roles (nested object)
	// Authentik: groups (top-level flat array)
	ExtractRoles(claims jwt.MapClaims) []string

	// Name returns the provider identifier ("keycloak" or "authentik").
	Name() string
}
