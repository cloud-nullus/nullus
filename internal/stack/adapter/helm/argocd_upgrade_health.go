package helm

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// ArgoCDUpgradeHealthVerifier checks the control-plane workloads after Helm
// reports success. It intentionally does not read Secret values.
type ArgoCDUpgradeHealthVerifier struct{}

func (ArgoCDUpgradeHealthVerifier) Verify(ctx context.Context, kubeconfig []byte, stack *domain.Stack, bundle domain.UpgradeBundle) error {
	if bundle.Tool != "argocd" {
		return fmt.Errorf("unsupported upgrade health driver: %s", bundle.Tool)
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("parse kubeconfig: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create kubernetes client: %w", err)
	}
	ns := strings.TrimSpace(stack.Namespace)
	if ns == "" {
		ns = "nullus"
	}
	deployments, err := client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/part-of=argocd"})
	if err != nil {
		return fmt.Errorf("list Argo CD deployments: %w", err)
	}
	if len(deployments.Items) == 0 {
		return fmt.Errorf("Argo CD deployments not found")
	}
	for _, deployment := range deployments.Items {
		desired := int32(1)
		if deployment.Spec.Replicas != nil {
			desired = *deployment.Spec.Replicas
		}
		if deployment.Status.ObservedGeneration < deployment.Generation || deployment.Status.ReadyReplicas < desired {
			return fmt.Errorf("Argo CD deployment %s is not ready (%d/%d)", deployment.Name, deployment.Status.ReadyReplicas, desired)
		}
	}
	return nil
}
