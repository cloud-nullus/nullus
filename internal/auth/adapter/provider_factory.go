package adapter

import (
	"fmt"

	"github.com/cloud-nullus/draft/internal/auth/adapter/authentik"
	"github.com/cloud-nullus/draft/internal/auth/adapter/keycloak"
	authport "github.com/cloud-nullus/draft/internal/auth/port"
)

func NewOIDCProvider(providerName string) (authport.OIDCProvider, error) {
	switch providerName {
	case "", "keycloak":
		return keycloak.NewOIDCProvider(), nil
	case "authentik":
		return authentik.NewOIDCProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported OIDC provider: %s", providerName)
	}
}
