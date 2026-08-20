package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// stackHelmReleaseNames 는 삭제가 훑는 릴리스 목록이다.
//
// 설치 목록과 따로 관리하면 반드시 어긋나므로 domain 의 단일 출처를 쓴다.
// (external-secrets 가 빠져 있어 ESO 의 CRD·ClusterRole 이 고아로 남던 문제,
// harbor·nexus 가 빠져 있어 레지스트리 파드와 PVC 가 남던 문제)
var stackHelmReleaseNames = domain.AllHelmReleaseNames()

const legacyEnvoyGatewayNamespace = "envoy-gateway-system"

const stackNameLabelKey = "nullus.io/stack-name"

var orphanTempoResourceNames = map[string]struct{}{
	"tempo":        {},
	"tempo-svc":    {},
	"tempo-config": {},
}

var legacyReleaseArtifactExactNames = map[string]struct{}{
	"argo-cd-argocd-redis-secret-init":                      {},
	"argocd-initial-admin-secret":                           {},
	"argocd-redis":                                          {},
	"eg-gateway-helm-certgen":                               {},
	"data-nullus-postgresql-0":                              {},
	"opensearch-cluster-master-opensearch-cluster-master-0": {},
	"redis-data-gitlab-redis-master-0":                      {},
	"repo-data-gitlab-gitaly-0":                             {},
	// OpenBao 금고 데이터와 unseal key. 반드시 함께 삭제해야 한다 —
	// 한쪽만 남으면 재설치 시 init 이 "이미 초기화됨"으로 건너뛰는데
	// 금고를 열 키가 없어 영구 봉인 상태가 된다.
	"data-openbao-0":      {},
	"openbao-unseal-keys": {},
	"openbao-init":        {},

	// 프로비저닝 단계가 Helm 이 아니라 kubectl 로 만드는 것들. 소유자 표시가
	// 없어 고아로 남으므로 이름으로 지운다. 접두사로 뭉뚱그리지 않고 하나씩
	// 적는다 — 이 네임스페이스를 플랫폼과 공유할 수 있기 때문이다.
	domain.ProvisionedPostgresSecret:        {},
	domain.ProvisionedMinIOSecret:           {},
	domain.ProvisionedObjectStorageSecret:   {},
	domain.ProvisionedRegistryStorageSecret: {},
	domain.HarborAdminSecret:                {},
	domain.NexusAdminSecret:                 {},
	domain.GiteaAdminSecret:                 {},
	domain.JenkinsAdminSecret:               {},
	domain.AccessDomainTLSSecretName:        {},
	domain.AccessDomainCertName:             {},
}

// stackHelmReleaseNameSet 은 소유권 판정을 위한 조회용 집합이다.
var stackHelmReleaseNameSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(stackHelmReleaseNames))
	for _, name := range stackHelmReleaseNames {
		set[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	return set
}()

func isStackHelmRelease(name string) bool {
	_, ok := stackHelmReleaseNameSet[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// namespacedResource 는 청소 후보 한 건과 그 소유자 표시다.
//
// 이름만 보고 지우면 같은 네임스페이스를 쓰는 남의 리소스를 지운다. 쿠버네티스는
// 이미 소유자를 적어 두고 있다 — Helm 은 meta.helm.sh/release-name 애노테이션을,
// 이 플랫폼은 nullus.io/stack-name 라벨을 붙인다. 그것을 읽어서 판단한다.
type namespacedResource struct {
	// Ref 는 kubectl 이 그대로 받는 형태다 (예: "deployment/nullus-api").
	Ref string
	// HelmRelease 는 meta.helm.sh/release-name 애노테이션이다. 비어 있으면 고아다.
	HelmRelease string
	// StackLabel 은 nullus.io/stack-name 라벨이다.
	StackLabel string
}

var legacyReleaseArtifactPrefixes = []string{
	"gitlab-",
	"argo-cd-",
	"argocd-",
	// "nullus-" 접두사는 뺐다. 스택이 플랫폼과 같은 네임스페이스에 깔리면 이
	// 접두사가 플랫폼 자신의 리소스(nullus-api / nullus-web / nullus-keycloak)를
	// 전부 삼킨다 — 2026-08-20 에 실제로 그렇게 nullus.io 가 통째로 지워졌다.
	// 스택이 만드는 nullus-* 리소스는 이름을 알고 있으므로 아래 exact 목록에 적는다.
	"opensearch-",
	"tempo-",
	"loki-",
	"grafana-",
	"prometheus-",
	"kube-prometheus-",
	"postgresql-",
	"data-nullus-postgresql-",
	"redis-data-gitlab-",
	"repo-data-gitlab-",
}

// argoCDCRDNames 는 Argo CD 차트가 만드는 CRD 다.
//
// helm uninstall 은 CRD 를 지우지 않는다. 남으면 소유 애노테이션이 삭제된
// 릴리스를 가리킨 채로 있어, 같은 클러스터에 다음 스택을 설치할 때
// "invalid ownership metadata" 로 Argo CD 설치가 막힌다.
var argoCDCRDNames = []string{
	"applications.argoproj.io",
	"applicationsets.argoproj.io",
	"appprojects.argoproj.io",
}

var gatewayCRDNames = []string{
	"gatewayclasses.gateway.networking.k8s.io",
	"gateways.gateway.networking.k8s.io",
	"httproutes.gateway.networking.k8s.io",
	"grpcroutes.gateway.networking.k8s.io",
	"referencegrants.gateway.networking.k8s.io",
	"tcproutes.gateway.networking.k8s.io",
	"tlsroutes.gateway.networking.k8s.io",
	"udproutes.gateway.networking.k8s.io",
}

// 스택 네임스페이스에서 지워야 할 Gateway API 리소스.
//
// gatewayclasses 는 없다 — 클러스터 스코프이고 다른 스택과 공유한다.
var namespacedGatewayAPIResources = []string{
	"httproutes.gateway.networking.k8s.io",
	"grpcroutes.gateway.networking.k8s.io",
	"tcproutes.gateway.networking.k8s.io",
	"tlsroutes.gateway.networking.k8s.io",
	"udproutes.gateway.networking.k8s.io",
	"referencegrants.gateway.networking.k8s.io",
	// Gateway 는 마지막이다. 라우트를 먼저 떼어내야 컨트롤러가 게이트웨이를
	// 정리하는 동안 참조가 남지 않는다.
	"gateways.gateway.networking.k8s.io",
}

type DeleteStack struct {
	stackRepo           port.StackRepository
	kubeconfigProvider  port.KubeconfigProvider
	executorFactoryFunc func(kubeconfig []byte) port.HelmInstaller
	streamer            port.LogStreamer
	deleteManifestFunc  func(ctx context.Context, kubeconfig []byte, namespace, manifest string) error
	listResourcesFunc   func(ctx context.Context, kubeconfig []byte, namespace string) ([]namespacedResource, error)
	// platformNamespace 는 플랫폼 자신이 사는 네임스페이스다. 비어 있으면 모른다는
	// 뜻이고(클러스터 밖에서 도는 개발 환경), 알고 있으면 그 안에서는 이름 기반
	// 청소를 아예 하지 않는다.
	platformNamespace  string
	deleteResourceFunc func(ctx context.Context, kubeconfig []byte, namespace, resource string) error
	runKubectlFunc     func(ctx context.Context, kubeconfig []byte, args ...string) (string, error)
	// ssoFactory 는 설치 때 OIDC 클라이언트를 만든 provisioner 를 다시 만든다.
	// 없으면 SSO 프로비저닝을 안 쓰는 설치다(BYO IdP / 미사용).
	ssoFactory port.SSOProvisionerFactory
}

// SetSSOProvisionerFactory 는 SSO provisioner 생성기를 주입한다.
//
// 설치 때와 같은 팩토리여야 같은 client ID 를 계산해 지울 수 있다.
func (uc *DeleteStack) SetSSOProvisionerFactory(factory port.SSOProvisionerFactory) {
	uc.ssoFactory = factory
}

// runKubectl 은 삭제 경로의 모든 kubectl 호출이 지나는 한 지점이다. 테스트가
// 실제 클러스터 없이 "무엇을 지우려 했는가" 를 검사할 수 있어야 하므로 주입한다.
func (uc *DeleteStack) runKubectl(ctx context.Context, kubeconfig []byte, args ...string) (string, error) {
	if uc.runKubectlFunc != nil {
		return uc.runKubectlFunc(ctx, kubeconfig, args...)
	}
	return runKubectlWithKubeconfig(ctx, kubeconfig, args...)
}

func NewDeleteStack(
	stackRepo port.StackRepository,
	kubeconfigProvider port.KubeconfigProvider,
	executorFactory func(kubeconfig []byte) port.HelmInstaller,
	streamer ...port.LogStreamer,
) *DeleteStack {
	var logStreamer port.LogStreamer
	if len(streamer) > 0 {
		logStreamer = streamer[0]
	}

	return &DeleteStack{
		stackRepo:           stackRepo,
		kubeconfigProvider:  kubeconfigProvider,
		executorFactoryFunc: executorFactory,
		streamer:            logStreamer,
		deleteManifestFunc:  deleteManifest,
		listResourcesFunc:   listNamespaceResources,
		deleteResourceFunc:  deleteResource,
	}
}

func (uc *DeleteStack) Execute(ctx context.Context, stackID string) error {
	stack, err := uc.stackRepo.GetByID(ctx, stackID)
	if err != nil {
		if isStackNotFoundError(err) {
			uc.emit(ctx, stackID, "delete_failed", "error", "stack not found")
			return fmt.Errorf("%w: %s", ErrStackNotFound, stackID)
		}
		uc.emit(ctx, stackID, "delete_failed", "error", err.Error())
		return fmt.Errorf("get stack: %w", err)
	}
	if stack == nil {
		uc.emit(ctx, stackID, "delete_failed", "error", "stack not found")
		return fmt.Errorf("%w: %s", ErrStackNotFound, stackID)
	}

	uc.emit(ctx, stackID, "deleting_started", "info", "stack delete started")

	stack.State = domain.StateCancelled
	stack.UpdatedAt = time.Now()
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		uc.emit(ctx, stackID, "delete_failed", "error", err.Error())
		return fmt.Errorf("mark stack canceled: %w", err)
	}
	if err := uc.stackRepo.Delete(ctx, stackID); err != nil {
		uc.emit(ctx, stackID, "delete_failed", "error", err.Error())
		return fmt.Errorf("mark stack deleted: %w", err)
	}

	kubeconfig := uc.loadKubeconfig(ctx, stack.ClusterID)
	gatewayNames := uc.collectGatewayNames(ctx, kubeconfig, stack)
	gatewayNames = uc.mergeGatewayNames(gatewayNames, uc.collectGatewayNamesFromManagedResources(ctx, kubeconfig, stack))
	uc.bestEffortDeleteYAMLResources(ctx, kubeconfig, stack, stackID)
	// ESO 커스텀 리소스를 오퍼레이터보다 먼저 지운다.
	// 순서가 뒤바뀌면 ExternalSecret 의 finalizer 를 처리할 컨트롤러가 사라져
	// 네임스페이스와 CRD 가 영구 Terminating 상태로 남는다.
	uc.bestEffortDeleteExternalSecretResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortUninstall(ctx, kubeconfig, stack.Namespace, stackID)
	uc.bestEffortDeleteYAMLResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteStackLabeledResources(ctx, kubeconfig, stack, stackID)
	// Gateway 를 먼저 지운다. 순서가 뒤바뀌면 컨트롤러가 살아 있는 Gateway 를 보고
	// 방금 지운 데이터플레인 Deployment 를 복구한다.
	uc.bestEffortDeleteGatewayAPIResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteGatewayManagedResources(ctx, kubeconfig, stack, gatewayNames, stackID)
	uc.bestEffortDeleteLegacyMonitoringResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteLegacyGatewayPolicyResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteLegacyReleaseArtifacts(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteOrphanGatewayTempoResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteGatewayCRDs(ctx, kubeconfig, stackID)
	uc.bestEffortDeleteArgoCDCRDs(ctx, kubeconfig, stackID)
	uc.bestEffortDeleteStackLabeledResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteLegacyGatewayPolicyResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteLegacyReleaseArtifacts(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteOrphanGatewayTempoResources(ctx, kubeconfig, stack, stackID)
	// PVC 는 마지막이다. 릴리스와 StatefulSet 이 아직 살아 있는 동안 지우면
	// 컨트롤러가 곧바로 다시 만든다.
	uc.bestEffortDeletePersistentVolumeClaims(ctx, kubeconfig, stack, stackID)
	// Keycloak 정리는 클러스터 밖이라 kubeconfig 와 무관하다. 클러스터 리소스를
	// 다 치운 뒤에 한다 — Keycloak 이 안 떠 있어도 삭제는 끝나야 하기 때문이다.
	uc.bestEffortDeleteAccessDomainCertificate(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeprovisionSSO(ctx, stack, stackID)

	uc.emit(ctx, stackID, "deleted", "info", "stack delete completed")
	uc.clearStreamHistory(stackID)

	return nil
}

// bestEffortDeleteAccessDomainCertificate 는 게이트웨이 HTTPS 리스너용 와일드카드
// 인증서를 지운다.
//
// 라벨 정리(bestEffortDeleteStackLabeledResources)로는 잡히지 않는다 — 목록에
// certificate 가 없고, 이 매니페스트의 라벨은 stack.Name 이 아니라 접속 도메인에서
// 오기 때문이다. cert-manager 는 Certificate 를 지워도 TLS 시크릿을 남기므로
// 시크릿도 함께 지운다.
func (uc *DeleteStack) bestEffortDeleteAccessDomainCertificate(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil {
		return
	}
	namespace := strings.TrimSpace(stack.Namespace)
	if namespace == "" {
		return
	}

	targets := []struct{ kind, name string }{
		{"certificate.cert-manager.io", domain.AccessDomainCertName},
		{"secret", domain.AccessDomainTLSSecretName},
	}
	for _, t := range targets {
		if _, err := uc.runKubectl(ctx, kubeconfig, "delete", t.kind, t.name, "-n", namespace, "--ignore-not-found"); err != nil {
			slog.Warn("access-domain certificate delete warning", "kind", t.kind, "name", t.name, "namespace", namespace, "error", err)
			uc.emit(ctx, stackID, "deleting", "warn",
				fmt.Sprintf("접속 도메인 %s(%s) 삭제 경고: %v", t.kind, t.name, err))
		}
	}
}

// bestEffortDeprovisionSSO 는 설치가 IdP 에 등록한 OIDC 클라이언트를 지운다.
//
// 실패해도 삭제를 멈추지 않는다. 여기서 멈추면 IdP 가 잠깐 안 떠 있다는 이유로
// 클러스터 리소스가 통째로 남는다 — 훨씬 비싼 누수다.
func (uc *DeleteStack) bestEffortDeprovisionSSO(ctx context.Context, stack *domain.Stack, stackID string) {
	if uc.ssoFactory == nil {
		return
	}
	accessDomain := ""
	if cfg, ok := extractStackConfig(stack.Config); ok {
		accessDomain = strings.TrimSpace(cfg.AccessDomain)
	}
	// 슬러그는 설치 때와 같아야 한다 — 오케스트레이터도 네임스페이스를 쓴다.
	provisioner := uc.ssoFactory(accessDomain, strings.TrimSpace(stack.Namespace))
	if provisioner == nil {
		return
	}
	for _, step := range provisioner.ToolSteps() {
		if err := provisioner.Deprovision(ctx, step); err != nil {
			clientID, _ := provisioner.ClientIDFor(step)
			uc.emit(ctx, stackID, "deleting", "warn",
				fmt.Sprintf("OIDC 클라이언트 삭제 실패 (%s): %v", clientID, err))
		}
	}
}

func (uc *DeleteStack) markDeleteFailedState(ctx context.Context, stack *domain.Stack, stackID string) {
	if stack == nil {
		return
	}
	stack.State = domain.StateFailed
	stack.UpdatedAt = time.Now()
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		slog.Warn("failed to mark stack failed after delete error", "stack_id", stackID, "error", err)
	}
}

type historyClearer interface {
	ClearHistory(deploymentID string)
}

func (uc *DeleteStack) clearStreamHistory(stackID string) {
	if stackID == "" || uc.streamer == nil {
		return
	}
	if clearer, ok := uc.streamer.(historyClearer); ok {
		clearer.ClearHistory(stackID)
	}
}

func (uc *DeleteStack) loadKubeconfig(ctx context.Context, clusterID string) []byte {
	if uc.kubeconfigProvider == nil || clusterID == "" {
		return nil
	}

	kubeconfig, err := uc.kubeconfigProvider.GetKubeconfig(ctx, clusterID)
	if err != nil {
		slog.Warn("stack delete continues without kubeconfig", "cluster_id", clusterID, "error", err)
		return nil
	}
	return kubeconfig
}

func (uc *DeleteStack) bestEffortUninstall(ctx context.Context, kubeconfig []byte, namespace, stackID string) {
	if uc.executorFactoryFunc == nil || len(kubeconfig) == 0 || namespace == "" {
		return
	}

	installer := uc.executorFactoryFunc(kubeconfig)
	if installer == nil {
		return
	}

	for _, releaseName := range stackHelmReleaseNames {
		if isSharedInfraRelease(releaseName) {
			uc.emit(ctx, stackID, "deleting_release", "info",
				fmt.Sprintf("%s 는 스택들이 함께 쓰므로 남겨 둡니다", releaseName))
			continue
		}
		namespaces := uninstallNamespacesForRelease(namespace, releaseName)
		for _, targetNamespace := range namespaces {
			uc.emit(ctx, stackID, "deleting_release", "info", fmt.Sprintf("uninstalling release %s in namespace %s", releaseName, targetNamespace))
			if err := installer.Uninstall(ctx, releaseName, targetNamespace); err != nil {
				slog.Warn("helm uninstall failed during stack delete", "release", releaseName, "namespace", targetNamespace, "error", err)
				uc.emit(ctx, stackID, "deleting_release", "warn", fmt.Sprintf("release %s uninstall warning in %s: %v", releaseName, targetNamespace, err))
			}
		}
	}
}

func uninstallNamespacesForRelease(stackNamespace, releaseName string) []string {
	namespaces := []string{stackNamespace}
	if stackNamespace != "default" {
		namespaces = append(namespaces, "default")
	}

	// 옛 설치는 Envoy Gateway 를 플랫폼 네임스페이스에 두기도 했다. 지금은 전용
	// 자리(SharedGatewayNamespace)에 있고 공용이라 지우지 않으므로, 남의 집을
	// 뒤질 이유가 없다. 옛 envoy-gateway-system 만 잔재 정리를 위해 남긴다.
	if releaseName == "envoy-gateway" {
		namespaces = append(namespaces, legacyEnvoyGatewayNamespace)
	}

	seen := make(map[string]struct{}, len(namespaces))
	ordered := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		trimmed := strings.TrimSpace(ns)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		ordered = append(ordered, trimmed)
	}

	return ordered
}

// sharedInfraReleaseNames 는 스택들이 함께 쓰는 릴리스다. 한 스택을 지운다고
// 걷어 가면 다른 스택의 라우팅이 함께 죽는다.
var sharedInfraReleaseNames = map[string]struct{}{
	"eg":            {},
	"envoy-gateway": {},
}

func isSharedInfraRelease(name string) bool {
	_, ok := sharedInfraReleaseNames[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// cleanupNamespacesForStack 은 이름 기반 청소가 훑을 네임스페이스다.
//
// 공용 게이트웨이 자리는 절대 넣지 않는다 — 그 안의 것은 어느 스택 것도 아니다.
func cleanupNamespacesForStack(stackNamespace string) []string {
	out := make([]string, 0, 2)
	for _, ns := range uninstallNamespacesForRelease(stackNamespace, "") {
		if strings.EqualFold(ns, domain.SharedGatewayNamespace) {
			continue
		}
		out = append(out, ns)
	}
	return out
}

func (uc *DeleteStack) bestEffortDeleteYAMLResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil {
		return
	}
	if uc.deleteManifestFunc == nil {
		return
	}

	cfg, ok := extractStackConfig(stack.Config)
	if !ok {
		return
	}

	overrideKeys := make([]string, 0, len(cfg.YAMLOverrides))
	for key := range cfg.YAMLOverrides {
		overrideKeys = append(overrideKeys, key)
	}
	sort.Strings(overrideKeys)

	for _, key := range overrideKeys {
		body := cfg.YAMLOverrides[key]
		trimmed := strings.TrimSpace(body)
		if trimmed == "" || !looksLikeManifest(trimmed) {
			continue
		}
		uc.emit(ctx, stackID, "deleting_manifest", "info", fmt.Sprintf("deleting yaml manifest %s", key))
		if err := uc.deleteManifestFunc(ctx, kubeconfig, stack.Namespace, body); err != nil {
			slog.Warn("yaml manifest delete failed during stack delete", "step", key, "namespace", stack.Namespace, "error", err)
			uc.emit(ctx, stackID, "deleting_manifest", "warn", fmt.Sprintf("manifest %s delete warning: %v", key, err))
		}
	}
}

func (uc *DeleteStack) bestEffortDeleteLegacyMonitoringResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil || uc.listResourcesFunc == nil || uc.deleteResourceFunc == nil {
		return
	}
	if uc.isPlatformNamespace(stack.Namespace) {
		return
	}

	resources, err := uc.listResourcesFunc(ctx, kubeconfig, stack.Namespace)
	if err != nil {
		slog.Warn("legacy monitoring resources list failed during stack delete", "namespace", stack.Namespace, "error", err)
		uc.emit(ctx, stackID, "deleting_manifest", "warn", fmt.Sprintf("legacy monitoring resource list warning: %v", err))
		return
	}

	legacyTokens := []string{
		"prometheus-yaml",
		"grafana-yaml",
		"del-prom-yaml",
		"del-graf-yaml",
		"kube-prometheus-stack",
	}

	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		trimmed := strings.TrimSpace(resource.Ref)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}

		// 남의 릴리스 소유물은 이름이 무엇이든 건드리지 않는다.
		if ownedByAnotherRelease(resource) {
			continue
		}

		shouldDelete := false
		for _, token := range legacyTokens {
			if strings.Contains(trimmed, token) {
				shouldDelete = true
				break
			}
		}
		if !shouldDelete {
			continue
		}

		uc.emit(ctx, stackID, "deleting_manifest", "info", fmt.Sprintf("deleting legacy monitoring resource %s", trimmed))
		if err := uc.deleteResourceFunc(ctx, kubeconfig, stack.Namespace, trimmed); err != nil {
			slog.Warn("legacy monitoring resource delete failed during stack delete", "resource", trimmed, "namespace", stack.Namespace, "error", err)
			uc.emit(ctx, stackID, "deleting_manifest", "warn", fmt.Sprintf("legacy monitoring resource %s delete warning: %v", trimmed, err))
		}
	}
}

// bestEffortDeleteArgoCDCRDs 는 남은 Argo CD CRD 를 정리한다.
//
// 다른 스택이 아직 Application 을 갖고 있으면 지우지 않는다 — 지우면 그 스택의
// 배포 정의가 통째로 사라진다.
func (uc *DeleteStack) bestEffortDeleteArgoCDCRDs(ctx context.Context, kubeconfig []byte, stackID string) {
	if len(kubeconfig) == 0 {
		return
	}

	applicationsOut, err := uc.runKubectl(ctx, kubeconfig,
		"get", "applications.argoproj.io", "-A", "-o", "name")
	if err != nil {
		// CRD 자체가 없으면 조회가 실패한다 — 지울 것도 없으므로 그냥 끝낸다.
		slog.Warn("argocd crd cleanup skipped due to check failure", "resource", "applications", "error", err)
		return
	}
	appProjectsOut, err := uc.runKubectl(ctx, kubeconfig,
		"get", "appprojects.argoproj.io", "-A", "-o", "name")
	if err != nil {
		slog.Warn("argocd crd cleanup skipped due to check failure", "resource", "appprojects", "error", err)
		return
	}

	if argoCDResourcesInUse(applicationsOut, appProjectsOut) {
		uc.emit(ctx, stackID, "deleting_crd", "info", "skipping argocd CRD delete because argocd resources still exist")
		return
	}

	for _, crd := range argoCDCRDNames {
		uc.emit(ctx, stackID, "deleting_crd", "info", fmt.Sprintf("deleting argocd crd %s", crd))
		if _, err := uc.runKubectl(ctx, kubeconfig, "delete", "crd", crd, "--ignore-not-found"); err != nil {
			slog.Warn("argocd crd delete warning", "crd", crd, "error", err)
			uc.emit(ctx, stackID, "deleting_crd", "warn", fmt.Sprintf("argocd crd %s delete warning: %v", crd, err))
		}
	}
}

// autoCreatedArgoCDAppProject 는 Argo CD 가 기동하면서 스스로 만드는 기본
// 프로젝트다. 차트가 만든 게 아니라 application-controller 가 런타임에 만들기
// 때문에 helm uninstall 로는 사라지지 않는다.
const autoCreatedArgoCDAppProject = "appproject.argoproj.io/default"

// argoCDResourcesInUse 는 `kubectl get ... -o name` 출력 두 개를 보고 Argo CD CRD 를
// 남겨야 하는지 판단한다.
//
// 지우면 안 되는 경우는 남의 배포 정의가 아직 있을 때뿐이다. 반면 default
// AppProject 는 Argo CD 가 자기 부팅용으로 만든 것이라 "쓰는 중" 의 근거가 못 된다.
// 이걸 근거로 삼으면 조건이 항상 참이 돼 CRD 정리가 영영 안 돌고, 다음 스택 설치가
// `invalid ownership metadata` 로 실패한다.
func argoCDResourcesInUse(applicationsOut, appProjectsOut string) bool {
	if strings.TrimSpace(applicationsOut) != "" {
		return true
	}

	for _, line := range strings.Split(appProjectsOut, "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == autoCreatedArgoCDAppProject {
			continue
		}
		return true
	}

	return false
}

func (uc *DeleteStack) bestEffortDeleteGatewayCRDs(ctx context.Context, kubeconfig []byte, stackID string) {
	if len(kubeconfig) == 0 {
		return
	}

	hasGatewayResources := false
	checks := [][]string{
		{"get", "gateways.gateway.networking.k8s.io", "-A", "-o", "name"},
		{"get", "httproutes.gateway.networking.k8s.io", "-A", "-o", "name"},
		{"get", "gatewayclasses.gateway.networking.k8s.io", "-o", "name"},
	}
	for _, args := range checks {
		out, err := uc.runKubectl(ctx, kubeconfig, args...)
		if err != nil {
			slog.Warn("gateway crd cleanup skipped due to check failure", "args", strings.Join(args, " "), "error", err)
			return
		}
		if strings.TrimSpace(out) != "" {
			hasGatewayResources = true
			break
		}
	}
	if hasGatewayResources {
		uc.emit(ctx, stackID, "deleting_crd", "info", "skipping gateway CRD delete because gateway resources still exist")
		return
	}

	for _, crd := range gatewayCRDNames {
		uc.emit(ctx, stackID, "deleting_crd", "info", fmt.Sprintf("deleting gateway crd %s", crd))
		if _, err := uc.runKubectl(ctx, kubeconfig, "delete", "crd", crd, "--ignore-not-found"); err != nil {
			slog.Warn("gateway crd delete warning", "crd", crd, "error", err)
			uc.emit(ctx, stackID, "deleting_crd", "warn", fmt.Sprintf("gateway crd %s delete warning: %v", crd, err))
		}
	}
}

func (uc *DeleteStack) collectGatewayNames(ctx context.Context, kubeconfig []byte, stack *domain.Stack) []string {
	set := make(map[string]struct{})
	if stack == nil {
		return nil
	}

	cfg, ok := extractStackConfig(stack.Config)
	if ok {
		for _, raw := range cfg.YAMLOverrides {
			for _, gatewayName := range parseGatewayNamesFromManifest(raw) {
				set[gatewayName] = struct{}{}
			}
		}
	}

	if len(kubeconfig) != 0 && strings.TrimSpace(stack.Namespace) != "" {
		output, err := uc.runKubectl(ctx, kubeconfig, "get", "gateways.gateway.networking.k8s.io", "-n", stack.Namespace, "-o", "name")
		if err == nil {
			lines := strings.Split(strings.TrimSpace(output), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				name := strings.TrimSpace(strings.TrimPrefix(trimmed, "gateway.gateway.networking.k8s.io/"))
				if name == "" {
					continue
				}
				set[name] = struct{}{}
			}
		}
	}

	if len(set) == 0 {
		return nil
	}

	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseGatewayNamesFromManifest(manifest string) []string {
	trimmed := strings.TrimSpace(manifest)
	if trimmed == "" {
		return nil
	}

	dec := yaml.NewDecoder(strings.NewReader(trimmed))
	set := make(map[string]struct{})
	for {
		doc := map[string]any{}
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			continue
		}
		kind, _ := doc["kind"].(string)
		if !strings.EqualFold(strings.TrimSpace(kind), "Gateway") {
			continue
		}
		metadata, _ := doc["metadata"].(map[string]any)
		name, _ := metadata["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		set[name] = struct{}{}
	}

	if len(set) == 0 {
		return nil
	}

	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseGatewayNamesFromManagedResourceJSON(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var payload struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}

	set := make(map[string]struct{})
	for _, item := range payload.Items {
		labels := item.Metadata.Labels
		if len(labels) == 0 {
			continue
		}
		gatewayName := strings.TrimSpace(labels["gateway.envoyproxy.io/owning-gateway-name"])
		if gatewayName == "" {
			continue
		}
		set[gatewayName] = struct{}{}
	}

	if len(set) == 0 {
		return nil
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (uc *DeleteStack) collectGatewayNamesFromManagedResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack) []string {
	if len(kubeconfig) == 0 || stack == nil || strings.TrimSpace(stack.Namespace) == "" {
		return nil
	}

	stackNamespace := strings.TrimSpace(stack.Namespace)
	namespaces := cleanupNamespacesForStack(stackNamespace)
	selector := fmt.Sprintf("gateway.envoyproxy.io/owning-gateway-namespace=%s", stackNamespace)
	set := make(map[string]struct{})
	for _, targetNamespace := range namespaces {
		output, err := uc.runKubectl(ctx, kubeconfig, "get", "deploy,svc", "-n", targetNamespace, "-l", selector, "-o", "json")
		if err != nil {
			continue
		}
		for _, name := range parseGatewayNamesFromManagedResourceJSON(output) {
			set[name] = struct{}{}
		}
	}

	if len(set) == 0 {
		return nil
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (uc *DeleteStack) mergeGatewayNames(primary []string, extra []string) []string {
	if len(primary) == 0 && len(extra) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(primary)+len(extra))
	for _, name := range primary {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	for _, name := range extra {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// bestEffortDeletePersistentVolumeClaims 는 스택 네임스페이스의 PVC 를 지운다.
//
// helm uninstall 은 PVC 를 남긴다 — StatefulSet 의 volumeClaimTemplate 이 만든
// 것은 애초에 릴리스 소유가 아니고, 차트가 직접 만든 것도 대개 남긴다. 그래서
// 스택을 지울 때마다 디스크가 쌓였다(실측: Gitea 10Gi 포함 5개).
//
// 라벨로는 잡을 수 없다. bestEffortDeleteStackLabeledResources 의 목록에 pvc 가
// 이미 있지만 이 PVC 들에는 nullus.io/stack-name 라벨이 붙지 않는다. 실측한
// 라벨은 Helm 차트 것뿐이었고, 릴리스 라벨조차 없는 것이 있었다:
//
//	data-harbor-redis-0    release=harbor, heritage=Helm
//	gitea-shared-storage   app.kubernetes.io/managed-by=Helm 하나뿐
//
// 그래서 네임스페이스를 통째로 훑는다. 범위는 스택 자신의 네임스페이스뿐이다 —
// cleanupNamespacesForStack 이 함께 도는 default·nullus·envoy-gateway-system 은
// 다른 스택이나 사용자의 볼륨이 있을 수 있고, 거기서 --all 을 던지면 남의
// 데이터를 파기한다.
//
// 이 단계는 되돌릴 수 없다. Execute 의 마지막에 두는 이유도 그것이다 — 릴리스와
// StatefulSet 이 모두 사라진 뒤여야 컨트롤러가 PVC 를 다시 만들지 않는다.
func (uc *DeleteStack) bestEffortDeletePersistentVolumeClaims(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil {
		return
	}
	namespace := strings.TrimSpace(stack.Namespace)
	if namespace == "" {
		// 네임스페이스를 모르면 --all 이 현재 컨텍스트의 기본 네임스페이스를
		// 향한다. 지울 대상을 특정하지 못하면 아무것도 지우지 않는다.
		return
	}

	uc.emit(ctx, stackID, "deleting_pvc", "info",
		fmt.Sprintf("deleting persistent volume claims in namespace %s", namespace))
	if _, err := uc.runKubectl(ctx, kubeconfig, "delete", "pvc", "-n", namespace,
		"--all", "--ignore-not-found", "--timeout=120s"); err != nil {
		slog.Warn("pvc delete warning during stack delete", "namespace", namespace, "error", err)
		uc.emit(ctx, stackID, "deleting_pvc", "warn",
			fmt.Sprintf("pvc delete warning in %s: %v", namespace, err))
	}
}

// bestEffortDeleteGatewayAPIResources 는 스택의 Gateway/HTTPRoute 를 지운다.
//
// 이 단계가 없으면 삭제가 끝나도 Gateway 커스텀 리소스가 남고, Envoy Gateway
// 컨트롤러가 그것을 보고 데이터플레인 Deployment 를 다시 만든다. 그래서 아래
// bestEffortDeleteGatewayManagedResources 가 envoy 파드를 지워도 곧바로 되살아나
// 스택을 지운 뒤에도 envoy-<stack>-gateway 파드가 계속 떠 있었다 — helm list 는
// 깨끗하게 나오므로 발견도 늦었다(실측: 삭제 2시간 뒤에도 2/2 Running).
//
// 지우는 범위는 스택 자신의 네임스페이스뿐이다. cleanupNamespacesForStack 이
// 함께 도는 default·nullus·envoy-gateway-system 은 다른 스택과 공유될 수 있어
// --all 로 지우면 남의 게이트웨이를 지운다.
func (uc *DeleteStack) bestEffortDeleteGatewayAPIResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil {
		return
	}
	namespace := strings.TrimSpace(stack.Namespace)
	if namespace == "" {
		return
	}

	for _, resource := range namespacedGatewayAPIResources {
		uc.emit(ctx, stackID, "deleting_gateway_api", "info",
			fmt.Sprintf("deleting %s in namespace %s", resource, namespace))
		if _, err := uc.runKubectl(ctx, kubeconfig, "delete", resource, "-n", namespace,
			"--all", "--ignore-not-found", "--timeout=60s"); err != nil {
			// CRD 자체가 없는 클러스터에서는 "the server doesn't have a resource
			// type" 이 난다 — 지울 것이 없다는 뜻이므로 경고로만 남긴다.
			slog.Warn("gateway api resource delete warning", "resource", resource, "namespace", namespace, "error", err)
			uc.emit(ctx, stackID, "deleting_gateway_api", "warn",
				fmt.Sprintf("%s delete warning in %s: %v", resource, namespace, err))
		}
	}
}

func (uc *DeleteStack) bestEffortDeleteGatewayManagedResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, gatewayNames []string, stackID string) {
	if len(kubeconfig) == 0 || stack == nil || strings.TrimSpace(stack.Namespace) == "" || len(gatewayNames) == 0 {
		return
	}

	namespaces := cleanupNamespacesForStack(stack.Namespace)
	for _, gatewayName := range gatewayNames {
		selector := fmt.Sprintf("gateway.envoyproxy.io/owning-gateway-name=%s", gatewayName)
		for _, targetNamespace := range namespaces {
			uc.emit(ctx, stackID, "deleting_gateway_managed", "info", fmt.Sprintf("deleting gateway managed resources for %s in namespace %s", gatewayName, targetNamespace))
			for _, kind := range []string{"deploy", "svc", "cm", "sa", "pod", "rs", "secret", "pvc"} {
				if _, err := uc.runKubectl(ctx, kubeconfig, "delete", kind, "-n", targetNamespace, "-l", selector, "--ignore-not-found"); err != nil {
					slog.Warn("gateway managed resource delete warning", "kind", kind, "namespace", targetNamespace, "gateway", gatewayName, "error", err)
					uc.emit(ctx, stackID, "deleting_gateway_managed", "warn", fmt.Sprintf("gateway managed %s delete warning for %s in %s: %v", kind, gatewayName, targetNamespace, err))
				}
			}
		}
	}
}

func (uc *DeleteStack) bestEffortDeleteStackLabeledResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil {
		return
	}

	stackName := strings.TrimSpace(stack.Name)
	if stackName == "" {
		return
	}

	selector := fmt.Sprintf("%s=%s", stackNameLabelKey, stackName)
	namespaces := cleanupNamespacesForStack(stack.Namespace)
	for _, targetNamespace := range namespaces {
		uc.emit(ctx, stackID, "deleting_stack_labeled", "info", fmt.Sprintf("deleting stack-labeled resources in namespace %s", targetNamespace))
		for _, kind := range []string{"deploy", "svc", "cm", "sa", "pod", "rs", "sts", "job", "cronjob", "secret", "pvc"} {
			if _, err := uc.runKubectl(ctx, kubeconfig, "delete", kind, "-n", targetNamespace, "-l", selector, "--ignore-not-found"); err != nil {
				slog.Warn("stack-labeled resource delete warning", "kind", kind, "namespace", targetNamespace, "selector", selector, "error", err)
				uc.emit(ctx, stackID, "deleting_stack_labeled", "warn", fmt.Sprintf("stack-labeled %s delete warning in %s: %v", kind, targetNamespace, err))
			}
		}
	}
}

func (uc *DeleteStack) bestEffortDeleteLegacyGatewayPolicyResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil || strings.TrimSpace(stack.Namespace) == "" {
		return
	}
	namespace := strings.TrimSpace(stack.Namespace)
	legacyResources := []string{
		"backendtlspolicy.gateway.networking.k8s.io/opensearch-backend-tls",
		"configmap/opensearch-root-ca",
	}
	for _, resource := range legacyResources {
		uc.emit(ctx, stackID, "deleting_manifest", "info", fmt.Sprintf("deleting legacy gateway policy resource %s", resource))
		if _, err := uc.runKubectl(ctx, kubeconfig, "delete", "-n", namespace, resource, "--ignore-not-found"); err != nil {
			slog.Warn("legacy gateway policy resource delete warning", "resource", resource, "namespace", namespace, "error", err)
			uc.emit(ctx, stackID, "deleting_manifest", "warn", fmt.Sprintf("legacy gateway policy resource %s delete warning: %v", resource, err))
		}
	}
}

func (uc *DeleteStack) bestEffortDeleteLegacyReleaseArtifacts(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil || uc.listResourcesFunc == nil || uc.deleteResourceFunc == nil {
		return
	}

	stackName := strings.TrimSpace(stack.Name)
	namespaces := cleanupNamespacesForStack(stack.Namespace)
	seen := make(map[string]struct{})
	for _, targetNamespace := range namespaces {
		if uc.isPlatformNamespace(targetNamespace) {
			uc.emit(ctx, stackID, "deleting_manifest", "warn",
				fmt.Sprintf("%s 는 플랫폼 네임스페이스라 이름 기반 정리를 건너뜁니다", targetNamespace))
			continue
		}
		resources, err := uc.listResourcesFunc(ctx, kubeconfig, targetNamespace)
		if err != nil {
			slog.Warn("legacy release artifact list warning", "namespace", targetNamespace, "error", err)
			uc.emit(ctx, stackID, "deleting_manifest", "warn", fmt.Sprintf("legacy release artifact list warning in %s: %v", targetNamespace, err))
			continue
		}
		for _, resource := range resources {
			trimmed := strings.TrimSpace(resource.Ref)
			if trimmed == "" {
				continue
			}
			key := targetNamespace + "::" + trimmed
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			if !shouldDeleteReleaseArtifact(resource, stackName) {
				continue
			}

			uc.emit(ctx, stackID, "deleting_manifest", "info", fmt.Sprintf("deleting legacy release artifact %s in namespace %s", trimmed, targetNamespace))
			if err := uc.deleteResourceFunc(ctx, kubeconfig, targetNamespace, trimmed); err != nil {
				slog.Warn("legacy release artifact delete warning", "resource", trimmed, "namespace", targetNamespace, "error", err)
				uc.emit(ctx, stackID, "deleting_manifest", "warn", fmt.Sprintf("legacy release artifact %s delete warning in %s: %v", trimmed, targetNamespace, err))
			}
		}
	}
}

// shouldDeleteReleaseArtifact 는 "이 리소스가 이 스택 것인가" 를 판정한다.
//
// 판정 순서가 곧 안전장치다. 소유자를 밝힌 리소스는 그 말을 그대로 따르고,
// 이름 규칙은 소유자가 없는 고아에만 쓴다. 예전에는 순서가 없었고 이름에 스택
// 이름이 들어가기만 하면 지웠다 — 스택 이름이 "nullus" 이고 네임스페이스가
// 플랫폼과 같았을 때 플랫폼 자신을 지워 버렸다.
func shouldDeleteReleaseArtifact(resource namespacedResource, stackName string) bool {
	if label := strings.TrimSpace(resource.StackLabel); label != "" {
		return strings.EqualFold(label, strings.TrimSpace(stackName))
	}

	// Helm 이 소유를 밝혔으면 이 스택이 만든 릴리스일 때만 지운다. 남의
	// 릴리스면 이름이 무엇이든 건드리지 않는다.
	if release := strings.TrimSpace(resource.HelmRelease); release != "" {
		return isStackHelmRelease(release)
	}

	return matchesLegacyArtifactName(resourceNameFromRef(resource.Ref))
}

// matchesLegacyArtifactName 은 소유자 표시가 없는 고아를 이름으로 알아본다.
func matchesLegacyArtifactName(rawName string) bool {
	name := strings.ToLower(strings.TrimSpace(rawName))
	if name == "" {
		return false
	}
	if strings.Contains(name, "yaml") {
		return false
	}

	if _, ok := legacyReleaseArtifactExactNames[name]; ok {
		return true
	}

	for _, prefix := range legacyReleaseArtifactPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

// ownedByAnotherRelease 는 이 스택이 만들지 않은 Helm 릴리스의 소유물인지 본다.
func ownedByAnotherRelease(resource namespacedResource) bool {
	release := strings.TrimSpace(resource.HelmRelease)
	return release != "" && !isStackHelmRelease(release)
}

// SetPlatformNamespace 는 플랫폼 자신이 사는 네임스페이스를 알려준다.
//
// 스택이 그 네임스페이스에 깔려 있으면 이름 기반 청소를 하지 않는다. 소유권
// 판정만으로도 남의 것은 지키지만, 플랫폼을 지우는 사고는 한 번으로 족하다.
func (uc *DeleteStack) SetPlatformNamespace(namespace string) {
	uc.platformNamespace = strings.TrimSpace(namespace)
}

func (uc *DeleteStack) isPlatformNamespace(namespace string) bool {
	return uc.platformNamespace != "" &&
		strings.EqualFold(strings.TrimSpace(namespace), uc.platformNamespace)
}

func (uc *DeleteStack) bestEffortDeleteOrphanGatewayTempoResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil || uc.listResourcesFunc == nil || uc.deleteResourceFunc == nil {
		return
	}

	stackName := strings.ToLower(strings.TrimSpace(stack.Name))
	namespaces := cleanupNamespacesForStack(stack.Namespace)
	seen := make(map[string]struct{})
	for _, targetNamespace := range namespaces {
		if uc.isPlatformNamespace(targetNamespace) {
			continue
		}
		resources, err := uc.listResourcesFunc(ctx, kubeconfig, targetNamespace)
		if err != nil {
			slog.Warn("orphan gateway/tempo resource list warning", "namespace", targetNamespace, "error", err)
			uc.emit(ctx, stackID, "deleting_orphan_resources", "warn", fmt.Sprintf("orphan resource list warning in %s: %v", targetNamespace, err))
			continue
		}

		for _, resource := range resources {
			trimmed := strings.TrimSpace(resource.Ref)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[targetNamespace+"::"+trimmed]; ok {
				continue
			}
			seen[targetNamespace+"::"+trimmed] = struct{}{}

			if ownedByAnotherRelease(resource) {
				continue
			}

			if !shouldDeleteOrphanGatewayTempoResource(trimmed, stackName, targetNamespace, stack.Namespace) {
				continue
			}

			uc.emit(ctx, stackID, "deleting_orphan_resources", "info", fmt.Sprintf("deleting orphan resource %s in namespace %s", trimmed, targetNamespace))
			if err := uc.deleteResourceFunc(ctx, kubeconfig, targetNamespace, trimmed); err != nil {
				slog.Warn("orphan gateway/tempo resource delete warning", "resource", trimmed, "namespace", targetNamespace, "error", err)
				uc.emit(ctx, stackID, "deleting_orphan_resources", "warn", fmt.Sprintf("orphan resource %s delete warning in %s: %v", trimmed, targetNamespace, err))
				uc.bestEffortClearResourceFinalizers(ctx, kubeconfig, targetNamespace, trimmed, stackID)
				uc.bestEffortForceDeleteResource(ctx, kubeconfig, targetNamespace, trimmed, stackID)
			}
		}
	}
}

func shouldDeleteOrphanGatewayTempoResource(resourceRef, stackNameLower, targetNamespace, stackNamespace string) bool {
	name := strings.ToLower(strings.TrimSpace(resourceNameFromRef(resourceRef)))
	if name == "" {
		return false
	}

	targetNamespace = strings.TrimSpace(targetNamespace)
	stackNamespace = strings.TrimSpace(stackNamespace)
	if targetNamespace == "" || stackNamespace == "" {
		return false
	}

	isStackNamespace := targetNamespace == stackNamespace

	if _, ok := orphanTempoResourceNames[name]; ok && isStackNamespace {
		return true
	}
	if strings.HasPrefix(name, "tempo-") && isStackNamespace {
		return true
	}

	if strings.HasPrefix(name, "envoy-") {
		return stackNameLower != "" && strings.Contains(name, stackNameLower)
	}

	if stackNameLower != "" && strings.Contains(name, stackNameLower) && strings.Contains(name, "gateway") {
		return true
	}

	return false
}

func resourceNameFromRef(resourceRef string) string {
	trimmed := strings.TrimSpace(resourceRef)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		return strings.TrimSpace(trimmed[idx+1:])
	}
	return trimmed
}

func (uc *DeleteStack) bestEffortClearResourceFinalizers(ctx context.Context, kubeconfig []byte, namespace, resource, stackID string) {
	if len(kubeconfig) == 0 || strings.TrimSpace(namespace) == "" || strings.TrimSpace(resource) == "" {
		return
	}
	if _, err := uc.runKubectl(ctx, kubeconfig, "patch", "-n", namespace, resource, "--type=merge", "-p", `{"metadata":{"finalizers":[]}}`); err != nil {
		slog.Warn("clear finalizers warning", "resource", resource, "namespace", namespace, "error", err)
		uc.emit(ctx, stackID, "deleting_orphan_resources", "warn", fmt.Sprintf("clear finalizers warning for %s in %s: %v", resource, namespace, err))
	}
}

func (uc *DeleteStack) bestEffortForceDeleteResource(ctx context.Context, kubeconfig []byte, namespace, resource, stackID string) {
	if len(kubeconfig) == 0 || strings.TrimSpace(namespace) == "" || strings.TrimSpace(resource) == "" {
		return
	}
	if _, err := uc.runKubectl(ctx, kubeconfig, "delete", "-n", namespace, resource, "--ignore-not-found", "--force", "--grace-period=0"); err != nil {
		slog.Warn("force delete orphan warning", "resource", resource, "namespace", namespace, "error", err)
		uc.emit(ctx, stackID, "deleting_orphan_resources", "warn", fmt.Sprintf("force delete warning for %s in %s: %v", resource, namespace, err))
	}
}

func looksLikeManifest(raw string) bool {
	return strings.Contains(raw, "apiVersion:") && strings.Contains(raw, "kind:")
}

func extractStackConfig(raw any) (domain.StackConfig, bool) {
	if cfg, ok := raw.(domain.StackConfig); ok {
		return cfg, true
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return domain.StackConfig{}, false
	}
	var cfg domain.StackConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return domain.StackConfig{}, false
	}
	return cfg, true
}

func deleteManifest(ctx context.Context, kubeconfig []byte, namespace, manifest string) error {
	if strings.TrimSpace(manifest) == "" {
		return nil
	}
	tmpFile, err := os.CreateTemp("", "nullus-delete-kubeconfig-*.yaml")
	if err != nil {
		return fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(kubeconfig); err != nil {
		return fmt.Errorf("write kubeconfig temp file: %w", err)
	}

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", tmpFile.Name(), "delete", "-n", namespace, "-f", "-", "--ignore-not-found")
	cmd.Stdin = strings.NewReader(manifest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// listNamespaceResources 는 청소 후보와 그 소유자 표시를 함께 읽는다.
//
// 예전에는 -o name 으로 이름만 읽었다. 이름만 있으면 남의 것인지 알 길이 없어
// 이름 규칙에 기대게 되고, 그 규칙이 플랫폼을 지웠다. 소유자는 이미 리소스에
// 적혀 있으므로 함께 읽어 온다.
func listNamespaceResources(ctx context.Context, kubeconfig []byte, namespace string) ([]namespacedResource, error) {
	if strings.TrimSpace(namespace) == "" {
		return nil, nil
	}
	output, err := runKubectlWithKubeconfig(ctx, kubeconfig, "get", "deploy,svc,cm,sa,pod,rs,sts,job,cronjob,secret,pvc", "-n", namespace, "-o", "json")
	if err != nil {
		return nil, err
	}
	return parseNamespaceResources(output)
}

func parseNamespaceResources(raw string) ([]namespacedResource, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	var list struct {
		Items []struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name        string            `json:"name"`
				Labels      map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
		return nil, fmt.Errorf("parse namespace resources: %w", err)
	}

	out := make([]namespacedResource, 0, len(list.Items))
	for _, item := range list.Items {
		kind := strings.ToLower(strings.TrimSpace(item.Kind))
		name := strings.TrimSpace(item.Metadata.Name)
		if kind == "" || name == "" {
			continue
		}
		out = append(out, namespacedResource{
			Ref:         kind + "/" + name,
			HelmRelease: strings.TrimSpace(item.Metadata.Annotations["meta.helm.sh/release-name"]),
			StackLabel:  strings.TrimSpace(item.Metadata.Labels[stackNameLabelKey]),
		})
	}
	return out, nil
}

func deleteResource(ctx context.Context, kubeconfig []byte, namespace, resource string) error {
	if strings.TrimSpace(resource) == "" {
		return nil
	}
	_, err := runKubectlWithKubeconfig(ctx, kubeconfig, "delete", "-n", namespace, resource, "--ignore-not-found")
	return err
}

func runKubectlWithKubeconfig(ctx context.Context, kubeconfig []byte, args ...string) (string, error) {
	tmpFile, err := os.CreateTemp("", "nullus-delete-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(kubeconfig); err != nil {
		return "", fmt.Errorf("write kubeconfig temp file: %w", err)
	}

	kubectlArgs := append([]string{"--kubeconfig", tmpFile.Name()}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", kubectlArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func (uc *DeleteStack) emit(ctx context.Context, stackID, step, level, message string) {
	if uc.streamer == nil || stackID == "" {
		return
	}
	uc.streamer.Stream(ctx, stackID, port.LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Step:      step,
		Message:   message,
		Phase:     "delete",
	})
}

func isStackNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrStackNotFound) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

// externalSecretResourceKinds 는 ESO 오퍼레이터 제거 전에 먼저 지워야 하는
// 커스텀 리소스 종류다.
var externalSecretResourceKinds = []string{
	"externalsecrets.external-secrets.io",
	"secretstores.external-secrets.io",
}

// bestEffortDeleteExternalSecretResources 는 ESO 커스텀 리소스를 먼저 정리한다.
//
// ExternalSecret 에는 finalizer 가 붙어 있어 컨트롤러가 있어야 삭제가 끝난다.
// 오퍼레이터를 먼저 uninstall 하면 finalizer 를 처리할 주체가 사라져
// 네임스페이스가 Terminating 에서 벗어나지 못하고, CRD 도 함께 묶인다.
// 그 상태에서는 같은 클러스터에 스택을 다시 설치할 수 없다.
func (uc *DeleteStack) bestEffortDeleteExternalSecretResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil {
		return
	}
	namespace := strings.TrimSpace(stack.Namespace)
	if namespace == "" {
		return
	}

	for _, kind := range externalSecretResourceKinds {
		if _, err := uc.runKubectl(ctx, kubeconfig, "delete", kind,
			"-n", namespace, "--all", "--ignore-not-found", "--timeout=60s"); err != nil {
			slog.Warn("external-secrets 리소스 삭제 경고", "kind", kind, "namespace", namespace, "error", err)
			uc.emit(ctx, stackID, "deleting_external_secrets", "warn",
				fmt.Sprintf("%s 삭제 경고 (%s): %v", kind, namespace, err))
		}
	}
}
