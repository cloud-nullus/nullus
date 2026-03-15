package keycloak

import "context"

type SSOProvisioner struct {
	kc        *KeycloakClient
	toolSpecs map[string]ToolSSOSpec
}

type ToolSSOSpec struct {
	ClientID     string
	RedirectURIs []string
	DisplayName  string
}

func NewSSOProvisioner(kc *KeycloakClient) *SSOProvisioner {
	return &SSOProvisioner{
		kc: kc,
		toolSpecs: map[string]ToolSSOSpec{
			"installing_gitlab": {
				ClientID:     "gitlab",
				RedirectURIs: []string{"https://gitlab.nullus.local/users/auth/openid_connect/callback"},
				DisplayName:  "GitLab CE",
			},
			"installing_grafana": {
				ClientID:     "grafana",
				RedirectURIs: []string{"https://grafana.nullus.local/login/generic_oauth"},
				DisplayName:  "Grafana",
			},
			"installing_argocd": {
				ClientID:     "argocd",
				RedirectURIs: []string{"https://argocd.nullus.local/auth/callback"},
				DisplayName:  "Argo CD",
			},
			"installing_minio": {
				ClientID:     "minio",
				RedirectURIs: []string{"https://minio.nullus.local/oauth_callback"},
				DisplayName:  "MinIO",
			},
		},
	}
}

func (p *SSOProvisioner) ProvisionSSO(ctx context.Context, stepName string) error {
	spec, ok := p.toolSpecs[stepName]
	if !ok {
		return nil
	}
	return p.kc.RegisterOIDCClient(ctx, spec.ClientID, spec.RedirectURIs, spec.DisplayName)
}

func (p *SSOProvisioner) DeprovisionSSO(ctx context.Context, stepName string) error {
	spec, ok := p.toolSpecs[stepName]
	if !ok {
		return nil
	}
	return p.kc.DeleteOIDCClient(ctx, spec.ClientID)
}
