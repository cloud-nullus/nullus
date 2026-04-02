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

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"gopkg.in/yaml.v3"
)

var stackHelmReleaseNames = []string{
	"cert-manager",
	"nullus-postgresql",
	"postgresql",
	"nullus-minio",
	"minio",
	"gitlab",
	"argo-cd",
	"gitlab-runner",
	"kube-prometheus-stack",
	"grafana",
	"loki",
	"opensearch",
	"elasticsearch",
	"tempo",
	"jaeger",
	"opentelemetry-collector",
	"eg",
	"envoy-gateway",
}

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
	"nullus-object-storage":                                 {},
	"data-nullus-postgresql-0":                              {},
	"opensearch-cluster-master-opensearch-cluster-master-0": {},
	"redis-data-gitlab-redis-master-0":                      {},
	"repo-data-gitlab-gitaly-0":                             {},
}

var legacyReleaseArtifactPrefixes = []string{
	"gitlab-",
	"argo-cd-",
	"argocd-",
	"envoy-",
	"nullus-",
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

type DeleteStack struct {
	stackRepo           port.StackRepository
	kubeconfigProvider  port.KubeconfigProvider
	executorFactoryFunc func(kubeconfig []byte) port.HelmInstaller
	streamer            port.LogStreamer
	deleteManifestFunc  func(ctx context.Context, kubeconfig []byte, namespace, manifest string) error
	listResourcesFunc   func(ctx context.Context, kubeconfig []byte, namespace string) ([]string, error)
	deleteResourceFunc  func(ctx context.Context, kubeconfig []byte, namespace, resource string) error
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
		return fmt.Errorf("mark stack cancelled: %w", err)
	}

	kubeconfig := uc.loadKubeconfig(ctx, stack.ClusterID)
	gatewayNames := uc.collectGatewayNames(ctx, kubeconfig, stack)
	gatewayNames = uc.mergeGatewayNames(gatewayNames, uc.collectGatewayNamesFromManagedResources(ctx, kubeconfig, stack))
	uc.bestEffortDeleteYAMLResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortUninstall(ctx, kubeconfig, stack.Namespace, stackID)
	uc.bestEffortDeleteStackLabeledResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteGatewayManagedResources(ctx, kubeconfig, stack, gatewayNames, stackID)
	uc.bestEffortDeleteLegacyMonitoringResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteLegacyGatewayPolicyResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteLegacyReleaseArtifacts(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteOrphanGatewayTempoResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteGatewayCRDs(ctx, kubeconfig, stackID)

	if err := uc.stackRepo.Delete(ctx, stackID); err != nil {
		uc.emit(ctx, stackID, "delete_failed", "error", err.Error())
		return fmt.Errorf("delete stack: %w", err)
	}

	uc.emit(ctx, stackID, "deleted", "info", "stack delete completed")
	uc.clearStreamHistory(stackID)

	return nil
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

	if releaseName == "eg" || releaseName == "envoy-gateway" {
		namespaces = append(namespaces, "nullus", legacyEnvoyGatewayNamespace)
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

func cleanupNamespacesForStack(stackNamespace string) []string {
	return uninstallNamespacesForRelease(stackNamespace, "eg")
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
		trimmed := strings.TrimSpace(resource)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}

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
		out, err := runKubectlWithKubeconfig(ctx, kubeconfig, args...)
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
		if _, err := runKubectlWithKubeconfig(ctx, kubeconfig, "delete", "crd", crd, "--ignore-not-found"); err != nil {
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
		output, err := runKubectlWithKubeconfig(ctx, kubeconfig, "get", "gateways.gateway.networking.k8s.io", "-n", stack.Namespace, "-o", "name")
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
				if stack.Name != "" && !strings.Contains(name, stack.Name) {
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

func parseGatewayNamesFromManagedResourceJSON(raw string, stackName string) []string {
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

	stackName = strings.ToLower(strings.TrimSpace(stackName))
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
		if stackName != "" && !strings.Contains(strings.ToLower(gatewayName), stackName) {
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
		output, err := runKubectlWithKubeconfig(ctx, kubeconfig, "get", "deploy,svc", "-n", targetNamespace, "-l", selector, "-o", "json")
		if err != nil {
			continue
		}
		for _, name := range parseGatewayNamesFromManagedResourceJSON(output, stack.Name) {
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
				if _, err := runKubectlWithKubeconfig(ctx, kubeconfig, "delete", kind, "-n", targetNamespace, "-l", selector, "--ignore-not-found"); err != nil {
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
			if _, err := runKubectlWithKubeconfig(ctx, kubeconfig, "delete", kind, "-n", targetNamespace, "-l", selector, "--ignore-not-found"); err != nil {
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
		if _, err := runKubectlWithKubeconfig(ctx, kubeconfig, "delete", "-n", namespace, resource, "--ignore-not-found"); err != nil {
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
		resources, err := uc.listResourcesFunc(ctx, kubeconfig, targetNamespace)
		if err != nil {
			slog.Warn("legacy release artifact list warning", "namespace", targetNamespace, "error", err)
			uc.emit(ctx, stackID, "deleting_manifest", "warn", fmt.Sprintf("legacy release artifact list warning in %s: %v", targetNamespace, err))
			continue
		}
		for _, resource := range resources {
			trimmed := strings.TrimSpace(resource)
			if trimmed == "" {
				continue
			}
			key := targetNamespace + "::" + trimmed
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			if !shouldDeleteLegacyReleaseArtifact(trimmed, stackName) {
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

func shouldDeleteLegacyReleaseArtifact(resourceRef, stackName string) bool {
	name := strings.ToLower(strings.TrimSpace(resourceNameFromRef(resourceRef)))
	if name == "" {
		return false
	}

	if _, ok := legacyReleaseArtifactExactNames[name]; ok {
		return true
	}

	stackName = strings.ToLower(strings.TrimSpace(stackName))
	if stackName != "" && strings.Contains(name, stackName) {
		return true
	}

	for _, prefix := range legacyReleaseArtifactPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

func (uc *DeleteStack) bestEffortDeleteOrphanGatewayTempoResources(ctx context.Context, kubeconfig []byte, stack *domain.Stack, stackID string) {
	if len(kubeconfig) == 0 || stack == nil || uc.listResourcesFunc == nil || uc.deleteResourceFunc == nil {
		return
	}

	stackName := strings.ToLower(strings.TrimSpace(stack.Name))
	namespaces := cleanupNamespacesForStack(stack.Namespace)
	seen := make(map[string]struct{})
	for _, targetNamespace := range namespaces {
		resources, err := uc.listResourcesFunc(ctx, kubeconfig, targetNamespace)
		if err != nil {
			slog.Warn("orphan gateway/tempo resource list warning", "namespace", targetNamespace, "error", err)
			uc.emit(ctx, stackID, "deleting_orphan_resources", "warn", fmt.Sprintf("orphan resource list warning in %s: %v", targetNamespace, err))
			continue
		}

		for _, resource := range resources {
			trimmed := strings.TrimSpace(resource)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[targetNamespace+"::"+trimmed]; ok {
				continue
			}
			seen[targetNamespace+"::"+trimmed] = struct{}{}

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
	if _, err := runKubectlWithKubeconfig(ctx, kubeconfig, "patch", "-n", namespace, resource, "--type=merge", "-p", `{"metadata":{"finalizers":[]}}`); err != nil {
		slog.Warn("clear finalizers warning", "resource", resource, "namespace", namespace, "error", err)
		uc.emit(ctx, stackID, "deleting_orphan_resources", "warn", fmt.Sprintf("clear finalizers warning for %s in %s: %v", resource, namespace, err))
	}
}

func (uc *DeleteStack) bestEffortForceDeleteResource(ctx context.Context, kubeconfig []byte, namespace, resource, stackID string) {
	if len(kubeconfig) == 0 || strings.TrimSpace(namespace) == "" || strings.TrimSpace(resource) == "" {
		return
	}
	if _, err := runKubectlWithKubeconfig(ctx, kubeconfig, "delete", "-n", namespace, resource, "--ignore-not-found", "--force", "--grace-period=0"); err != nil {
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

func listNamespaceResources(ctx context.Context, kubeconfig []byte, namespace string) ([]string, error) {
	if strings.TrimSpace(namespace) == "" {
		return nil, nil
	}
	output, err := runKubectlWithKubeconfig(ctx, kubeconfig, "get", "deploy,svc,cm,sa,pod,rs,sts,job,cronjob,secret,pvc", "-n", namespace, "-o", "name")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return nil, nil
	}
	return lines, nil
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
