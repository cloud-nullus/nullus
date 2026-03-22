package authentik

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestOIDCProvider_Name(t *testing.T) {
	provider := NewOIDCProvider()

	require.Equal(t, "authentik", provider.Name())
}

func TestOIDCProvider_ExtractRoles_ValidClaims(t *testing.T) {
	provider := NewOIDCProvider()
	claims := jwt.MapClaims{
		"groups": []any{"admin", "developer"},
	}

	require.Equal(t, []string{"admin", "developer"}, provider.ExtractRoles(claims))
}

func TestOIDCProvider_ExtractRoles_MissingGroups(t *testing.T) {
	provider := NewOIDCProvider()

	require.Empty(t, provider.ExtractRoles(jwt.MapClaims{}))
}

func TestOIDCProvider_ExtractRoles_MalformedGroups(t *testing.T) {
	provider := NewOIDCProvider()
	claims := jwt.MapClaims{
		"groups": "invalid",
	}

	require.Empty(t, provider.ExtractRoles(claims))
}

func TestOIDCProvider_ExtractRoles_EmptyGroups(t *testing.T) {
	provider := NewOIDCProvider()
	claims := jwt.MapClaims{
		"groups": []any{},
	}

	require.Empty(t, provider.ExtractRoles(claims))
}
