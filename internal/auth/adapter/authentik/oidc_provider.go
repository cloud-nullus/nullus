package authentik

import "github.com/golang-jwt/jwt/v5"

type OIDCProvider struct{}

func NewOIDCProvider() *OIDCProvider { return &OIDCProvider{} }

func (p *OIDCProvider) Name() string { return "authentik" }

func (p *OIDCProvider) ExtractRoles(claims jwt.MapClaims) []string {
	rawGroups, ok := claims["groups"].([]any)
	if !ok {
		return nil
	}

	groups := make([]string, 0, len(rawGroups))
	for _, g := range rawGroups {
		s, ok := g.(string)
		if ok {
			groups = append(groups, s)
		}
	}

	return groups
}
