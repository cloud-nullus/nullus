package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type fakeGatewayReader struct {
	ip         string
	err        error
	namespace  string
	kubeconfig string
}

func (f *fakeGatewayReader) GatewayClusterIP(_ context.Context, namespace string) (string, error) {
	f.namespace = namespace
	return f.ip, f.err
}

const oldRunnerConfig = `[[runners]]
  [runners.kubernetes]
    [[runners.kubernetes.host_aliases]]
      ip = "10.96.45.47"
      hostnames = ["gitlab.nullus.local", "registry.nullus.local"]
`

func newRepinFixture(t *testing.T, gatewayIP string, accessDomain string) (*usecase.GatewayAliasRepin, *fakeReleaseManager, *fakeGatewayReader) {
	t.Helper()
	repo := stackrepo.NewMemoryStackRepository()
	if err := repo.Create(context.Background(), &domain.Stack{
		ID: "stk_1", Name: "prod", ClusterID: "cls_1", Namespace: "nullus", State: domain.StateCompleted,
		Config: domain.StackConfig{AccessDomain: accessDomain},
	}); err != nil {
		t.Fatal(err)
	}
	manager := &fakeReleaseManager{
		releases: []port.ReleaseInfo{
			{ReleaseName: "argo-cd", ChartName: "argo-cd", Namespace: "restored-ns"},
			{ReleaseName: "gitlab-runner", ChartName: "gitlab-runner", Namespace: "restored-ns"},
			{ReleaseName: "gitlab", ChartName: "gitlab", Namespace: "restored-ns"},
		},
		values: map[string]map[string]any{
			"argo-cd": {"global": map[string]any{"hostAliases": []any{
				map[string]any{"ip": "10.96.45.47", "hostnames": []any{"gitlab.nullus.local", "registry.nullus.local"}},
			}}},
			"gitlab-runner": {"runners": map[string]any{"config": oldRunnerConfig}},
			"gitlab":        {"global": map[string]any{"hosts": map[string]any{"domain": "nullus.local"}}},
		},
	}
	reader := &fakeGatewayReader{ip: gatewayIP}
	uc := usecase.NewGatewayAliasRepin(repo, fakeKubeconfigProvider{},
		func(kc []byte) port.HelmReleaseManager { manager.kubeconfig = string(kc); return manager },
		func(kc []byte) (port.GatewayAddressReader, error) { reader.kubeconfig = string(kc); return reader, nil },
	)
	return uc, manager, reader
}

// failUpgradeFor 는 한 릴리스의 적용만 실패시킨다(Argo CD 차트를 다시 받지 못한 경우 등).
type failUpgradeFor struct {
	*fakeReleaseManager
	release string
}

func (f *failUpgradeFor) Upgrade(ctx context.Context, req port.HelmUpgradeRequest) (*port.HelmUpgradeResult, error) {
	if req.ReleaseName == f.release {
		f.upgrades = append(f.upgrades, req)
		return nil, errors.New("locate chart argo-cd 7.7.16: dial tcp: no route to host")
	}
	return f.fakeReleaseManager.Upgrade(ctx, req)
}

// 복구로 게이트웨이 주소가 바뀌었다(다른 클러스터, ClusterIP 를 담지 않은 옛 백업). 러너와 Argo CD 의
// values 를 새 주소로 바꿔 다시 적용한다 — 그래야 릴리스 Secret·러너 ConfigMap·Argo CD 파드가 함께
// 맞고, 나중에 values 를 고쳐도 옛 주소가 돌아오지 않는다.
func TestGatewayAliasRepin_UpdatesRunnerAndArgoCD(t *testing.T) {
	uc, manager, reader := newRepinFixture(t, "10.96.200.9", "nullus.local")

	ip, updated, err := uc.RepinGatewayAliases(context.Background(), "stk_1", "restored-ns", nil)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "10.96.200.9" || reader.namespace != "restored-ns" {
		t.Fatalf("ip=%s ns=%s", ip, reader.namespace)
	}
	if strings.Join(updated, ",") != "argo-cd,gitlab-runner" {
		t.Fatalf("updated=%v", updated)
	}
	if len(manager.upgrades) != 2 {
		t.Fatalf("upgrades=%d", len(manager.upgrades))
	}
	for _, up := range manager.upgrades {
		if up.Namespace != "restored-ns" || up.DryRun {
			t.Fatalf("upgrade %+v", up)
		}
	}
	argo := manager.upgrades[0].Values["global"].(map[string]any)["hostAliases"].([]any)[0].(map[string]any)
	if argo["ip"] != "10.96.200.9" {
		t.Fatalf("argo ip=%v", argo["ip"])
	}
	runner := manager.upgrades[1].Values["runners"].(map[string]any)["config"].(string)
	if !strings.Contains(runner, `ip = "10.96.200.9"`) || strings.Contains(runner, "10.96.45.47") {
		t.Fatalf("runner config=%s", runner)
	}
}

// 주소가 그대로면(같은 클러스터로 되돌림) 아무것도 다시 적용하지 않는다 — 불필요한 재기동을 만들지 않는다.
func TestGatewayAliasRepin_NoopWhenAddressUnchanged(t *testing.T) {
	uc, manager, _ := newRepinFixture(t, "10.96.45.47", "nullus.local")

	_, updated, err := uc.RepinGatewayAliases(context.Background(), "stk_1", "restored-ns", nil)
	if err != nil || len(updated) != 0 || len(manager.upgrades) != 0 {
		t.Fatalf("updated=%v upgrades=%d err=%v", updated, len(manager.upgrades), err)
	}
}

// 접근 도메인이 없는 스택은 박아 둔 주소가 없다.
func TestGatewayAliasRepin_NothingPinnedWithoutAccessDomain(t *testing.T) {
	uc, manager, _ := newRepinFixture(t, "10.96.200.9", "")

	_, updated, err := uc.RepinGatewayAliases(context.Background(), "stk_1", "restored-ns", nil)
	if err != nil || len(updated) != 0 || len(manager.upgrades) != 0 {
		t.Fatalf("updated=%v upgrades=%d err=%v", updated, len(manager.upgrades), err)
	}
}

// 게이트웨이 주소를 못 읽으면 알린다. 조용히 넘기면 파이프라인이 실패할 때까지 아무도 모른다.
func TestGatewayAliasRepin_FailsWhenGatewayMissing(t *testing.T) {
	uc, manager, reader := newRepinFixture(t, "", "nullus.local")
	reader.err = errors.New("게이트웨이 Service 가 없다")

	_, _, err := uc.RepinGatewayAliases(context.Background(), "stk_1", "restored-ns", nil)
	if err == nil || len(manager.upgrades) != 0 {
		t.Fatalf("err=%v upgrades=%d", err, len(manager.upgrades))
	}
}

// 네임스페이스를 비우면 스택의 네임스페이스다.
func TestGatewayAliasRepin_DefaultsToStackNamespace(t *testing.T) {
	uc, _, reader := newRepinFixture(t, "10.96.45.47", "nullus.local")

	if _, _, err := uc.RepinGatewayAliases(context.Background(), "stk_1", " ", nil); err != nil {
		t.Fatal(err)
	}
	if reader.namespace != "nullus" {
		t.Fatalf("namespace=%s", reader.namespace)
	}
}

// Argo CD 는 서브차트를 잃은 차트라 같은 버전을 저장소에서 다시 받는다 — 닿지 못하면(에어갭·외부 차단)
// 그 릴리스만 실패다. 저장된 차트로 되는 러너까지 건너뛰면 CI 가 계속 멈춰 있다.
func TestGatewayAliasRepin_ContinuesPastAFailedRelease(t *testing.T) {
	uc, manager, _ := newRepinFixture(t, "10.96.200.9", "nullus.local")
	failing := &failUpgradeFor{fakeReleaseManager: manager, release: "argo-cd"}
	uc = usecase.NewGatewayAliasRepin(stackRepoOf(t, "nullus.local"), fakeKubeconfigProvider{},
		func(_ []byte) port.HelmReleaseManager { return failing },
		func(_ []byte) (port.GatewayAddressReader, error) { return &fakeGatewayReader{ip: "10.96.200.9"}, nil },
	)

	_, updated, err := uc.RepinGatewayAliases(context.Background(), "stk_1", "restored-ns", nil)

	if err == nil || !strings.Contains(err.Error(), "argo-cd") {
		t.Fatalf("err=%v", err)
	}
	if strings.Join(updated, ",") != "gitlab-runner" {
		t.Fatalf("updated=%v — 러너는 맞춰야 한다", updated)
	}
}

// full 복구는 플랫폼 DB 를 백업 시점으로 되돌린다 — 그 사이 스택을 다른 클러스터로 옮겼다면 DB 의
// kubeconfig 는 옛 클러스터다. 복구가 실제로 되돌린 클러스터의 kubeconfig 를 넘기면 그것을 쓴다.
func TestGatewayAliasRepin_UsesGivenKubeconfig(t *testing.T) {
	uc, manager, reader := newRepinFixture(t, "10.96.200.9", "nullus.local")

	if _, _, err := uc.RepinGatewayAliases(context.Background(), "stk_1", "restored-ns", []byte("restored-cluster")); err != nil {
		t.Fatal(err)
	}
	if reader.kubeconfig != "restored-cluster" || manager.kubeconfig != "restored-cluster" {
		t.Fatalf("reader=%q manager=%q", reader.kubeconfig, manager.kubeconfig)
	}
}

func stackRepoOf(t *testing.T, accessDomain string) *stackrepo.MemoryStackRepository {
	t.Helper()
	repo := stackrepo.NewMemoryStackRepository()
	if err := repo.Create(context.Background(), &domain.Stack{
		ID: "stk_1", Name: "prod", ClusterID: "cls_1", Namespace: "nullus", State: domain.StateCompleted,
		Config: domain.StackConfig{AccessDomain: accessDomain},
	}); err != nil {
		t.Fatal(err)
	}
	return repo
}
