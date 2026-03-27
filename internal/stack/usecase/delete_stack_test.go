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
	assert.Contains(t, installer.uninstallCalls, "cert-manager@devsecops")
	assert.Contains(t, installer.uninstallCalls, "cert-manager@default")
	assert.Contains(t, installer.uninstallCalls, "opensearch@devsecops")
	assert.Contains(t, installer.uninstallCalls, "opensearch@default")
	assert.Contains(t, installer.uninstallCalls, "eg@devsecops")
	assert.Contains(t, installer.uninstallCalls, "eg@default")
	assert.Contains(t, installer.uninstallCalls, "eg@nullus")
	assert.Contains(t, installer.uninstallCalls, "eg@envoy-gateway-system")
	assert.Contains(t, installer.uninstallCalls, "envoy-gateway@devsecops")
	assert.Contains(t, installer.uninstallCalls, "envoy-gateway@default")
	assert.Contains(t, installer.uninstallCalls, "envoy-gateway@nullus")
	assert.Contains(t, installer.uninstallCalls, "envoy-gateway@envoy-gateway-system")
	steps := make([]string, 0, len(streamer.entries))
	for _, e := range streamer.entries {
		steps = append(steps, e.Step)
	}
	assert.Contains(t, steps, "deleting_started")
	assert.Contains(t, steps, "deleted")
}

func TestUninstallNamespacesForRelease_GatewayIncludesFallbackNamespaces(t *testing.T) {
	namespaces := uninstallNamespacesForRelease("devsecops", "eg")
	assert.Equal(t, []string{"devsecops", "default", "nullus", "envoy-gateway-system"}, namespaces)
}

func TestUninstallNamespacesForRelease_DeduplicatesNullusNamespace(t *testing.T) {
	namespaces := uninstallNamespacesForRelease("nullus", "eg")
	assert.Equal(t, []string{"nullus", "default", "envoy-gateway-system"}, namespaces)
}

func TestCleanupNamespacesForStack_UsesGatewaySweepNamespaces(t *testing.T) {
	namespaces := cleanupNamespacesForStack("devsecops")
	assert.Equal(t, []string{"devsecops", "default", "nullus", "envoy-gateway-system"}, namespaces)
}

func TestCleanupNamespacesForStack_DeduplicatesNullusNamespace(t *testing.T) {
	namespaces := cleanupNamespacesForStack("nullus")
	assert.Equal(t, []string{"nullus", "default", "envoy-gateway-system"}, namespaces)
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
				"prometheus":         "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: prom\n",
				"grafana":            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: graf\n",
				"tempo":              "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: tempo\n",
				"installing_logging": "singleNode: true\nprotocol: http\n",
			},
		},
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}
	installer := &fakeHelmInstaller{}
	streamer := &captureStreamer{}
	manifestCalls := []string{}
	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return installer
	}, streamer)
	uc.deleteManifestFunc = func(_ context.Context, _ []byte, _ string, manifest string) error {
		manifestCalls = append(manifestCalls, manifest)
		return nil
	}

	err := uc.Execute(context.Background(), "stk-3")
	require.NoError(t, err)
	require.Len(t, manifestCalls, 6)
	assert.True(t, strings.Contains(manifestCalls[0], "apiVersion:") || strings.Contains(manifestCalls[1], "apiVersion:"))
	assert.True(t, strings.Contains(strings.Join(manifestCalls, "\n---\n"), "name: tempo"))

	firstManifestIdx := -1
	firstUninstallIdx := -1
	for i, entry := range streamer.entries {
		if firstManifestIdx < 0 && entry.Step == "deleting_manifest" {
			firstManifestIdx = i
		}
		if firstUninstallIdx < 0 && entry.Step == "deleting_release" {
			firstUninstallIdx = i
		}
	}
	require.GreaterOrEqual(t, firstManifestIdx, 0)
	require.GreaterOrEqual(t, firstUninstallIdx, 0)
	assert.Less(t, firstManifestIdx, firstUninstallIdx)
}

func TestDeleteStack_MarksCancelledBeforeManifestCleanup(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-3b",
		ClusterID: "cluster-3b",
		Namespace: "nullus",
		State:     domain.StateCompleted,
		Config: domain.StackConfig{
			YAMLOverrides: map[string]string{
				"gateway": "apiVersion: gateway.networking.k8s.io/v1\nkind: Gateway\nmetadata:\n  name: g1\n",
			},
		},
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}
	installer := &fakeHelmInstaller{}
	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return installer
	})

	stateSeenDuringManifestDelete := domain.StatePending
	uc.deleteManifestFunc = func(_ context.Context, _ []byte, _ string, _ string) error {
		stateSeenDuringManifestDelete = repo.getState("stk-3b")
		return nil
	}

	err := uc.Execute(context.Background(), "stk-3b")
	require.NoError(t, err)
	assert.Equal(t, domain.StateCancelled, stateSeenDuringManifestDelete)
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

func TestParseGatewayNamesFromManifest(t *testing.T) {
	manifest := `apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: nullus-devsecops-stack-gateway
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: gitlab-route
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: nullus-devsecops-stack-2-gateway
`

	names := parseGatewayNamesFromManifest(manifest)
	assert.Equal(t, []string{"nullus-devsecops-stack-2-gateway", "nullus-devsecops-stack-gateway"}, names)
}

func TestParseGatewayNamesFromManagedResourceJSON(t *testing.T) {
	raw := `{
  "items": [
    {
      "metadata": {
        "name": "envoy-nullus-nullus-devsecops-stack-gateway-3197e0f2",
        "labels": {
          "gateway.envoyproxy.io/owning-gateway-name": "nullus-devsecops-stack-gateway"
        }
      }
    },
    {
      "metadata": {
        "name": "envoy-nullus-other-gateway-1234",
        "labels": {
          "gateway.envoyproxy.io/owning-gateway-name": "other-stack-gateway"
        }
      }
    }
  ]
}`

	names := parseGatewayNamesFromManagedResourceJSON(raw, "nullus-devsecops-stack")
	assert.Equal(t, []string{"nullus-devsecops-stack-gateway"}, names)
}

func TestDeleteStack_MergeGatewayNames(t *testing.T) {
	uc := &DeleteStack{}
	merged := uc.mergeGatewayNames(
		[]string{"nullus-devsecops-stack-gateway", ""},
		[]string{"nullus-devsecops-stack-gateway", "another-gateway"},
	)
	assert.Equal(t, []string{"another-gateway", "nullus-devsecops-stack-gateway"}, merged)
}

func TestShouldDeleteOrphanGatewayTempoResource(t *testing.T) {
	assert.True(t, shouldDeleteOrphanGatewayTempoResource("deployment.apps/tempo", "nullus-devsecops-stack", "nullus", "nullus"))
	assert.True(t, shouldDeleteOrphanGatewayTempoResource("service/tempo-svc", "nullus-devsecops-stack", "nullus", "nullus"))
	assert.False(t, shouldDeleteOrphanGatewayTempoResource("deployment.apps/tempo", "nullus-devsecops-stack", "default", "nullus"))
	assert.True(t, shouldDeleteOrphanGatewayTempoResource("deployment.apps/envoy-nullus-nullus-devsecops-stack-gateway-3197e0f2", "nullus-devsecops-stack", "nullus", "nullus"))
	assert.False(t, shouldDeleteOrphanGatewayTempoResource("deployment.apps/envoy-gateway", "nullus-devsecops-stack", "nullus", "nullus"))
	assert.False(t, shouldDeleteOrphanGatewayTempoResource("deployment.apps/app-web", "nullus-devsecops-stack", "nullus", "nullus"))
}

func TestDeleteStack_DeletesOrphanGatewayTempoResourcesAcrossSweepNamespaces(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-orphan",
		Name:      "nullus-devsecops-stack",
		ClusterID: "cluster-orphan",
		Namespace: "nullus",
		State:     domain.StateCompleted,
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}
	installer := &fakeHelmInstaller{}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return installer
	})
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]string, error) {
		switch namespace {
		case "nullus":
			return []string{
				"deployment.apps/envoy-nullus-nullus-devsecops-stack-gateway-3197e0f2",
				"deployment.apps/tempo",
				"service/tempo-svc",
				"service/kubernetes",
			}, nil
		case "default":
			return []string{"deployment.apps/envoy-shared-gateway"}, nil
		default:
			return nil, nil
		}
	}

	deleted := []string{}
	uc.deleteResourceFunc = func(_ context.Context, _ []byte, namespace, resource string) error {
		deleted = append(deleted, namespace+":"+resource)
		return nil
	}

	err := uc.Execute(context.Background(), "stk-orphan")
	require.NoError(t, err)

	assert.Contains(t, deleted, "nullus:deployment.apps/envoy-nullus-nullus-devsecops-stack-gateway-3197e0f2")
	assert.Contains(t, deleted, "nullus:deployment.apps/tempo")
	assert.Contains(t, deleted, "nullus:service/tempo-svc")
	assert.NotContains(t, deleted, "nullus:service/kubernetes")
	assert.NotContains(t, deleted, "default:deployment.apps/envoy-shared-gateway")
}

func TestShouldDeleteLegacyReleaseArtifact(t *testing.T) {
	assert.True(t, shouldDeleteLegacyReleaseArtifact("secret/gitlab-gitlab-initial-root-password", "nullus-devsecops-stack"))
	assert.True(t, shouldDeleteLegacyReleaseArtifact("pvc/data-nullus-postgresql-0", "nullus-devsecops-stack"))
	assert.True(t, shouldDeleteLegacyReleaseArtifact("secret/nullus-devsecops-stack-wildcard-tls", "nullus-devsecops-stack"))
	assert.False(t, shouldDeleteLegacyReleaseArtifact("configmap/kube-root-ca.crt", "nullus-devsecops-stack"))
	assert.False(t, shouldDeleteLegacyReleaseArtifact("serviceaccount/default", "nullus-devsecops-stack"))
}

func TestDeleteStack_DeletesLegacyReleaseArtifacts(t *testing.T) {
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-legacy-artifacts",
		Name:      "nullus-devsecops-stack",
		ClusterID: "cluster-legacy-artifacts",
		Namespace: "nullus",
		State:     domain.StateCompleted,
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}
	installer := &fakeHelmInstaller{}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return installer
	})
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]string, error) {
		if namespace != "nullus" {
			return nil, nil
		}
		return []string{
			"secret/gitlab-gitlab-initial-root-password",
			"pvc/data-nullus-postgresql-0",
			"serviceaccount/default",
			"configmap/kube-root-ca.crt",
		}, nil
	}

	deleted := []string{}
	uc.deleteResourceFunc = func(_ context.Context, _ []byte, namespace, resource string) error {
		deleted = append(deleted, namespace+":"+resource)
		return nil
	}

	err := uc.Execute(context.Background(), "stk-legacy-artifacts")
	require.NoError(t, err)

	assert.Contains(t, deleted, "nullus:secret/gitlab-gitlab-initial-root-password")
	assert.Contains(t, deleted, "nullus:pvc/data-nullus-postgresql-0")
	assert.NotContains(t, deleted, "nullus:serviceaccount/default")
	assert.NotContains(t, deleted, "nullus:configmap/kube-root-ca.crt")
}
