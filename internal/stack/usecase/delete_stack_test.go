package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type captureStreamer struct {
	entries          []port.LogEntry
	clearedHistoryID []string
}

func (c *captureStreamer) Stream(_ context.Context, _ string, entry port.LogEntry) {
	c.entries = append(c.entries, entry)
}

func (c *captureStreamer) Subscribe(_ string) <-chan port.LogEntry {
	ch := make(chan port.LogEntry, 16)
	return ch
}

func (c *captureStreamer) Unsubscribe(_ string, _ <-chan port.LogEntry) {}

func (c *captureStreamer) ClearHistory(deploymentID string) {
	c.clearedHistoryID = append(c.clearedHistoryID, deploymentID)
}

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
	require.Error(t, getErr)
	assert.Nil(t, s)
	assert.Contains(t, installer.uninstallCalls, "cert-manager@devsecops")
	assert.Contains(t, installer.uninstallCalls, "cert-manager@default")
	assert.Contains(t, installer.uninstallCalls, "opensearch@devsecops")
	assert.Contains(t, installer.uninstallCalls, "opensearch@default")
	// 게이트웨이는 스택 것이다 — 지우면 함께 회수된다. 다만 훑는 자리는 스택
	// 네임스페이스와 default 뿐이다. 플랫폼 네임스페이스까지 뒤지던 옛 동작이
	// 2026-08-20 에 플랫폼을 지운 경로였다.
	assert.Contains(t, installer.uninstallCalls, "eg@devsecops")
	for _, call := range installer.uninstallCalls {
		assert.NotContains(t, call, "@nullus")
	}
	steps := make([]string, 0, len(streamer.entries))
	for _, e := range streamer.entries {
		steps = append(steps, e.Step)
	}
	assert.Contains(t, steps, "deleting_started")
	assert.Contains(t, steps, "deleted")
	assert.Equal(t, []string{"stk-1"}, streamer.clearedHistoryID)
}

// Harbor/Nexus 를 지우지 않으면 스택을 삭제해도 레지스트리 파드와 PVC 가 남아
// 다음 스택 설치가 같은 네임스페이스에서 리소스를 물고 늘어진다.
func TestDeleteStack_UninstallsRegistryReleases(t *testing.T) {
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

	require.NoError(t, uc.Execute(context.Background(), "stk-1"))

	assert.Contains(t, installer.uninstallCalls, domain.HarborReleaseName+"@devsecops")
	assert.Contains(t, installer.uninstallCalls, domain.NexusReleaseName+"@devsecops")
}

// Argo CD 의 application-controller 는 기동하면서 default AppProject 를 스스로
// 만든다. helm uninstall 로는 안 지워지므로, 이걸 "다른 스택이 쓰는 중" 으로 보면
// CRD 정리가 영원히 건너뛰어진다 — 그러면 다음 스택 설치가 CRD 소유권 충돌로 죽는다.
func TestArgoCDResourcesInUse(t *testing.T) {
	tests := []struct {
		name         string
		applications string
		appProjects  string
		want         bool
	}{
		{
			name:         "nothing left",
			applications: "",
			appProjects:  "",
			want:         false,
		},
		{
			name:         "only the auto-created default AppProject",
			applications: "",
			appProjects:  "appproject.argoproj.io/default\n",
			want:         false,
		},
		{
			name:         "an Application still exists",
			applications: "application.argoproj.io/my-app\n",
			appProjects:  "appproject.argoproj.io/default\n",
			want:         true,
		},
		{
			name:         "a user-defined AppProject still exists",
			applications: "",
			appProjects:  "appproject.argoproj.io/default\nappproject.argoproj.io/team-a\n",
			want:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, argoCDResourcesInUse(tc.applications, tc.appProjects))
		})
	}
}

// eg 는 이제 공용이라 언인스톨 대상이 아니다. 옛 이름(envoy-gateway)으로 깔린
// 잔재만 legacy 네임스페이스까지 훑는다.
func TestUninstallNamespacesForRelease_LegacyGatewayIncludesFallbackNamespace(t *testing.T) {
	namespaces := uninstallNamespacesForRelease("devsecops", "envoy-gateway")
	assert.Equal(t, []string{"devsecops", "default", "envoy-gateway-system"}, namespaces)
}

func TestUninstallNamespacesForRelease_StaysWithinStackAndDefault(t *testing.T) {
	namespaces := uninstallNamespacesForRelease("devsecops", "harbor")
	assert.Equal(t, []string{"devsecops", "default"}, namespaces)
}

// 게이트웨이가 전용 네임스페이스로 옮겨간 뒤로는 남의 집을 훑지 않는다.
// 예전에는 Envoy Gateway 를 찾겠다고 플랫폼 네임스페이스(nullus)까지 뒤졌고,
// 그것이 2026-08-20 플랫폼 삭제 사고의 경로였다.
func TestCleanupNamespacesForStack_StaysWithinStackAndDefault(t *testing.T) {
	namespaces := cleanupNamespacesForStack("devsecops")
	assert.Equal(t, []string{"devsecops", "default"}, namespaces)
}

func TestCleanupNamespacesForStack_DeduplicatesNamespaces(t *testing.T) {
	namespaces := cleanupNamespacesForStack("nullus")
	assert.Equal(t, []string{"nullus", "default"}, namespaces)
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
	require.Error(t, getErr)
	assert.Nil(t, s)
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
	uc.listResourcesFunc = func(_ context.Context, _ []byte, _ string) ([]namespacedResource, error) {
		return []namespacedResource{
			{Ref: "deployment.apps/prometheus-yaml-v2"},
			{Ref: "service/grafana-yaml-svc"},
			{Ref: "service/kubernetes"},
			{Ref: "deployment.apps/app-web"},
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

	names := parseGatewayNamesFromManagedResourceJSON(raw)
	assert.Equal(t, []string{"nullus-devsecops-stack-gateway", "other-stack-gateway"}, names)
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
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		switch namespace {
		case "nullus":
			return []namespacedResource{
				{Ref: "deployment.apps/envoy-nullus-nullus-devsecops-stack-gateway-3197e0f2"},
				{Ref: "deployment.apps/tempo"},
				{Ref: "service/tempo-svc"},
				{Ref: "service/kubernetes"},
			}, nil
		case "default":
			return []namespacedResource{{Ref: "deployment.apps/envoy-shared-gateway"}}, nil
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

func TestShouldDeleteReleaseArtifact(t *testing.T) {
	artifact := func(ref string) namespacedResource { return namespacedResource{Ref: ref} }

	assert.True(t, shouldDeleteReleaseArtifact(artifact("secret/gitlab-gitlab-initial-root-password"), "nullus-devsecops-stack"))
	assert.True(t, shouldDeleteReleaseArtifact(artifact("pvc/data-nullus-postgresql-0"), "nullus-devsecops-stack"))
	assert.False(t, shouldDeleteReleaseArtifact(artifact("configmap/kube-root-ca.crt"), "nullus-devsecops-stack"))
	assert.False(t, shouldDeleteReleaseArtifact(artifact("serviceaccount/default"), "nullus-devsecops-stack"))

	// 스택 이름이 들어간 와일드카드 TLS 시크릿은 더 이상 이름만으로 지우지 않는다.
	// 스택이 플랫폼의 공용 인증서를 그대로 지정할 수 있기 때문이다 — 실제로
	// 2026-08-20 의 스택 설정이 secret_name=nullus-wildcard-tls 를 가리키고 있었다.
	// 그 이름을 지웠다면 플랫폼 전체의 TLS 가 끊긴다.
	assert.False(t, shouldDeleteReleaseArtifact(artifact("secret/nullus-devsecops-stack-wildcard-tls"), "nullus-devsecops-stack"))

	// 소유자를 밝힌 것은 그 말을 따른다.
	assert.True(t, shouldDeleteReleaseArtifact(namespacedResource{Ref: "deployment.apps/harbor-core", HelmRelease: "harbor"}, "any"))
	assert.False(t, shouldDeleteReleaseArtifact(namespacedResource{Ref: "deployment.apps/nullus-api", HelmRelease: "nullus"}, "nullus"))
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
	uc.listResourcesFunc = func(_ context.Context, _ []byte, namespace string) ([]namespacedResource, error) {
		if namespace != "nullus" {
			return nil, nil
		}
		return []namespacedResource{
			{Ref: "secret/gitlab-gitlab-initial-root-password"},
			{Ref: "pvc/data-nullus-postgresql-0"},
			{Ref: "serviceaccount/default"},
			{Ref: "configmap/kube-root-ca.crt"},
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

// 삭제 정리는 HTTP 요청보다 오래 걸린다 — 릴리스 uninstall 과 PVC 재시도만으로도
// 몇 분이다. 요청 컨텍스트에 매달아 두면 게이트웨이가 연결을 끊는 순간 정리가
// 중간에서 멈추고, 마지막 단계인 볼륨·네임스페이스 회수는 아예 실행되지 않는다.
// 2026-08-21 운영에서 스택을 지운 뒤 PVC 와 네임스페이스가 함께 남은 모양이 이것이다.
func TestDeleteStack_ExecuteAsync_CleanupSurvivesRequestCancel(t *testing.T) {
	stackName := "detach stack"
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-detach",
		Name:      stackName,
		ClusterID: "cluster-detach",
		Namespace: domain.DefaultStackNamespaceFor(stackName),
		State:     domain.StateCompleted,
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return &fakeHelmInstaller{}
	})

	var mu sync.Mutex
	commands := []string{}
	uc.runKubectlFunc = func(ctx context.Context, _ []byte, args ...string) (string, error) {
		// 실제 exec 은 컨텍스트가 끊기면 즉시 실패한다. 가짜도 그렇게 행동해야
		// 이 테스트가 의미를 갖는다.
		if err := ctx.Err(); err != nil {
			return "", err
		}
		mu.Lock()
		commands = append(commands, strings.Join(args, " "))
		mu.Unlock()
		return "", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, uc.ExecuteAsync(ctx, "stk-detach"))
	// 요청이 끝났다 — 클라이언트가 끊겼든 게이트웨이가 잘랐든 컨텍스트는 죽는다.
	cancel()

	want := "delete namespace " + domain.DefaultStackNamespaceFor(stackName)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, cmd := range commands {
			if strings.Contains(cmd, want) {
				return true
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "요청이 끊겨도 네임스페이스 회수까지 끝나야 한다")
}

// ExecuteAsync 는 스택 레코드까지는 요청 안에서 지운다. 목록 새로고침이 방금
// 지운 스택을 다시 보여주면 사용자는 삭제가 실패한 줄 안다.
func TestDeleteStack_ExecuteAsync_DeletesRecordBeforeReturning(t *testing.T) {
	stackName := "record stack"
	repo := newFakeStackRepo(&domain.Stack{
		ID:        "stk-record",
		Name:      stackName,
		ClusterID: "cluster-record",
		Namespace: domain.DefaultStackNamespaceFor(stackName),
		State:     domain.StateCompleted,
	})
	provider := &fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")}

	uc := NewDeleteStack(repo, provider, func([]byte) port.HelmInstaller {
		return &fakeHelmInstaller{}
	})
	uc.runKubectlFunc = func(context.Context, []byte, ...string) (string, error) { return "", nil }

	require.NoError(t, uc.ExecuteAsync(context.Background(), "stk-record"))

	_, err := repo.GetByID(context.Background(), "stk-record")
	require.Error(t, err, "ExecuteAsync 가 돌아온 시점에 레코드는 이미 없어야 한다")
}

func TestDeleteStack_ExecuteAsync_ReportsMissingStack(t *testing.T) {
	repo := newFakeStackRepo()
	uc := NewDeleteStack(repo, nil, nil)

	err := uc.ExecuteAsync(context.Background(), "stk-missing")
	require.ErrorIs(t, err, ErrStackNotFound)
}
