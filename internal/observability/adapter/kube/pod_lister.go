package kube

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// PodInfo describes a single Pod relevant to the observability dashboard.
type PodInfo struct {
	Name   string `json:"name"`
	Node   string `json:"node"`
	Status string `json:"status"`
}

// ListPodsInNamespace returns all pods in the given namespace via the supplied kubeconfig.
// Status reports the container waiting reason (e.g. CrashLoopBackOff) when present,
// otherwise the Pod phase (Running, Pending, ...).
func ListPodsInNamespace(ctx context.Context, kubeconfig []byte, namespace string) ([]PodInfo, error) {
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("empty kubeconfig")
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	cfg.Timeout = 5 * time.Second

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	list, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	out := make([]PodInfo, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		status := string(p.Status.Phase)
		for _, cstat := range p.Status.ContainerStatuses {
			if cstat.State.Waiting != nil && cstat.State.Waiting.Reason != "" {
				status = cstat.State.Waiting.Reason
				break
			}
		}
		out = append(out, PodInfo{
			Name:   p.Name,
			Node:   p.Spec.NodeName,
			Status: status,
		})
	}
	return out, nil
}
