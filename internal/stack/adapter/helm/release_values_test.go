package helm

import (
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func rev(version int, status release.Status) *release.Release {
	return &release.Release{Version: version, Info: &release.Info{Status: status}}
}

// 업그레이드가 실패하면 Helm 은 실패한 리비전을 최신으로 남긴다. 그 리비전의
// values 에는 적용된 적 없는 값이 들어 있으므로, 편집기가 그걸 보여 주면
// 사용자는 돌지도 않는 설정을 현재 값으로 착각한다.
func TestReadableRevision_SkipsFailedLatest(t *testing.T) {
	history := []*release.Release{
		rev(1, release.StatusSuperseded),
		rev(2, release.StatusSuperseded),
		rev(3, release.StatusDeployed),
		rev(4, release.StatusFailed),
	}

	if got := readableRevision(history); got != 3 {
		t.Fatalf("실제 배포된 리비전을 골라야 한다: got %d, want 3", got)
	}
}

func TestReadableRevision_UsesLatestDeployed(t *testing.T) {
	history := []*release.Release{
		rev(1, release.StatusSuperseded),
		rev(2, release.StatusDeployed),
	}

	if got := readableRevision(history); got != 2 {
		t.Fatalf("가장 최근의 deployed 를 골라야 한다: got %d, want 2", got)
	}
}

// deployed 가 하나도 없으면 (설치 자체가 실패한 릴리스) 최신을 그대로 쓴다.
// 0 은 helm 에서 "최신" 을 뜻한다.
func TestReadableRevision_NoDeployedFallsBackToLatest(t *testing.T) {
	history := []*release.Release{rev(1, release.StatusFailed)}

	if got := readableRevision(history); got != 0 {
		t.Fatalf("deployed 가 없으면 최신(0)으로 떨어져야 한다: got %d", got)
	}
}

func TestReadableRevision_EmptyHistory(t *testing.T) {
	if got := readableRevision(nil); got != 0 {
		t.Fatalf("이력이 없으면 최신(0): got %d", got)
	}
}

// Info 가 없는 리코드가 섞여도 죽지 않아야 한다.
func TestReadableRevision_ToleratesMissingInfo(t *testing.T) {
	history := []*release.Release{
		{Version: 1},
		rev(2, release.StatusDeployed),
		nil,
	}

	if got := readableRevision(history); got != 2 {
		t.Fatalf("불완전한 이력에서 deployed 를 못 찾았다: got %d", got)
	}
}

// 릴리스 → 단계 매핑이 비면 편집한 values 를 어느 오버라이드 키로 저장할지
// 알 수 없다. 그러면 다음 재배포에서 사용자의 편집이 조용히 사라진다.
func TestStepForRelease_CoversEveryInstalledRelease(t *testing.T) {
	for _, release := range domain.InstalledHelmReleaseNames {
		if step := StepForRelease(release); step == "" {
			t.Errorf("릴리스 %q 에 대응하는 설치 단계가 없다 — 편집이 재배포에서 유실된다", release)
		}
	}
}

// 매핑된 단계는 실제로 존재하는 차트 단계여야 한다. 오타 하나면 오버라이드가
// 아무 단계에도 걸리지 않는 키로 저장된다.
func TestStepForRelease_MapsToRealChartSteps(t *testing.T) {
	for release, step := range releaseStepNames {
		if _, ok := defaultChartSpecForStep(step); !ok {
			t.Errorf("릴리스 %q 가 가리키는 단계 %q 에 차트 스펙이 없다", release, step)
		}
	}
}

// Helm 은 릴리스에 차트를 저장할 때 의존 서브차트를 함께 담지 않는다
// (chart.Chart 의 Raw 는 json:"-", dependencies 는 비공개 필드다).
// 그래서 bitnami common 같은 라이브러리 차트에 기대는 차트는 저장본만으로는
// 렌더되지 않는다 — "no template \"common.names.fullname\"" 로 깨진다.
func TestStoredChartLostDependencies_DetectsMissingSubchart(t *testing.T) {
	stored := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         "postgresql",
			Version:      "18.8.8",
			Dependencies: []*chart.Dependency{{Name: "common", Version: "2.x"}},
		},
	}

	if !storedChartLostDependencies(stored) {
		t.Fatal("의존성이 유실된 저장 차트를 못 알아봤다")
	}
}

func TestStoredChartLostDependencies_SelfContainedChartIsFine(t *testing.T) {
	stored := &chart.Chart{Metadata: &chart.Metadata{Name: "grafana", Version: "8.9.0"}}

	if storedChartLostDependencies(stored) {
		t.Fatal("의존성이 없는 차트를 다시 받아 올 이유가 없다")
	}
}

func TestStoredChartLostDependencies_DependencyPresentIsFine(t *testing.T) {
	stored := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         "postgresql",
			Dependencies: []*chart.Dependency{{Name: "common"}},
		},
	}
	stored.AddDependency(&chart.Chart{Metadata: &chart.Metadata{Name: "common"}})

	if storedChartLostDependencies(stored) {
		t.Fatal("의존성이 살아 있는 차트를 유실로 봤다")
	}
}

func TestStoredChartLostDependencies_NilIsSafe(t *testing.T) {
	if storedChartLostDependencies(nil) {
		t.Fatal("nil 차트에서 true 를 내면 안 된다")
	}
	if storedChartLostDependencies(&chart.Chart{}) {
		t.Fatal("메타데이터가 없는 차트에서 true 를 내면 안 된다")
	}
}

func TestStepForRelease_UnknownReleaseIsEmpty(t *testing.T) {
	if step := StepForRelease("some-random-release"); step != "" {
		t.Fatalf("모르는 릴리스는 빈 단계여야 한다: %q", step)
	}
}

func TestStepForRelease_TrimsWhitespace(t *testing.T) {
	if step := StepForRelease("  gitlab  "); step != "installing_gitlab" {
		t.Fatalf("공백이 붙은 이름을 못 읽는다: %q", step)
	}
}
