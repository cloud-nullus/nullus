package helm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helm 은 cluster-scoped 리소스의 소유권을 meta.helm.sh/release-{name,namespace}
// 로 판별한다. 스택을 지웠다가 다른 네임스페이스로 재설치하면 옛 네임스페이스가
// 박혀 있어
//
//	exists and cannot be imported into the current release
//
// 로 설치가 막힌다. 그래서 설치 전에 소유권을 인수한다.
//
// 다만 한 클러스터에 여러 스택이 살 수 있으므로(create_stack 의 이름 유일성이
// 클러스터 단위), 옛 릴리스가 아직 살아 있으면 절대 인수하면 안 된다 — 인수하면
// 다른 스택이 쓰는 리소스를 탈취하고, 그 스택을 지울 때 함께 삭제된다.
func TestShouldAdoptClusterScopedResource(t *testing.T) {
	cases := []struct {
		name            string
		owner           clusterScopedOwner
		releaseName     string
		targetNamespace string
		oldReleaseAlive bool
		want            bool
	}{
		{
			name:            "옛 릴리스가 사라졌으면 인수한다",
			owner:           clusterScopedOwner{Name: "applications.argoproj.io", ReleaseName: "argo-cd", ReleaseNamespace: "devsecops"},
			releaseName:     "argo-cd",
			targetNamespace: "devsecops2",
			oldReleaseAlive: false,
			want:            true,
		},
		{
			name:            "옛 릴리스가 살아 있으면 인수하지 않는다 (다른 스택 보호)",
			owner:           clusterScopedOwner{Name: "applications.argoproj.io", ReleaseName: "argo-cd", ReleaseNamespace: "team-a"},
			releaseName:     "argo-cd",
			targetNamespace: "team-b",
			oldReleaseAlive: true,
			want:            false,
		},
		{
			name:            "이미 우리 네임스페이스 소유면 건드리지 않는다",
			owner:           clusterScopedOwner{Name: "grafana-clusterrole", ReleaseName: "grafana", ReleaseNamespace: "devsecops2"},
			releaseName:     "grafana",
			targetNamespace: "devsecops2",
			oldReleaseAlive: false,
			want:            false,
		},
		{
			name:            "다른 릴리스 소유는 건드리지 않는다",
			owner:           clusterScopedOwner{Name: "cert-manager-webhook", ReleaseName: "cert-manager", ReleaseNamespace: "cert-manager"},
			releaseName:     "argo-cd",
			targetNamespace: "devsecops2",
			oldReleaseAlive: false,
			want:            false,
		},
		{
			name:            "release-namespace 주석이 없으면 인수한다 (지킬 소유자가 없다)",
			owner:           clusterScopedOwner{Name: "externalsecrets.external-secrets.io", ReleaseName: "external-secrets", ReleaseNamespace: ""},
			releaseName:     "external-secrets",
			targetNamespace: "devsecops2",
			oldReleaseAlive: false,
			want:            true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAdoptClusterScopedResource(tc.owner, tc.releaseName, tc.targetNamespace, tc.oldReleaseAlive)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseClusterScopedOwners(t *testing.T) {
	output := []byte(`applications.argoproj.io      argo-cd            devsecops
grafana-clusterrole          grafana            devsecops
some-unmanaged-role          <none>             <none>
`)

	owners := parseClusterScopedOwners(output)
	require.Len(t, owners, 3)

	assert.Equal(t, "applications.argoproj.io", owners[0].Name)
	assert.Equal(t, "argo-cd", owners[0].ReleaseName)
	assert.Equal(t, "devsecops", owners[0].ReleaseNamespace)

	// <none> 은 빈 값으로 정규화된다 — Helm 이 소유하지 않는 리소스다.
	assert.Equal(t, "some-unmanaged-role", owners[2].Name)
	assert.Empty(t, owners[2].ReleaseName)
	assert.Empty(t, owners[2].ReleaseNamespace)
}

func TestParseClusterScopedOwners_IgnoresMalformedLines(t *testing.T) {
	output := []byte("\n짧은줄\n\nname rel ns\n")

	owners := parseClusterScopedOwners(output)
	require.Len(t, owners, 1)
	assert.Equal(t, "name", owners[0].Name)
}

// Helm 이 소유하지 않는 리소스(release-name 주석 없음)는 절대 건드리지 않는다.
func TestShouldAdoptClusterScopedResource_SkipsUnmanaged(t *testing.T) {
	owner := clusterScopedOwner{Name: "system:controller", ReleaseName: "", ReleaseNamespace: ""}

	assert.False(t, shouldAdoptClusterScopedResource(owner, "argo-cd", "devsecops2", false))
}

// 가짜 kubeconfig 에서는 클러스터를 건드리지 않고 조용히 빠져나가야 한다.
func TestAdoptClusterScopedResources_NoopWithoutKubeconfig(t *testing.T) {
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "devsecops2")

	called := false
	original := releaseAliveInNamespace
	releaseAliveInNamespace = func(_ context.Context, _ *Orchestrator, _, _ string) bool {
		called = true
		return false
	}
	t.Cleanup(func() { releaseAliveInNamespace = original })

	o.adoptClusterScopedResources(context.Background(), "argo-cd", "devsecops2")

	assert.False(t, called, "kubeconfig 가 없으면 클러스터 조회를 하지 않아야 한다")
}

// 릴리스 이름이나 네임스페이스가 비면 아무것도 하지 않는다.
func TestAdoptClusterScopedResources_RequiresReleaseAndNamespace(t *testing.T) {
	o := NewOrchestrator(nil, []byte("apiVersion: v1\nclusters: []\n"), "devsecops2")

	called := false
	original := releaseAliveInNamespace
	releaseAliveInNamespace = func(_ context.Context, _ *Orchestrator, _, _ string) bool {
		called = true
		return false
	}
	t.Cleanup(func() { releaseAliveInNamespace = original })

	o.adoptClusterScopedResources(context.Background(), "", "devsecops2")
	o.adoptClusterScopedResources(context.Background(), "argo-cd", "")

	assert.False(t, called)
}
