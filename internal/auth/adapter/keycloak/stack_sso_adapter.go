package keycloak

import (
	"context"

	stackport "github.com/cloud-nullus/draft/internal/stack/port"
)

// StackSSOAdapter 는 SSOProvisioner 를 stack 모듈의 포트에 맞춘다.
//
// stack 모듈은 이 어댑터의 구체 타입을 알지 못하고 포트 인터페이스만 본다.
type StackSSOAdapter struct {
	inner *SSOProvisioner
}

func NewStackSSOAdapter(inner *SSOProvisioner) *StackSSOAdapter {
	return &StackSSOAdapter{inner: inner}
}

func (a *StackSSOAdapter) ClientIDFor(stepName string) (string, bool) {
	if a == nil || a.inner == nil {
		return "", false
	}
	return a.inner.ClientIDFor(stepName)
}

func (a *StackSSOAdapter) ToolSteps() []string {
	if a == nil || a.inner == nil {
		return nil
	}
	return a.inner.ToolSteps()
}

func (a *StackSSOAdapter) Provision(ctx context.Context, spec stackport.SSOClientSpec) error {
	if a == nil || a.inner == nil {
		return nil
	}
	return a.inner.ProvisionSSO(ctx, spec.StepName, spec.ClientSecret)
}

func (a *StackSSOAdapter) Deprovision(ctx context.Context, stepName string) error {
	if a == nil || a.inner == nil {
		return nil
	}
	return a.inner.DeprovisionSSO(ctx, stepName)
}

// NewStackSSOFactory 는 스택별 provisioner 를 만드는 팩토리를 돌려준다.
func NewStackSSOFactory(kc *KeycloakClient) stackport.SSOProvisionerFactory {
	return func(accessDomain, stackSlug string) stackport.SSOProvisioner {
		if kc == nil {
			return nil
		}
		return NewStackSSOAdapter(
			NewSSOProvisionerWithDomain(kc, accessDomain).WithStackSlug(stackSlug),
		)
	}
}
