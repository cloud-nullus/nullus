//go:build integration

package helm

import (
	"context"
	"testing"
)

// 실제 클러스터에서 cluster-scoped 소유권 인수를 검증한다.
//
// 단위 테스트는 판단 로직만 덮는다. 여기서는 kubectl 조회 형태(custom-columns /
// jsonpath 의 이스케이프된 점 표기)와 patch 가 실제로 동작하는지, 그리고 안전
// 가드가 살아 있는 릴리스를 지켜 주는지를 확인한다.
//
// 실행:
//
//	NULLUS_IT_KUBECONFIG=~/.kube/config go test -tags=integration \
//	  ./internal/stack/adapter/helm/ -run TestIntegration_ClusterScopedAdoption -v
const (
	itAdoptionKind     = "clusterrole"
	itAdoptionTestName = "nullus-adoption-probe"
)

// 조회 형태가 실제 클러스터에서 동작하는지 확인한다.
func TestIntegration_ClusterScopedAdoption_QueryShapes(t *testing.T) {
	kubeconfig := loadITKubeconfig(t)
	o := NewOrchestrator(nil, kubeconfig, "default")
	ctx := context.Background()

	owners, err := o.listClusterScopedOwners(ctx, itAdoptionKind)
	if err != nil {
		t.Fatalf("listClusterScopedOwners 실패: %v", err)
	}
	if len(owners) == 0 {
		t.Fatal("clusterrole 이 하나도 조회되지 않았다 — custom-columns 형태를 확인하라")
	}

	// Helm 이 소유한 항목이 최소 하나는 파싱되어야 한다.
	managed := 0
	for _, owner := range owners {
		if owner.ReleaseName != "" && owner.ReleaseNamespace != "" {
			managed++
		}
	}
	if managed == 0 {
		t.Fatal("Helm 소유 clusterrole 을 하나도 파싱하지 못했다 — 주석 경로 이스케이프를 확인하라")
	}
	t.Logf("clusterrole %d개 중 Helm 소유 %d개 파싱", len(owners), managed)
}

// 살아 있는 릴리스는 인수 대상에서 제외되어야 한다.
func TestIntegration_ClusterScopedAdoption_ProtectsLiveRelease(t *testing.T) {
	kubeconfig := loadITKubeconfig(t)
	o := NewOrchestrator(nil, kubeconfig, "default")
	ctx := context.Background()

	owners, err := o.listClusterScopedOwners(ctx, itAdoptionKind)
	if err != nil {
		t.Fatalf("listClusterScopedOwners 실패: %v", err)
	}

	// 실제로 살아 있는 릴리스가 소유한 항목을 하나 고른다.
	var live clusterScopedOwner
	for _, owner := range owners {
		if owner.ReleaseName == "" || owner.ReleaseNamespace == "" {
			continue
		}
		if releaseAliveInNamespace(ctx, o, owner.ReleaseName, owner.ReleaseNamespace) {
			live = owner
			break
		}
	}
	if live.Name == "" {
		t.Skip("살아 있는 Helm 릴리스가 소유한 clusterrole 이 없다")
	}

	// 다른 네임스페이스가 같은 릴리스 이름으로 설치를 시도하는 상황.
	alive := releaseAliveInNamespace(ctx, o, live.ReleaseName, live.ReleaseNamespace)
	if !alive {
		t.Fatalf("릴리스 %s/%s 가 살아 있다고 판정되어야 한다", live.ReleaseNamespace, live.ReleaseName)
	}

	if shouldAdoptClusterScopedResource(live, live.ReleaseName, "some-other-namespace", alive) {
		t.Fatalf("살아 있는 릴리스 %s/%s 의 %s 를 인수하려 했다 — 다른 스택을 탈취한다",
			live.ReleaseNamespace, live.ReleaseName, live.Name)
	}
	t.Logf("보호 확인: %s (owner=%s/%s)", live.Name, live.ReleaseNamespace, live.ReleaseName)
}

// 죽은 네임스페이스 소유 리소스는 실제로 인수되어야 한다.
func TestIntegration_ClusterScopedAdoption_AdoptsOrphan(t *testing.T) {
	kubeconfig := loadITKubeconfig(t)
	o := NewOrchestrator(nil, kubeconfig, "adoption-target-ns")
	ctx := context.Background()

	// 존재하지 않는 네임스페이스가 소유한 것처럼 보이는 ClusterRole 을 만든다.
	manifest := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ` + itAdoptionTestName + `
  labels:
    app.kubernetes.io/managed-by: Helm
  annotations:
    meta.helm.sh/release-name: ` + itAdoptionTestName + `
    meta.helm.sh/release-namespace: this-namespace-does-not-exist
rules: []
`
	if err := o.applyManifest(ctx, "default", manifest); err != nil {
		t.Fatalf("probe ClusterRole 생성 실패: %v", err)
	}
	t.Cleanup(func() {
		_, _ = o.runKubectl(context.Background(), "delete", itAdoptionKind, itAdoptionTestName, "--ignore-not-found")
	})

	before, err := o.clusterScopedOwnerOf(ctx, itAdoptionKind, itAdoptionTestName)
	if err != nil {
		t.Fatalf("소유권 조회 실패: %v", err)
	}
	if before.ReleaseNamespace != "this-namespace-does-not-exist" {
		t.Fatalf("사전 조건이 맞지 않다: %+v", before)
	}

	o.adoptClusterScopedResources(ctx, itAdoptionTestName, "adoption-target-ns")

	after, err := o.clusterScopedOwnerOf(ctx, itAdoptionKind, itAdoptionTestName)
	if err != nil {
		t.Fatalf("인수 후 소유권 조회 실패: %v", err)
	}
	if after.ReleaseNamespace != "adoption-target-ns" {
		t.Fatalf("인수되지 않았다: release-namespace=%q (기대: adoption-target-ns)", after.ReleaseNamespace)
	}
	t.Logf("인수 확인: %s → %s", before.ReleaseNamespace, after.ReleaseNamespace)
}
