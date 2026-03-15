package helm

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
)

func TestHelmInstaller_ImplementsPort(t *testing.T) {
	t.Parallel()
	var _ port.HelmInstaller = &HelmInstaller{}
}

func TestHelmInstaller_Install_ReturnsActionConfigError(t *testing.T) {
	installer := &HelmInstaller{
		newActionConfig: func(_ []byte, _ string) (*action.Configuration, error) {
			return nil, fmt.Errorf("init failed")
		},
	}

	_, err := installer.Install(context.Background(), port.HelmInstallRequest{
		ReleaseName: "cert-manager",
		ChartName:   "cert-manager",
		RepoURL:     "https://charts.jetstack.io",
		Namespace:   "nullus",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

func TestHelmInstaller_Uninstall_ReturnsActionConfigError(t *testing.T) {
	installer := &HelmInstaller{
		newActionConfig: func(_ []byte, _ string) (*action.Configuration, error) {
			return nil, fmt.Errorf("init failed")
		},
	}

	err := installer.Uninstall(context.Background(), "cert-manager", "nullus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

func TestHelmInstaller_Status_ReturnsActionConfigError(t *testing.T) {
	installer := &HelmInstaller{
		newActionConfig: func(_ []byte, _ string) (*action.Configuration, error) {
			return nil, fmt.Errorf("init failed")
		},
	}

	_, err := installer.Status(context.Background(), "cert-manager", "nullus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}
