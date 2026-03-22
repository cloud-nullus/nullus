package keycloak

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestOIDCProvider_Name(t *testing.T) {
	provider := NewOIDCProvider()

	require.Equal(t, "keycloak", provider.Name())
}

func TestOIDCProvider_ExtractRoles_ValidClaims(t *testing.T) {
	provider := NewOIDCProvider()
	claims := jwt.MapClaims{
		"realm_access": map[string]any{
			"roles": []any{"admin", "devops"},
		},
	}

	require.Equal(t, []string{"admin", "devops"}, provider.ExtractRoles(claims))
}

func TestOIDCProvider_ExtractRoles_MissingRealmAccess(t *testing.T) {
	provider := NewOIDCProvider()

	require.Empty(t, provider.ExtractRoles(jwt.MapClaims{}))
}

func TestOIDCProvider_ExtractRoles_MalformedRealmAccess(t *testing.T) {
	provider := NewOIDCProvider()
	claims := jwt.MapClaims{
		"realm_access": "invalid",
	}

	require.Empty(t, provider.ExtractRoles(claims))
}

func TestOIDCProvider_ExtractRoles_EmptyRoles(t *testing.T) {
	provider := NewOIDCProvider()
	claims := jwt.MapClaims{
		"realm_access": map[string]any{
			"roles": []any{},
		},
	}

	require.Empty(t, provider.ExtractRoles(claims))
}
