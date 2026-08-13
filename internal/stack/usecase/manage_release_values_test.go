package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type fakeReleaseManager struct {
	releases     []port.ReleaseInfo
	values       map[string]map[string]any
	upgrades     []port.HelmUpgradeRequest
	upgradeError error
}

func (f *fakeReleaseManager) ListReleases(_ context.Context, namespace string) ([]port.ReleaseInfo, error) {
	out := make([]port.ReleaseInfo, 0, len(f.releases))
	for _, rel := range f.releases {
		if rel.Namespace == "" || rel.Namespace == namespace {
			out = append(out, rel)
		}
	}
	return out, nil
}

func (f *fakeReleaseManager) GetValues(_ context.Context, releaseName, _ string) (map[string]any, error) {
	values, ok := f.values[releaseName]
	if !ok {
		return nil, errors.New("release not found: " + releaseName)
	}
	return values, nil
}

func (f *fakeReleaseManager) Upgrade(_ context.Context, req port.HelmUpgradeRequest) (*port.HelmUpgradeResult, error) {
	f.upgrades = append(f.upgrades, req)
	if f.upgradeError != nil {
		return nil, f.upgradeError
	}
	return &port.HelmUpgradeResult{
		ReleaseName: req.ReleaseName,
		Namespace:   req.Namespace,
		Revision:    3,
		Status:      "deployed",
		Manifest:    "# rendered",
	}, nil
}

type fakeKubeconfigProvider struct{}

func (fakeKubeconfigProvider) GetKubeconfig(_ context.Context, _ string) ([]byte, error) {
	return []byte("apiVersion: v1\nkind: Config\n"), nil
}

func newReleaseValuesFixture(t *testing.T) (*usecase.ManageReleaseValues, *stackrepo.MemoryStackRepository, *fakeReleaseManager) {
	t.Helper()

	stackRepo := stackrepo.NewMemoryStackRepository()
	stack := &domain.Stack{
		ID:        "stk_1",
		Name:      "prod",
		ClusterID: "cls_1",
		Namespace: "nullus",
		State:     domain.StateCompleted,
		Config: domain.StackConfig{
			AccessDomain: "nullus.local",
		},
	}
	if err := stackRepo.Create(context.Background(), stack); err != nil {
		t.Fatalf("create stack: %v", err)
	}

	manager := &fakeReleaseManager{
		releases: []port.ReleaseInfo{
			{ReleaseName: "harbor", StepName: "installing_harbor", ChartName: "harbor", Namespace: "nullus", Revision: 2, Status: "deployed"},
		},
		values: map[string]map[string]any{
			"harbor": {
				"externalURL": "http://harbor.nullus.local",
				"trivy":       map[string]any{"enabled": true},
			},
		},
	}

	uc := usecase.NewManageReleaseValues(
		stackRepo,
		fakeKubeconfigProvider{},
		func(_ []byte) port.HelmReleaseManager { return manager },
		usecase.WithReleaseValuesHistory(usecase.NewManageHistory(stackrepo.NewMemoryHistoryRepository())),
	)

	return uc, stackRepo, manager
}

func TestManageReleaseValues_ListReleases(t *testing.T) {
	uc, _, _ := newReleaseValuesFixture(t)

	releases, err := uc.ListReleases(context.Background(), "stk_1")
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	if len(releases) != 1 || releases[0].ReleaseName != "harbor" {
		t.Fatalf("예상과 다른 릴리스 목록: %+v", releases)
	}
}

// 모드 A — 실제 배포된 values 전체를 그대로 읽어 온다.
func TestManageReleaseValues_GetValues_LiveMode(t *testing.T) {
	uc, _, _ := newReleaseValuesFixture(t)

	out, err := uc.GetValues(context.Background(), usecase.GetReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
	})
	if err != nil {
		t.Fatalf("get values: %v", err)
	}
	if !strings.Contains(out.YAML, "externalURL") {
		t.Fatalf("배포된 values 가 안 보인다:\n%s", out.YAML)
	}
	if out.StepName != "installing_harbor" {
		t.Fatalf("step 이 안 붙었다: %q", out.StepName)
	}
}

// 모드 B — 사용자가 저장해 둔 오버라이드만 읽는다.
func TestManageReleaseValues_GetValues_OverrideMode(t *testing.T) {
	uc, stackRepo, _ := newReleaseValuesFixture(t)

	stack, _ := stackRepo.GetByID(context.Background(), "stk_1")
	stack.Config = domain.StackConfig{
		AccessDomain:  "nullus.local",
		YAMLOverrides: map[string]string{"installing_harbor": "trivy:\n  enabled: false\n"},
	}
	if err := stackRepo.Update(context.Background(), stack); err != nil {
		t.Fatalf("update stack: %v", err)
	}

	out, err := uc.GetValues(context.Background(), usecase.GetReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeOverride,
	})
	if err != nil {
		t.Fatalf("get values: %v", err)
	}
	if strings.Contains(out.YAML, "externalURL") {
		t.Fatalf("오버라이드 모드인데 배포값까지 새어 나왔다:\n%s", out.YAML)
	}
	if !strings.Contains(out.YAML, "enabled: false") {
		t.Fatalf("오버라이드가 안 보인다:\n%s", out.YAML)
	}
}

// 모드 B 에 오버라이드가 아직 없으면 빈 문서를 준다 — 에러가 아니다.
func TestManageReleaseValues_GetValues_OverrideMode_Empty(t *testing.T) {
	uc, _, _ := newReleaseValuesFixture(t)

	out, err := uc.GetValues(context.Background(), usecase.GetReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeOverride,
	})
	if err != nil {
		t.Fatalf("get values: %v", err)
	}
	if strings.TrimSpace(out.YAML) != "" {
		t.Fatalf("오버라이드가 없으면 빈 문서여야 한다: %q", out.YAML)
	}
}

// 모드 A 적용 — 편집본이 그대로 upgrade 로 넘어가고, 재배포에 대비해
// YAMLOverrides 에도 저장된다.
func TestManageReleaseValues_Apply_LiveMode(t *testing.T) {
	uc, stackRepo, manager := newReleaseValuesFixture(t)

	out, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "externalURL: http://harbor.nullus.local\ntrivy:\n  enabled: false\n",
		ChangedBy:   "devops@nullus.dev",
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out.Revision != 3 {
		t.Fatalf("리비전이 안 돌아왔다: %+v", out)
	}
	if len(manager.upgrades) != 1 {
		t.Fatalf("upgrade 가 한 번 호출돼야 한다: %d", len(manager.upgrades))
	}
	applied := manager.upgrades[0]
	if applied.DryRun {
		t.Fatal("적용 요청인데 드라이런으로 나갔다")
	}
	trivy, _ := applied.Values["trivy"].(map[string]any)
	if trivy["enabled"] != false {
		t.Fatalf("편집한 값이 반영되지 않았다: %+v", applied.Values)
	}

	stack, _ := stackRepo.GetByID(context.Background(), "stk_1")
	cfg, ok := stack.Config.(domain.StackConfig)
	if !ok {
		t.Fatalf("config 타입이 바뀌었다: %T", stack.Config)
	}
	if !strings.Contains(cfg.YAMLOverrides["installing_harbor"], "enabled: false") {
		t.Fatalf("재배포용 오버라이드가 저장되지 않았다: %+v", cfg.YAMLOverrides)
	}
}

// 모드 B 적용 — 오버라이드는 현재 배포값 위에 얹힌다. 플랫폼이 계산해 넣은
// externalURL 은 사용자가 적지 않아도 살아남아야 한다.
func TestManageReleaseValues_Apply_OverrideMode_MergesOntoLive(t *testing.T) {
	uc, stackRepo, manager := newReleaseValuesFixture(t)

	if _, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeOverride,
		YAML:        "trivy:\n  enabled: false\n",
		ChangedBy:   "devops@nullus.dev",
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	applied := manager.upgrades[0]
	if applied.Values["externalURL"] != "http://harbor.nullus.local" {
		t.Fatalf("플랫폼이 넣은 값이 사라졌다: %+v", applied.Values)
	}
	trivy, _ := applied.Values["trivy"].(map[string]any)
	if trivy["enabled"] != false {
		t.Fatalf("오버라이드가 병합되지 않았다: %+v", applied.Values)
	}

	stack, _ := stackRepo.GetByID(context.Background(), "stk_1")
	cfg, _ := stack.Config.(domain.StackConfig)
	if strings.Contains(cfg.YAMLOverrides["installing_harbor"], "externalURL") {
		t.Fatalf("오버라이드 모드는 사용자가 적은 것만 저장해야 한다: %q", cfg.YAMLOverrides["installing_harbor"])
	}
}

// 드라이런은 클러스터에도 DB 에도 아무것도 남기지 않는다.
func TestManageReleaseValues_Apply_DryRunDoesNotPersist(t *testing.T) {
	uc, stackRepo, manager := newReleaseValuesFixture(t)

	out, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "trivy:\n  enabled: false\n",
		DryRun:      true,
		ChangedBy:   "devops@nullus.dev",
	})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !out.DryRun || !manager.upgrades[0].DryRun {
		t.Fatal("드라이런 플래그가 전달되지 않았다")
	}

	stack, _ := stackRepo.GetByID(context.Background(), "stk_1")
	cfg, _ := stack.Config.(domain.StackConfig)
	if len(cfg.YAMLOverrides) != 0 {
		t.Fatalf("드라이런인데 설정이 저장됐다: %+v", cfg.YAMLOverrides)
	}
}

// 플랫폼이 소유한 externalURL 을 지운 편집본은 경고와 함께 돌아온다.
func TestManageReleaseValues_Apply_ReportsProtectedValueWarnings(t *testing.T) {
	uc, _, _ := newReleaseValuesFixture(t)

	out, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "trivy:\n  enabled: false\n",
		DryRun:      true,
		ChangedBy:   "devops@nullus.dev",
	})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(out.Warnings) != 1 || out.Warnings[0].Path != "externalURL" {
		t.Fatalf("보호 경로 경고가 없다: %+v", out.Warnings)
	}
}

// 적용 전에 직전 설정이 이력으로 남아야 롤백할 수 있다.
func TestManageReleaseValues_Apply_SnapshotsHistory(t *testing.T) {
	stackRepo := stackrepo.NewMemoryStackRepository()
	if err := stackRepo.Create(context.Background(), &domain.Stack{
		ID: "stk_1", Namespace: "nullus", ClusterID: "cls_1", State: domain.StateCompleted,
		Config: domain.StackConfig{AccessDomain: "nullus.local"},
	}); err != nil {
		t.Fatalf("create stack: %v", err)
	}
	historyRepo := stackrepo.NewMemoryHistoryRepository()
	manager := &fakeReleaseManager{
		releases: []port.ReleaseInfo{{ReleaseName: "harbor", StepName: "installing_harbor", Namespace: "nullus"}},
		values:   map[string]map[string]any{"harbor": {"externalURL": "http://harbor.nullus.local"}},
	}
	uc := usecase.NewManageReleaseValues(
		stackRepo,
		fakeKubeconfigProvider{},
		func(_ []byte) port.HelmReleaseManager { return manager },
		usecase.WithReleaseValuesHistory(usecase.NewManageHistory(historyRepo)),
	)

	if _, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeOverride,
		YAML:        "trivy:\n  enabled: false\n",
		ChangedBy:   "devops@nullus.dev",
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	versions, err := historyRepo.ListVersions(context.Background(), "stk_1")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("설정 변경 전 스냅샷이 남지 않았다")
	}
}

// 미리보기에서 차트가 렌더되지 않는 것은 서버 오류가 아니라 편집 결과다.
// 에러로 던지면 함께 계산해 둔 보호 경로 경고까지 사라져, 사용자는 무엇을
// 잘못 건드렸는지 알 길이 없어진다.
func TestManageReleaseValues_Preview_RenderFailureKeepsWarnings(t *testing.T) {
	uc, _, manager := newReleaseValuesFixture(t)
	manager.upgradeError = errors.New("template: harbor/templates/svc.yaml:10: nil pointer")

	out, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "trivy:\n  enabled: false\n",
		DryRun:      true,
		ChangedBy:   "devops@nullus.dev",
	})
	if err != nil {
		t.Fatalf("미리보기의 렌더 실패는 에러로 올리지 않는다: %v", err)
	}
	if out.RenderError == "" {
		t.Fatal("렌더 실패 내용이 전달되지 않았다")
	}
	if len(out.Warnings) != 1 || out.Warnings[0].Path != "externalURL" {
		t.Fatalf("렌더가 실패해도 보호 경로 경고는 남아야 한다: %+v", out.Warnings)
	}
}

// 반면 실제 적용의 실패는 에러다 — 사용자는 반영되지 않았다는 것을 알아야 한다.
func TestManageReleaseValues_Apply_RenderFailureIsAnError(t *testing.T) {
	uc, _, manager := newReleaseValuesFixture(t)
	manager.upgradeError = errors.New("template render failed")

	if _, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "trivy:\n  enabled: false\n",
		ChangedBy:   "devops@nullus.dev",
	}); err == nil {
		t.Fatal("적용 실패가 에러로 오지 않았다")
	}
}

// upgrade 가 실패하면 DB 에는 아무것도 남지 않아야 한다. 적용되지 않은 설정이
// 저장되면 다음 재배포가 검증된 적 없는 값을 들고 나간다.
func TestManageReleaseValues_Apply_UpgradeFailureLeavesConfigUntouched(t *testing.T) {
	uc, stackRepo, manager := newReleaseValuesFixture(t)
	manager.upgradeError = errors.New("chart render failed")

	if _, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeOverride,
		YAML:        "trivy:\n  enabled: false\n",
		ChangedBy:   "devops@nullus.dev",
	}); err == nil {
		t.Fatal("upgrade 실패가 전달되지 않았다")
	}

	stack, _ := stackRepo.GetByID(context.Background(), "stk_1")
	cfg, _ := stack.Config.(domain.StackConfig)
	if len(cfg.YAMLOverrides) != 0 {
		t.Fatalf("실패했는데 설정이 저장됐다: %+v", cfg.YAMLOverrides)
	}
}

// 빈 오버라이드를 저장하면 커스텀을 걷어낸다는 뜻이다.
func TestManageReleaseValues_Apply_EmptyOverrideClearsKey(t *testing.T) {
	uc, stackRepo, _ := newReleaseValuesFixture(t)

	stack, _ := stackRepo.GetByID(context.Background(), "stk_1")
	stack.Config = domain.StackConfig{YAMLOverrides: map[string]string{"installing_harbor": "trivy:\n  enabled: false\n"}}
	if err := stackRepo.Update(context.Background(), stack); err != nil {
		t.Fatalf("update stack: %v", err)
	}

	if _, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeOverride,
		YAML:        "",
		ChangedBy:   "devops@nullus.dev",
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	updated, _ := stackRepo.GetByID(context.Background(), "stk_1")
	cfg, _ := updated.Config.(domain.StackConfig)
	if _, still := cfg.YAMLOverrides["installing_harbor"]; still {
		t.Fatalf("빈 오버라이드는 키를 지워야 한다: %+v", cfg.YAMLOverrides)
	}
}

// 모드 A 에서 빈 YAML 은 values 전체를 지우겠다는 뜻이 되어 위험하다. 막는다.
func TestManageReleaseValues_Apply_LiveModeRejectsEmptyYAML(t *testing.T) {
	uc, _, _ := newReleaseValuesFixture(t)

	if _, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "   \n",
		ChangedBy:   "devops@nullus.dev",
	}); err == nil {
		t.Fatal("빈 values 전체 교체가 통과했다")
	}
}

func TestManageReleaseValues_Apply_RejectsInvalidYAML(t *testing.T) {
	uc, _, _ := newReleaseValuesFixture(t)

	_, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "trivy:\n\tenabled: false\n",
		ChangedBy:   "devops@nullus.dev",
	})
	if err == nil {
		t.Fatal("잘못된 YAML 이 통과했다")
	}
	if !errors.Is(err, usecase.ErrReleaseValuesInvalidYAML) {
		t.Fatalf("YAML 오류로 분류되지 않았다: %v", err)
	}
}

// 클러스터에 없는 릴리스는 편집 대상이 아니다.
func TestManageReleaseValues_Apply_UnknownRelease(t *testing.T) {
	uc, _, _ := newReleaseValuesFixture(t)

	_, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "not-installed",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "a: 1\n",
		ChangedBy:   "devops@nullus.dev",
	})
	if !errors.Is(err, usecase.ErrReleaseNotFound) {
		t.Fatalf("없는 릴리스가 ErrReleaseNotFound 로 분류되지 않았다: %v", err)
	}
}

// 편집 결과가 정말 파싱 가능한 YAML 로 저장되는지 — 저장된 문자열을 다시
// 읽어 원래 맵이 나와야 한다.
func TestManageReleaseValues_Apply_StoredOverrideRoundTrips(t *testing.T) {
	uc, stackRepo, _ := newReleaseValuesFixture(t)

	if _, err := uc.Apply(context.Background(), usecase.ApplyReleaseValuesInput{
		StackID:     "stk_1",
		ReleaseName: "harbor",
		Mode:        usecase.ReleaseValuesModeLive,
		YAML:        "externalURL: http://harbor.nullus.local\ntrivy:\n  enabled: false\n",
		ChangedBy:   "devops@nullus.dev",
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	stack, _ := stackRepo.GetByID(context.Background(), "stk_1")
	cfg, _ := stack.Config.(domain.StackConfig)

	var decoded map[string]any
	if err := yaml.Unmarshal([]byte(cfg.YAMLOverrides["installing_harbor"]), &decoded); err != nil {
		t.Fatalf("저장된 오버라이드가 YAML 로 안 읽힌다: %v", err)
	}
	if decoded["externalURL"] != "http://harbor.nullus.local" {
		t.Fatalf("왕복에서 값이 깨졌다: %+v", decoded)
	}
}
