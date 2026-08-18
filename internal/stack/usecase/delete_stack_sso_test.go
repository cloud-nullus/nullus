package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 설치가 Keycloak 에 만든 OIDC 클라이언트는 스택을 지워도 realm 에 그대로 남았다.
// DeprovisionSSO 구현은 있었지만 포트 인터페이스에 없어 stack 모듈이 부를 방법이
// 없었다. PVC·Gateway 누수와 같은 모양이고, 남은 클라이언트는 존재하지 않는
// 도구를 가리키는 redirect URI 를 realm 에 계속 들고 있게 된다.

type fakeSSOProvisioner struct {
	mu            sync.Mutex
	slug          string
	deprovisioned []string
	err           error
}

func (f *fakeSSOProvisioner) ClientIDFor(step string) (string, bool) {
	return f.slug + "-" + step, true
}

func (f *fakeSSOProvisioner) ToolSteps() []string {
	return []string{"installing_argocd", "installing_harbor"}
}

func (f *fakeSSOProvisioner) Provision(context.Context, port.SSOClientSpec) error { return nil }

func (f *fakeSSOProvisioner) Deprovision(_ context.Context, step string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deprovisioned = append(f.deprovisioned, step)
	return f.err
}

func newSSODeleteStack(t *testing.T, stack *domain.Stack, prov *fakeSSOProvisioner) (*DeleteStack, *string) {
	t.Helper()
	uc := newGatewayDeleteStack(t, newKubectlRecorder(), stack)
	var gotDomain string
	uc.SetSSOProvisionerFactory(func(accessDomain, slug string) port.SSOProvisioner {
		gotDomain = accessDomain
		prov.slug = slug
		return prov
	})
	return uc, &gotDomain
}

func ssoStack() *domain.Stack {
	return &domain.Stack{
		ID:        "stk-sso",
		Name:      "ssowire",
		ClusterID: "cluster-sso",
		Namespace: "ssowire",
		State:     domain.StateCompleted,
		Config:    map[string]any{"access_domain": "nullus.local"},
	}
}

func TestDeleteStack_DeprovisionsSSOClients(t *testing.T) {
	prov := &fakeSSOProvisioner{}
	uc, gotDomain := newSSODeleteStack(t, ssoStack(), prov)

	require.NoError(t, uc.Execute(context.Background(), "stk-sso"))

	assert.ElementsMatch(t, []string{"installing_argocd", "installing_harbor"}, prov.deprovisioned,
		"스택을 지웠는데 Keycloak 클라이언트가 남으면 realm 에 죽은 redirect URI 가 쌓인다")
	assert.Equal(t, "nullus.local", *gotDomain)
	assert.Equal(t, "ssowire", prov.slug, "설치 때와 같은 슬러그여야 같은 client ID 를 지운다")
}

// Keycloak 이 안 떠 있어도 스택 삭제 자체는 끝나야 한다. 삭제 경로의 다른 정리와
// 같은 best-effort 원칙이다 — 여기서 멈추면 클러스터 리소스가 통째로 남는다.
func TestDeleteStack_SSODeprovisionFailureDoesNotBlockDelete(t *testing.T) {
	prov := &fakeSSOProvisioner{err: errors.New("keycloak unreachable")}
	uc, _ := newSSODeleteStack(t, ssoStack(), prov)

	assert.NoError(t, uc.Execute(context.Background(), "stk-sso"))
}

// SSO 프로비저닝을 안 쓰는 설치(BYO IdP / 미사용)는 팩토리가 없다.
func TestDeleteStack_WithoutSSOFactoryStillDeletes(t *testing.T) {
	uc := newGatewayDeleteStack(t, newKubectlRecorder(), ssoStack())
	assert.NoError(t, uc.Execute(context.Background(), "stk-sso"))
}
