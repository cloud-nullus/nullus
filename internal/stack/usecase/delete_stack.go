package usecase

import (
	"context"
	"fmt"
	"log/slog"

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
}

func NewDeleteStack(
	stackRepo port.StackRepository,
	kubeconfigProvider port.KubeconfigProvider,
	executorFactory func(kubeconfig []byte) port.HelmInstaller,
) *DeleteStack {
	return &DeleteStack{
		stackRepo:           stackRepo,
		kubeconfigProvider:  kubeconfigProvider,
		executorFactoryFunc: executorFactory,
	}
}

func (uc *DeleteStack) Execute(ctx context.Context, stackID string) error {
	stack, err := uc.stackRepo.GetByID(ctx, stackID)
	if err != nil {
		return fmt.Errorf("get stack: %w", err)
	}
	if stack != nil {
		uc.bestEffortUninstall(ctx, stack.ClusterID, stack.Namespace)
	}

	if err := uc.stackRepo.Delete(ctx, stackID); err != nil {
		return fmt.Errorf("delete stack: %w", err)
	}

	return nil
}

func (uc *DeleteStack) bestEffortUninstall(ctx context.Context, clusterID, namespace string) {
	if uc.kubeconfigProvider == nil || uc.executorFactoryFunc == nil || clusterID == "" || namespace == "" {
		return
	}

	kubeconfig, err := uc.kubeconfigProvider.GetKubeconfig(ctx, clusterID)
	if err != nil {
		slog.Warn("stack delete continues without helm uninstall", "cluster_id", clusterID, "error", err)
		return
	}
	if len(kubeconfig) == 0 {
		return
	}

	installer := uc.executorFactoryFunc(kubeconfig)
	if installer == nil {
		return
	}

	for _, releaseName := range stackHelmReleaseNames {
		if err := installer.Uninstall(ctx, releaseName, namespace); err != nil {
			slog.Warn("helm uninstall failed during stack delete", "release", releaseName, "namespace", namespace, "error", err)
		}
	}
}
