package kube

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

// DefaultDiscoveryTimeout is the total wall-clock budget for DiscoverCluster:
// both `Discovery().ServerVersion()` and `CoreV1().Nodes().List()` must
// complete within this window.
const DefaultDiscoveryTimeout = 10 * time.Second

// VerifyResult is the legacy response shape kept for handler backwards
// compatibility. New call sites should prefer DiscoverCluster which returns
// the full ClusterDiscoveryInfo value object.
type VerifyResult struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// clientsetBuilder builds a kubernetes.Interface from a parsed rest.Config.
// Swapped out in tests to inject a fake clientset.
var clientsetBuilder = func(cfg *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(cfg)
}

// VerifyCluster is the original entry point that only returns server
// version. It now delegates to DiscoverCluster under the hood so a single
// round trip pulls both server version and node architectures.
func VerifyCluster(kubeconfigBytes []byte) (*VerifyResult, error) {
	info, err := DiscoverCluster(context.Background(), kubeconfigBytes)
	if err != nil {
		return nil, err
	}
	return &VerifyResult{Status: "connected", Version: info.ServerVersion}, nil
}

// DiscoverCluster connects to the cluster described by kubeconfigBytes and
// returns its server version and the sorted, de-duplicated set of node
// architectures (from `node.status.nodeInfo.architecture`). The Cluster
// aggregate stores this so the Pre-Deploy Gate can cross-check
// `ToolVersion.ArchSupport` against the actual workload fleet.
func DiscoverCluster(ctx context.Context, kubeconfigBytes []byte) (*domain.ClusterDiscoveryInfo, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	config.Timeout = DefaultDiscoveryTimeout

	clientset, err := clientsetBuilder(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, DefaultDiscoveryTimeout)
	defer cancel()

	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("verify cluster connection: %w", err)
	}

	nodeList, err := clientset.CoreV1().Nodes().List(callCtx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list cluster nodes: %w", err)
	}

	archs := make([]string, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		if a := node.Status.NodeInfo.Architecture; a != "" {
			archs = append(archs, a)
		}
	}

	return &domain.ClusterDiscoveryInfo{
		ServerVersion:     versionInfo.GitVersion,
		NodeArchitectures: domain.NormalizeNodeArchitectures(archs),
		NodeCount:         len(nodeList.Items),
		DiscoveredAt:      time.Now().UTC(),
	}, nil
}
