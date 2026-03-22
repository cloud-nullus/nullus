package keycloak

import "github.com/golang-jwt/jwt/v5"

type OIDCProvider struct{}

func NewOIDCProvider() *OIDCProvider { return &OIDCProvider{} }

func (p *OIDCProvider) Name() string { return "keycloak" }

func (p *OIDCProvider) ExtractRoles(claims jwt.MapClaims) []string {
	realmAccess, ok := claims["realm_access"].(map[string]any)
	if !ok {
		return nil
	}

	rawRoles, ok := realmAccess["roles"].([]any)
	if !ok {
		return nil
	}

	roles := make([]string, 0, len(rawRoles))
	for _, r := range rawRoles {
		s, ok := r.(string)
		if ok {
			roles = append(roles, s)
		}
	}

	return roles
}
