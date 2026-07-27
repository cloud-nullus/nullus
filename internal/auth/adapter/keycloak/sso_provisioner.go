package keycloak

import (
	"context"
	"fmt"
)

const defaultAccessDomain = "nullus.local"

// ToolSSOSpec defines SSO client parameters for an OSS tool.
type ToolSSOSpec struct {
	ClientID     string
	DisplayName  string
	Subdomain    string
	CallbackPath string
}

// buildRedirectURI constructs the OIDC redirect URI for a tool.
func buildRedirectURI(subdomain, accessDomain, callbackPath string) string {
	return fmt.Sprintf("https://%s.%s%s", subdomain, accessDomain, callbackPath)
}

type SSOProvisioner struct {
	kc           *KeycloakClient
	accessDomain string
	toolSpecs    map[string]ToolSSOSpec
}

func newToolSpecs() map[string]ToolSSOSpec {
	return map[string]ToolSSOSpec{
		"installing_gitlab": {
			ClientID:     "gitlab",
			DisplayName:  "GitLab CE",
			Subdomain:    "gitlab",
			CallbackPath: "/users/auth/openid_connect/callback",
		},
		"installing_grafana": {
			ClientID:     "grafana",
			DisplayName:  "Grafana",
			Subdomain:    "grafana",
			CallbackPath: "/login/generic_oauth",
		},
		"installing_argocd": {
			ClientID:     "argocd",
			DisplayName:  "Argo CD",
			Subdomain:    "argocd",
			CallbackPath: "/auth/callback",
		},
		"installing_harbor": {
			ClientID:     "harbor",
			DisplayName:  "Harbor",
			Subdomain:    "harbor",
			CallbackPath: "/c/oidc/callback",
		},
		"installing_minio": {
			ClientID:     "minio",
			DisplayName:  "MinIO",
			Subdomain:    "minio",
			CallbackPath: "/oauth_callback",
		},
	}
}

// NewSSOProvisioner creates a provisioner using the default access domain ("nullus.local").
// Preserves backward-compatible signature.
func NewSSOProvisioner(kc *KeycloakClient) *SSOProvisioner {
	return NewSSOProvisionerWithDomain(kc, defaultAccessDomain)
}

// NewSSOProvisionerWithDomain creates a provisioner with a custom access domain.
// redirect URIs are built as https://{subdomain}.{accessDomain}{callbackPath}.
func NewSSOProvisionerWithDomain(kc *KeycloakClient, accessDomain string) *SSOProvisioner {
	if accessDomain == "" {
		accessDomain = defaultAccessDomain
	}
	return &SSOProvisioner{
		kc:           kc,
		accessDomain: accessDomain,
		toolSpecs:    newToolSpecs(),
	}
}

func (p *SSOProvisioner) ProvisionSSO(ctx context.Context, stepName string) error {
	spec, ok := p.toolSpecs[stepName]
	if !ok {
		return fmt.Errorf("unknown SSO tool: %s", stepName)
	}
	redirectURI := buildRedirectURI(spec.Subdomain, p.accessDomain, spec.CallbackPath)
	return p.kc.RegisterOIDCClient(ctx, spec.ClientID, []string{redirectURI}, spec.DisplayName)
}

func (p *SSOProvisioner) DeprovisionSSO(ctx context.Context, stepName string) error {
	spec, ok := p.toolSpecs[stepName]
	if !ok {
		return nil
	}
	return p.kc.DeleteOIDCClient(ctx, spec.ClientID)
}
