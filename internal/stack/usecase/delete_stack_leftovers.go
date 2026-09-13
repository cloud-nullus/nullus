package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// namespaceSweepKinds 는 이름 기반 청소가 훑는 리소스 종류다.
//
// Role/RoleBinding 을 넣지 않으면 Helm 훅(eg-gateway-helm-certgen,
// argo-cd-argocd-redis-secret-init)과 OpenBao init 이 만든 RBAC 이 이름 목록에
// 있어도 목록에 오르지 않아 남는다 — kind 실측에서 스택 삭제 뒤 그대로 남았다.
const namespaceSweepKinds = "deploy,svc,cm,sa,pod,rs,sts,job,cronjob,secret,pvc,role,rolebinding"

// envoyGatewayReleaseName 은 스택이 설치하는 Envoy Gateway 릴리스다
// (helm 어댑터의 installing_gateway). 훅 리소스 이름이 여기서 파생된다.
const envoyGatewayReleaseName = "eg"

// envoyGatewayCertSecretNames 는 Envoy Gateway certgen 훅이 만드는 Secret 이다.
// 흔한 이름이라 control-plane=envoy-gateway 표시가 있을 때만 지운다.
var envoyGatewayCertSecretNames = map[string]struct{}{
	"envoy":            {},
	"envoy-gateway":    {},
	"envoy-oidc-hmac":  {},
	"envoy-rate-limit": {},
}

// openBaoBootstrapLeftovers 는 OpenBao 부트스트랩이 kubectl 로 만드는 것들이다.
// 소유 표시가 없어 고아로 남는다. 이름만으로 지우지 않고 종류까지 맞춘다 —
// 사용자가 고른 네임스페이스는 다른 것과 함께 쓰일 수 있다.
var openBaoBootstrapLeftovers = map[string]string{
	"openbao-bootstrap": "job",
	"nullus-controller": "serviceaccount",
}

// isInstallLeftoverArtifact 는 소유 표시(Helm 애노테이션·스택 라벨)가 없는 설치
// 잔여물 가운데, 이름 목록이 아니라 종류·라벨로 가려내는 것들이다.
func isInstallLeftoverArtifact(resource namespacedResource) bool {
	kind, name := splitResourceRef(resource.Ref)
	if want, ok := openBaoBootstrapLeftovers[name]; ok {
		return kind == want
	}
	if kind != "secret" {
		return false
	}
	if _, ok := envoyGatewayCertSecretNames[name]; ok {
		return resource.Labels["control-plane"] == "envoy-gateway"
	}
	// cicd 모듈이 Argo CD 네임스페이스(=스택 네임스페이스)에 만드는 저장소
	// 자격증명이다. Argo CD 가 사라지면 쓸 주체가 없다.
	return resource.Labels["app.kubernetes.io/managed-by"] == "nullus-cicd" &&
		resource.Labels["argocd.argoproj.io/secret-type"] == "repository"
}

func splitResourceRef(ref string) (kind, name string) {
	kind, name, found := strings.Cut(strings.ToLower(strings.TrimSpace(ref)), "/")
	if !found {
		return "", kind
	}
	return kind, name
}

// bestEffortDeleteEnvoyGatewayClusterHookResources 는 Envoy Gateway 차트의 Helm 훅이
// 만든 클러스터 범위 리소스를 지운다.
//
// 훅 리소스는 릴리스가 소유하지 않아 uninstall 이 지우지 않고, 클러스터 범위라
// 네임스페이스를 지워도 남는다. 이름에 설치 네임스페이스가 붙으므로 정확한
// 이름으로 지운다. 플랫폼 네임스페이스의 것은 플랫폼 것이라 건드리지 않는다.
func (uc *DeleteStack) bestEffortDeleteEnvoyGatewayClusterHookResources(
	ctx context.Context,
	kubeconfig []byte,
	stack *domain.Stack,
	stackID string,
) {
	if len(kubeconfig) == 0 || stack == nil {
		return
	}
	namespace := strings.TrimSpace(stack.Namespace)
	if namespace == "" || uc.isPlatformNamespace(namespace) {
		return
	}

	certgen := fmt.Sprintf("%s-gateway-helm-certgen:%s", envoyGatewayReleaseName, namespace)
	targets := []struct{ kind, name string }{
		{"clusterrole", certgen},
		{"clusterrolebinding", certgen},
		{"mutatingwebhookconfiguration", "envoy-gateway-topology-injector." + namespace},
	}
	for _, t := range targets {
		if _, err := uc.runKubectl(ctx, kubeconfig, "delete", t.kind, t.name, "--ignore-not-found"); err != nil {
			slog.Warn("envoy gateway cluster hook resource delete warning",
				"kind", t.kind, "name", t.name, "error", err)
			uc.emit(ctx, stackID, "deleting_manifest", "warn",
				fmt.Sprintf("%s/%s 삭제 경고: %v", t.kind, t.name, err))
		}
	}
}
