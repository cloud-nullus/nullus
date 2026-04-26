//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	helmadapter "github.com/cloud-nullus/draft/internal/stack/adapter/helm"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/stretchr/testify/require"
)

type stackConfigAwareExecutor interface {
	SetStackConfig(config domain.StackConfig)
}

type namespaceAwareExecutor interface {
	SetNamespace(namespace string)
}

type deploymentVerifiableExecutor interface {
	VerifyDeployment(ctx context.Context, stackID string) error
}

var _ port.StepExecutor = (*helmadapter.Orchestrator)(nil)
var _ stackConfigAwareExecutor = (*helmadapter.Orchestrator)(nil)
var _ namespaceAwareExecutor = (*helmadapter.Orchestrator)(nil)
var _ deploymentVerifiableExecutor = (*helmadapter.Orchestrator)(nil)

func TestHelmDeployOnKindCluster(t *testing.T) {
	clusterName, kubeconfig, available := discoverKindCluster(t)
	if !available {
		t.Skip("kind cluster not available")
	}

	installer := helmadapter.NewHelmInstaller(kubeconfig)
	orch := helmadapter.NewOrchestrator(installer, kubeconfig, "default")
	require.NotNil(t, orch)

	require.NotPanics(t, func() { orch.SetNamespace("default") })
	require.NotPanics(t, func() { orch.SetStackConfig(domain.StackConfig{}) })

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	releaseName := fmt.Sprintf("nullus-e2e-%d", time.Now().UnixNano())
	request := port.HelmInstallRequest{
		ReleaseName: releaseName,
		ChartName:   "nginx",
		RepoURL:     "https://charts.bitnami.com/bitnami",
		Namespace:   "default",
		Wait:        boolPtr(false),
	}

	result, err := installer.Install(ctx, request)
	require.NoErrorf(t, err, "failed to install test chart on kind cluster %q", clusterName)
	require.NotNil(t, result)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		_ = installer.Uninstall(cleanupCtx, releaseName, "default")
	})

	status, err := installer.Status(ctx, releaseName, "default")
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Equal(t, releaseName, status.ReleaseName)
	require.Equal(t, "default", status.Namespace)
	require.NotEmpty(t, status.Status)
}

func discoverKindCluster(t *testing.T) (clusterName string, kubeconfig []byte, ok bool) {
	t.Helper()

	if _, err := exec.LookPath("kind"); err != nil {
		return "", nil, false
	}

	listCmd := exec.Command("kind", "get", "clusters")
	listOut, err := listCmd.Output()
	if err != nil {
		return "", nil, false
	}

	clusters := strings.Fields(string(listOut))
	if len(clusters) == 0 {
		return "", nil, false
	}

	preferred := "nullus-platform"
	selected := clusters[0]
	for _, name := range clusters {
		if name == preferred {
			selected = preferred
			break
		}
	}

	kubeconfigCmd := exec.Command("kind", "get", "kubeconfig", "--name", selected)
	kubeconfigOut, err := kubeconfigCmd.Output()
	if err != nil {
		return "", nil, false
	}

	return selected, kubeconfigOut, true
}

func boolPtr(v bool) *bool {
	return &v
}
