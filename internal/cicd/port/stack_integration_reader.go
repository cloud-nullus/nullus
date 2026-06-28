package port

import "context"

// StackIntegration holds connection info for one deployed component in a Stack.
type StackIntegration struct {
	ComponentType string // code_repository, image_registry, ci_platform, cd_tool
	Provider      string // gitlab, argocd, harbor, etc.
	Endpoint      string // HTTP base URL
	Token         string // credential — read from stack config at runtime
}

// StackIntegrationProfile is the full connection profile for a Stack's CI/CD components.
type StackIntegrationProfile struct {
	StackID     string
	OrgID       string
	ClusterID   string
	State       string
	Integrations []StackIntegration
}

func (p *StackIntegrationProfile) ByType(componentType string) *StackIntegration {
	for i := range p.Integrations {
		if p.Integrations[i].ComponentType == componentType {
			return &p.Integrations[i]
		}
	}
	return nil
}

// StackIntegrationReader is the Port the CI/CD module uses to read Stack connection info.
// The CI/CD module never imports stack/domain directly.
type StackIntegrationReader interface {
	GetStackIntegrationProfile(ctx context.Context, stackID string) (*StackIntegrationProfile, error)
}
