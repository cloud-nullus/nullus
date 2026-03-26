package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureStreamer struct {
	entries []port.LogEntry
}

func (c *captureStreamer) Stream(_ context.Context, _ string, entry port.LogEntry) {
	c.entries = append(c.entries, entry)
}

func (c *captureStreamer) Subscribe(_ string) <-chan port.LogEntry {
	ch := make(chan port.LogEntry, 16)
	return ch
}

func (c *captureStreamer) Unsubscribe(_ string, _ <-chan port.LogEntry) {}

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
		State:     domain.StateCompleted,
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("kubeconfig")}
	installer := &fakeHelmInstaller{}
	streamer := &captureStreamer{}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return installer
	}, streamer)

	err := uc.Execute(context.Background(), "stk-1")
	require.NoError(t, err)

	s, getErr := repo.GetByID(context.Background(), "stk-1")
	require.NoError(t, getErr)
	assert.Equal(t, domain.StateCancelled, s.State)
	assert.Equal(t, []string{
		"cert-manager@devsecops",
		"minio@devsecops",
		"gitlab@devsecops",
		"argo-cd@devsecops",
		"gitlab-runner@devsecops",
		"kube-prometheus-stack@devsecops",
		"grafana@devsecops",
	}, installer.uninstallCalls)
	steps := make([]string, 0, len(streamer.entries))
	for _, e := range streamer.entries {
		steps = append(steps, e.Step)
	}
	assert.Contains(t, steps, "deleting_started")
	assert.Contains(t, steps, "deleted")
}

func TestDeleteStack_DeletesStackWhenKubeconfigAndUninstallFail(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-2",
		ClusterID: "cluster-2",
		Namespace: "nullus",
		State:     domain.StateCompleted,
	})
	provider := &fakeDeleteKubeconfigProvider{err: errors.New("kubeconfig unavailable")}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return &fakeHelmInstaller{uninstallErr: errors.New("release not found")}
	})

	err := uc.Execute(context.Background(), "stk-2")
	require.NoError(t, err)

	s, getErr := repo.GetByID(context.Background(), "stk-2")
	require.NoError(t, getErr)
	assert.Equal(t, domain.StateCancelled, s.State)
}

func TestDeleteStack_DeletesMonitoringManifestOverrides(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-3",
		ClusterID: "cluster-3",
		Namespace: "nullus",
		State:     domain.StateCompleted,
		Config: domain.StackConfig{
			Monitoring: domain.MonitoringConfig{
				Collection:    domain.ToolSelection{Name: "prometheus", Enabled: true},
				Visualization: domain.ToolSelection{Name: "grafana", Enabled: true},
			},
			YAMLOverrides: map[string]string{
				"prometheus": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: prom\n",
				"grafana":    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: graf\n",
			},
		},
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}
	installer := &fakeHelmInstaller{}
	manifestCalls := []string{}
	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return installer
	})
	uc.deleteManifestFunc = func(_ context.Context, _ []byte, _ string, manifest string) error {
		manifestCalls = append(manifestCalls, manifest)
		return nil
	}

	err := uc.Execute(context.Background(), "stk-3")
	require.NoError(t, err)
	require.Len(t, manifestCalls, 2)
	assert.True(t, strings.Contains(manifestCalls[0], "apiVersion:") || strings.Contains(manifestCalls[1], "apiVersion:"))
}

type nilReturningStackRepo struct {
	*fakeStackRepo
}

func (r *nilReturningStackRepo) GetByID(context.Context, string) (*domain.Stack, error) {
	return nil, nil
}

func (r *nilReturningStackRepo) FindByID(context.Context, string) (*domain.Stack, error) {
	return nil, nil
}

func TestDeleteStack_ReturnsErrStackNotFoundWhenRepositoryReturnsNilStack(t *testing.T) {
	repo := &nilReturningStackRepo{fakeStackRepo: newFakeStackRepo()}
	streamer := &captureStreamer{}

	uc := NewDeleteStack(repo, nil, nil, streamer)
	err := uc.Execute(context.Background(), "stk-missing")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrStackNotFound)
	steps := make([]string, 0, len(streamer.entries))
	for _, entry := range streamer.entries {
		steps = append(steps, entry.Step)
	}
	assert.Contains(t, steps, "delete_failed")
}

func TestDeleteStack_DeletesLegacyMonitoringResources(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-legacy",
		ClusterID: "cluster-legacy",
		Namespace: "nullus",
		State:     domain.StateCompleted,
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}
	installer := &fakeHelmInstaller{}
	streamer := &captureStreamer{}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return installer
	}, streamer)
	uc.listResourcesFunc = func(_ context.Context, _ []byte, _ string) ([]string, error) {
		return []string{
			"deployment.apps/prometheus-yaml-v2",
			"service/grafana-yaml-svc",
			"service/kubernetes",
			"deployment.apps/app-web",
		}, nil
	}
	deleted := []string{}
	uc.deleteResourceFunc = func(_ context.Context, _ []byte, _ string, resource string) error {
		deleted = append(deleted, resource)
		return nil
	}

	err := uc.Execute(context.Background(), "stk-legacy")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"deployment.apps/prometheus-yaml-v2",
		"service/grafana-yaml-svc",
	}, deleted)

	messages := make([]string, 0, len(streamer.entries))
	for _, entry := range streamer.entries {
		messages = append(messages, entry.Message)
	}
	assert.True(t, strings.Contains(strings.Join(messages, "\n"), "legacy monitoring resource"))
}
