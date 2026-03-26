package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

var stackHelmReleaseNames = []string{
	"cert-manager",
	"minio",
	"gitlab",
	"argo-cd",
	"gitlab-runner",
	"kube-prometheus-stack",
	"grafana",
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

	kubeconfig := uc.loadKubeconfig(ctx, stack.ClusterID)
	uc.bestEffortUninstall(ctx, kubeconfig, stack.Namespace, stackID)
	uc.bestEffortDeleteYAMLResources(ctx, kubeconfig, stack, stackID)
	uc.bestEffortDeleteLegacyMonitoringResources(ctx, kubeconfig, stack, stackID)

	stack.State = domain.StateCancelled
	stack.UpdatedAt = time.Now()
	if err := uc.stackRepo.Update(ctx, stack); err != nil {
		uc.emit(ctx, stackID, "delete_failed", "error", err.Error())
		return fmt.Errorf("mark stack cancelled: %w", err)
	}

	uc.emit(ctx, stackID, "deleted", "info", "stack delete completed")

	return nil
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
		uc.emit(ctx, stackID, "deleting_release", "info", fmt.Sprintf("uninstalling release %s", releaseName))
		if err := installer.Uninstall(ctx, releaseName, namespace); err != nil {
			slog.Warn("helm uninstall failed during stack delete", "release", releaseName, "namespace", namespace, "error", err)
			uc.emit(ctx, stackID, "deleting_release", "warn", fmt.Sprintf("release %s uninstall warning: %v", releaseName, err))
		}
	}
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

	manifests := []struct {
		step string
		body string
	}{
		{step: "prometheus", body: cfg.YAMLOverrides["prometheus"]},
		{step: "grafana", body: cfg.YAMLOverrides["grafana"]},
		{step: "logging", body: cfg.YAMLOverrides["logging"]},
		{step: "opentelemetry", body: cfg.YAMLOverrides["opentelemetry"]},
		{step: "opentelemetry-collector", body: cfg.YAMLOverrides["opentelemetry-collector"]},
		{step: "installing_prometheus", body: cfg.YAMLOverrides["installing_prometheus"]},
		{step: "installing_grafana", body: cfg.YAMLOverrides["installing_grafana"]},
		{step: "installing_logging", body: cfg.YAMLOverrides["installing_logging"]},
		{step: "installing_opentelemetry", body: cfg.YAMLOverrides["installing_opentelemetry"]},
	}

	for _, m := range manifests {
		trimmed := strings.TrimSpace(m.body)
		if trimmed == "" || !looksLikeManifest(trimmed) {
			continue
		}
		uc.emit(ctx, stackID, "deleting_manifest", "info", fmt.Sprintf("deleting yaml manifest %s", m.step))
		if err := uc.deleteManifestFunc(ctx, kubeconfig, stack.Namespace, m.body); err != nil {
			slog.Warn("yaml manifest delete failed during stack delete", "step", m.step, "namespace", stack.Namespace, "error", err)
			uc.emit(ctx, stackID, "deleting_manifest", "warn", fmt.Sprintf("manifest %s delete warning: %v", m.step, err))
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
	output, err := runKubectlWithKubeconfig(ctx, kubeconfig, "get", "deploy,svc", "-n", namespace, "-o", "name")
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
