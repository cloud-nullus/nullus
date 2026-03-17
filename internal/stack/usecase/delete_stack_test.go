package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHelmInstaller struct {
	uninstallCalls []string
	uninstallErr   error
}

func (f *fakeHelmInstaller) Install(context.Context, port.HelmInstallRequest) (*port.HelmInstallResult, error) {
	return nil, nil
}

func (f *fakeHelmInstaller) Uninstall(_ context.Context, releaseName, namespace string) error {
	f.uninstallCalls = append(f.uninstallCalls, releaseName+"@"+namespace)
	return f.uninstallErr
}

func (f *fakeHelmInstaller) Status(context.Context, string, string) (*port.HelmInstallResult, error) {
	return nil, nil
}

type fakeDeleteKubeconfigProvider struct {
	config []byte
	err    error
}

func (f *fakeDeleteKubeconfigProvider) GetKubeconfig(context.Context, string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.config, nil
}

func TestDeleteStack_UninstallsKnownReleasesThenDeletesStack(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-1",
		ClusterID: "cluster-1",
		Namespace: "devsecops",
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("kubeconfig")}
	installer := &fakeHelmInstaller{}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return installer
	})

	err := uc.Execute(context.Background(), "stk-1")
	require.NoError(t, err)

	_, getErr := repo.GetByID(context.Background(), "stk-1")
	require.Error(t, getErr)
	assert.Equal(t, []string{
		"cert-manager@devsecops",
		"minio@devsecops",
		"gitlab@devsecops",
		"argo-cd@devsecops",
		"gitlab-runner@devsecops",
		"kube-prometheus-stack@devsecops",
		"grafana@devsecops",
	}, installer.uninstallCalls)
}

func TestDeleteStack_DeletesStackWhenKubeconfigAndUninstallFail(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-2",
		ClusterID: "cluster-2",
		Namespace: "nullus",
	})
	provider := &fakeDeleteKubeconfigProvider{err: errors.New("kubeconfig unavailable")}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return &fakeHelmInstaller{uninstallErr: errors.New("release not found")}
	})

	err := uc.Execute(context.Background(), "stk-2")
	require.NoError(t, err)

	_, getErr := repo.GetByID(context.Background(), "stk-2")
	require.Error(t, getErr)
}
