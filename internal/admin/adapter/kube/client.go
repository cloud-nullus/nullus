package kube

import (
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type VerifyResult struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func VerifyCluster(kubeconfigBytes []byte) (*VerifyResult, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	config.Timeout = 10 * time.Second

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("verify cluster connection: %w", err)
	}

	return &VerifyResult{Status: "connected", Version: versionInfo.GitVersion}, nil
}
