package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func leftoverStack() *domain.Stack {
	return &domain.Stack{
		ID:        "stk-leftover",
		Name:      "scan-e2e",
		ClusterID: "cluster-leftover",
		Namespace: "scan-e2e",
		State:     domain.StateCompleted,
	}
}

// Envoy Gateway 차트의 Helm 훅은 클러스터 범위 리소스를 만든다. 훅 리소스는 릴리스가
// 소유하지 않아 uninstall 이 지우지 않고, 네임스페이스를 지워도 사라지지 않는다 —
// kind 실측에서 스택 삭제 뒤 ClusterRole·ClusterRoleBinding·MutatingWebhookConfiguration
// 이 남았다. 이름에 네임스페이스가 붙어 있으므로 정확한 이름으로 지운다.
func TestDeleteStack_DeletesEnvoyGatewayClusterHookResources(t *testing.T) {
	rec := newKubectlRecorder()
	uc := newGatewayDeleteStack(t, rec, leftoverStack())

	require.NoError(t, uc.Execute(context.Background(), "stk-leftover"))

	for _, want := range []string{
		"delete clusterrole eg-gateway-helm-certgen:scan-e2e --ignore-not-found",
		"delete clusterrolebinding eg-gateway-helm-certgen:scan-e2e --ignore-not-found",
		"delete mutatingwebhookconfiguration envoy-gateway-topology-injector.scan-e2e --ignore-not-found",
	} {
		assert.True(t, rec.has(want), "지우지 않았다: %s", want)
	}
}

// 플랫폼이 사는 네임스페이스의 게이트웨이 훅 리소스는 플랫폼 것이다.
func TestDeleteStack_KeepsClusterHookResourcesOfPlatformNamespace(t *testing.T) {
	rec := newKubectlRecorder()
	stack := leftoverStack()
	stack.Namespace = "nullus-system"
	uc := newGatewayDeleteStack(t, rec, stack)
	uc.SetPlatformNamespace("nullus-system")

	require.NoError(t, uc.Execute(context.Background(), "stk-leftover"))

	assert.False(t, rec.has("eg-gateway-helm-certgen:nullus-system"))
	assert.False(t, rec.has("envoy-gateway-topology-injector.nullus-system"))
}

// 사용자가 고른 네임스페이스는 통째로 회수하지 않는다. 그 안의 설치 잔여물은
// 하나씩 지워야 한다 — 실측에서 Helm 훅 Role/RoleBinding, OpenBao 부트스트랩 잡과
// 서비스 계정, Envoy Gateway 인증서 Secret, Argo CD 저장소 Secret 이 남았다.
func TestDeleteStack_CleansInstallHookAndBootstrapLeftovers(t *testing.T) {
	uc, deleted := newOwnershipDeleteStack(t, leftoverStack())
	envoy := map[string]string{"control-plane": "envoy-gateway"}
	argoRepo := map[string]string{
		"app.kubernetes.io/managed-by":   "nullus-cicd",
		"argocd.argoproj.io/secret-type": "repository",
	}
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "scan-e2e" {
			return nil, nil
		}
		return []namespacedResource{
			{Ref: "role/eg-gateway-helm-certgen"},
			{Ref: "rolebinding/eg-gateway-helm-certgen"},
			{Ref: "role/argo-cd-argocd-redis-secret-init"},
			{Ref: "rolebinding/argo-cd-argocd-redis-secret-init"},
			{Ref: "role/openbao-init"},
			{Ref: "rolebinding/openbao-init"},
			{Ref: "job/openbao-bootstrap"},
			{Ref: "serviceaccount/nullus-controller"},
			{Ref: "secret/envoy", Labels: envoy},
			{Ref: "secret/envoy-gateway", Labels: envoy},
			{Ref: "secret/envoy-oidc-hmac", Labels: envoy},
			{Ref: "secret/envoy-rate-limit", Labels: envoy},
			{Ref: "secret/nullus-repo-scan-app", Labels: argoRepo},

			// 같은 네임스페이스를 쓰는 남의 것 — 지우면 안 된다.
			{Ref: "secret/app-db", Labels: envoy},
			{Ref: "secret/team-repo", Labels: map[string]string{"argocd.argoproj.io/secret-type": "repository"}},
			{Ref: "serviceaccount/default"},
			{Ref: "configmap/kube-root-ca.crt"},
		}, nil
	}

	require.NoError(t, uc.Execute(context.Background(), "stk-leftover"))

	for _, ref := range []string{
		"role/eg-gateway-helm-certgen", "rolebinding/eg-gateway-helm-certgen",
		"role/argo-cd-argocd-redis-secret-init", "rolebinding/argo-cd-argocd-redis-secret-init",
		"role/openbao-init", "rolebinding/openbao-init",
		"job/openbao-bootstrap", "serviceaccount/nullus-controller",
		"secret/envoy", "secret/envoy-gateway", "secret/envoy-oidc-hmac", "secret/envoy-rate-limit",
		"secret/nullus-repo-scan-app",
	} {
		assert.Contains(t, *deleted, "scan-e2e:"+ref)
	}
	for _, ref := range []string{
		"secret/app-db", "secret/team-repo", "serviceaccount/default", "configmap/kube-root-ca.crt",
	} {
		assert.NotContains(t, *deleted, "scan-e2e:"+ref)
	}
}

// "envoy" 는 흔한 이름이다. Envoy Gateway 가 만든 표시가 없으면 남의 것으로 본다.
func TestDeleteStack_KeepsEnvoyNamedSecretWithoutGatewayLabel(t *testing.T) {
	uc, deleted := newOwnershipDeleteStack(t, leftoverStack())
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "scan-e2e" {
			return nil, nil
		}
		return []namespacedResource{{Ref: "secret/envoy"}}, nil
	}

	require.NoError(t, uc.Execute(context.Background(), "stk-leftover"))
	assert.NotContains(t, *deleted, "scan-e2e:secret/envoy")
}

// Role/RoleBinding 을 목록에 넣지 않으면 이름 목록에 있어도 지워지지 않는다 —
// 실측에서 eg-gateway-helm-certgen·openbao-init 이 이름 목록에 있었는데도 남았다.
func TestNamespaceSweepKinds_IncludeRBAC(t *testing.T) {
	kinds := strings.Split(namespaceSweepKinds, ",")
	assert.Contains(t, kinds, "role")
	assert.Contains(t, kinds, "rolebinding")
}
