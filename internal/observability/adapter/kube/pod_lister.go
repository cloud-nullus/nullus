package kube

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// PodInfo describes a single Pod relevant to the observability dashboard.
type PodInfo struct {
	Name   string `json:"name"`
	Node   string `json:"node"`
	Status string `json:"status"`
	// Phase 는 가공하지 않은 Pod phase 다. Status 는 컨테이너 대기 사유로 덮어써지므로
	// "Succeeded 로 끝난 일회성 Job 인가" 같은 판단은 이 값으로 해야 한다.
	Phase string `json:"phase"`
	Ready bool   `json:"ready"`
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
		ready := false
		for _, cond := range p.Status.Conditions {
			if cond.Type == corev1.PodReady {
				ready = cond.Status == corev1.ConditionTrue
				break
			}
		}

		out = append(out, PodInfo{
			Name:   p.Name,
			Node:   p.Spec.NodeName,
			Status: status,
			Phase:  string(p.Status.Phase),
			Ready:  ready,
		})
	}
	return out, nil
}
