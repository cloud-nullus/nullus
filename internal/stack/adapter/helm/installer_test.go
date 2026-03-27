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

func TestShouldUpgradeOnInstallError(t *testing.T) {
	assert.True(t, shouldUpgradeOnInstallError(fmt.Errorf("cannot re-use a name that is still in use")))
	assert.False(t, shouldUpgradeOnInstallError(fmt.Errorf("some other install error")))
	assert.False(t, shouldUpgradeOnInstallError(nil))
}

func TestShouldReinstallOnExistingStatus(t *testing.T) {
	assert.True(t, shouldReinstallOnExistingStatus("pending-upgrade"))
	assert.True(t, shouldReinstallOnExistingStatus("failed"))
	assert.False(t, shouldReinstallOnExistingStatus("deployed"))
	assert.False(t, shouldReinstallOnExistingStatus(""))
}

func TestShouldReinstallOnUpgradeError(t *testing.T) {
	assert.True(t, shouldReinstallOnUpgradeError(fmt.Errorf("upgrade release gitlab: context deadline exceeded")))
	assert.True(t, shouldReinstallOnUpgradeError(fmt.Errorf("another operation (install/upgrade/rollback) is in progress")))
	assert.False(t, shouldReinstallOnUpgradeError(fmt.Errorf("validation failed")))
	assert.False(t, shouldReinstallOnUpgradeError(nil))
}

func TestShouldIgnoreUninstallError(t *testing.T) {
	assert.True(t, shouldIgnoreUninstallError(nil))
	assert.True(t, shouldIgnoreUninstallError(fmt.Errorf("uninstall: release: not found")))
	assert.False(t, shouldIgnoreUninstallError(fmt.Errorf("permission denied")))
}

func TestShouldRetryReinstallError(t *testing.T) {
	assert.True(t, shouldRetryReinstallError(fmt.Errorf("services \"gitlab-gitaly\" not found")))
	assert.False(t, shouldRetryReinstallError(fmt.Errorf("context deadline exceeded")))
	assert.False(t, shouldRetryReinstallError(nil))
}

func TestResolveWait_DefaultsTrueWhenNil(t *testing.T) {
	assert.True(t, resolveWait(nil))
}

func TestResolveWait_UsesExplicitValue(t *testing.T) {
	falseValue := false
	trueValue := true
	assert.False(t, resolveWait(&falseValue))
	assert.True(t, resolveWait(&trueValue))
}
