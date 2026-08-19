package domain

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 기준 벡터는 stack_resource_defaults 시드값이다 (000045 계열 마이그레이션).
// 계획 계산은 이 벡터에 배수를 곱하는 일이므로, 여기가 흔들리면 아래 기대값도
// 전부 흔들린다.
var planningBase = map[string]ResourceVector{
	"gitea":      {CPURequest: 1, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4, StorageRequestGi: 15, StorageLimitGi: 30},
	"jenkins":    {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 20, StorageLimitGi: 40},
	"harbor":     {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 40, StorageLimitGi: 80},
	"argocd":     {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 3, MemoryLimitGi: 6, StorageRequestGi: 5, StorageLimitGi: 10},
	"gitlab":     {CPURequest: 4, CPULimit: 8, MemoryRequestGi: 8, MemoryLimitGi: 16, StorageRequestGi: 30, StorageLimitGi: 60},
	"prometheus": {CPURequest: 1, CPULimit: 2, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 20, StorageLimitGi: 40},
}

// 설치 마법사(web/src/features/stack/utils/install-planning-utils.ts)가 같은
// 입력으로 내놓는 값이다. 계산이 프론트에만 있던 동안 헤드리스 설치는 계획을
// 적용할 수 없었고, 옮기면서 숫자가 달라지면 UI 설치와 API 설치가 서로 다른
// 크기로 깔린다 — 그래서 이식의 성공 기준은 "동작한다"가 아니라 "같다"이다.
func TestPlanResourceVector_MatchesInstallWizard(t *testing.T) {
	cases := []struct {
		profile string
		slot    string
		tool    string
		want    ResourceVector
	}{
		// local — Lite 템플릿이 8Gi 노드에 들어가는 이유가 이 줄들이다.
		{PlanningProfileLocal, SlotSourceRepository, "gitea", ResourceVector{0.5, 0.5, 0.5, 1, 3.5, 7}},
		{PlanningProfileLocal, SlotCICDPlatform, "jenkins", ResourceVector{0.5, 0.5, 0.5, 1.5, 4.5, 9.5}},
		{PlanningProfileLocal, SlotContainerRegistry, "harbor", ResourceVector{0.5, 1, 1, 2, 9, 18}},
		{PlanningProfileLocal, SlotCDTool, "argocd", ResourceVector{0.5, 1, 0.5, 1.5, 1, 2.5}},
		{PlanningProfileLocal, SlotSourceRepository, "gitlab", ResourceVector{1, 2, 2, 3.5, 7, 13.5}},
		{PlanningProfileLocal, SlotMonitoringCollection, "prometheus", ResourceVector{0.5, 0.5, 1, 1.5, 4.5, 9}},

		// startup
		{PlanningProfileStartup, SlotSourceRepository, "gitea", ResourceVector{0.5, 1.5, 1.5, 3, 10.5, 21.5}},
		{PlanningProfileStartup, SlotCICDPlatform, "jenkins", ResourceVector{1, 2, 2, 4, 14.5, 28.5}},
		{PlanningProfileStartup, SlotContainerRegistry, "harbor", ResourceVector{1.5, 3, 3, 5.5, 28, 56}},
		{PlanningProfileStartup, SlotCDTool, "argocd", ResourceVector{1.5, 3, 2, 4.5, 4, 7.5}},

		// enterprise
		{PlanningProfileEnterprise, SlotSourceRepository, "gitea", ResourceVector{1.5, 3, 3, 6, 23.5, 47}},
		{PlanningProfileEnterprise, SlotCICDPlatform, "jenkins", ResourceVector{5.5, 11, 10, 20.5, 31, 61.5}},
		{PlanningProfileEnterprise, SlotContainerRegistry, "harbor", ResourceVector{3, 6.5, 6, 12, 63.5, 127}},
		{PlanningProfileEnterprise, SlotCDTool, "argocd", ResourceVector{3, 6.5, 4.5, 9, 6.5, 13}},
	}

	for _, tc := range cases {
		t.Run(tc.profile+"/"+tc.slot+"/"+tc.tool, func(t *testing.T) {
			got := PlanResourceVector(tc.profile, tc.slot, planningBase[tc.tool])
			assert.Equal(t, tc.want, got)
		})
	}
}

// standard 는 마법사의 기본 프로파일이다. 옵션을 건드리지 않으면 배수가 정확히
// 1 이라 관리자 기본값이 그대로 나와야 한다 — 여기가 어긋나면 프로파일을 고르지
// 않은 기존 스택의 설치 크기가 조용히 바뀐다.
func TestPlanResourceVector_StandardKeepsAdminDefaults(t *testing.T) {
	for _, tool := range []struct {
		slot string
		key  string
	}{
		{SlotSourceRepository, "gitea"},
		{SlotCICDPlatform, "jenkins"},
		{SlotContainerRegistry, "harbor"},
		{SlotCDTool, "argocd"},
		{SlotSourceRepository, "gitlab"},
		{SlotMonitoringCollection, "prometheus"},
	} {
		t.Run(tool.key, func(t *testing.T) {
			base := planningBase[tool.key]
			assert.Equal(t, base, PlanResourceVector(PlanningProfileStandard, tool.slot, base))
		})
	}
}

// 프로파일이 커질수록 같은 도구가 더 크게 깔려야 한다. 개별 숫자가 아니라 순서를
// 고정해 두면, 계수를 손볼 때 방향이 뒤집히는 실수를 잡을 수 있다.
func TestPlanResourceVector_ProfilesAreMonotonic(t *testing.T) {
	order := []string{PlanningProfileLocal, PlanningProfileStartup, PlanningProfileStandard, PlanningProfileEnterprise}

	for _, tool := range []struct {
		slot string
		key  string
	}{
		{SlotSourceRepository, "gitea"},
		{SlotContainerRegistry, "harbor"},
		{SlotCDTool, "argocd"},
	} {
		t.Run(tool.key, func(t *testing.T) {
			var prev ResourceVector
			for i, profile := range order {
				got := PlanResourceVector(profile, tool.slot, planningBase[tool.key])
				if i > 0 {
					assert.GreaterOrEqual(t, got.MemoryRequestGi, prev.MemoryRequestGi,
						"%s 는 %s 보다 작을 수 없다", profile, order[i-1])
					assert.GreaterOrEqual(t, got.StorageRequestGi, prev.StorageRequestGi,
						"%s 는 %s 보다 작을 수 없다", profile, order[i-1])
				}
				prev = got
			}
		})
	}
}

// 프로파일별 기본 옵션값. 마법사가 화면을 열 때 채워 넣는 값과 같아야 한다 —
// 사용자가 아무것도 만지지 않은 상태의 계획이 곧 헤드리스 설치의 계획이다.
func TestDefaultPlanningOptions_MatchesInstallWizard(t *testing.T) {
	local := DefaultPlanningOptions(PlanningProfileLocal, SlotCICDPlatform)
	require.NotEmpty(t, local)
	assert.Equal(t, 6.0, local["developers"])        // 20 × 0.3 (기타)
	assert.Equal(t, 1.0, local["concurrentRunners"]) // 4 × 0.25 (동시성)
	assert.Equal(t, 30.0, local["dailyCommits"])     // 120 × 0.25 (처리량)

	standard := DefaultPlanningOptions(PlanningProfileStandard, SlotCICDPlatform)
	assert.Equal(t, 20.0, standard["developers"])
	assert.Equal(t, 4.0, standard["concurrentRunners"])
	assert.Equal(t, 120.0, standard["dailyCommits"])
}

// 옵션 키의 성격을 무엇으로 읽느냐가 계수를 정한다. 분류가 바뀌면 기본값이
// 통째로 움직이므로 대표 키를 고정해 둔다.
func TestProfileFactorByOption_ClassifiesOptionKinds(t *testing.T) {
	// 보관 기간은 로컬에서 가장 크게 줄인다 — 스토리지를 직접 먹는 값이다.
	assert.Equal(t, 0.2, ProfileFactorByOption(PlanningProfileLocal, "retentionDays"))
	// 주기는 반대로 늘린다. 주기가 길수록 부하가 준다.
	assert.Equal(t, 2.5, ProfileFactorByOption(PlanningProfileLocal, "scrapeIntervalSec"))
	assert.Equal(t, 0.25, ProfileFactorByOption(PlanningProfileLocal, "concurrentRunners"))
	assert.Equal(t, 0.25, ProfileFactorByOption(PlanningProfileLocal, "dailyCommits"))
	// developers 는 처리량 어휘에 걸리지 않는다 (calls/events/... 어디에도 없다).
	assert.Equal(t, 0.3, ProfileFactorByOption(PlanningProfileLocal, "developers"))

	// standard 는 언제나 1 — 기준 프로파일이다.
	for _, key := range []string{"retentionDays", "scrapeIntervalSec", "concurrentRunners", "developers"} {
		assert.Equal(t, 1.0, ProfileFactorByOption(PlanningProfileStandard, key))
	}
}

// 알 수 없는 슬롯은 계획 대상이 아니다. 배수를 1 로 두어 기준 벡터가 그대로
// 나가야 한다 — 0 을 곱해 requests 가 사라지면 파드가 무제한으로 뜬다.
func TestPlanResourceVector_UnknownSlotKeepsBase(t *testing.T) {
	base := planningBase["harbor"]
	assert.Equal(t, base, PlanResourceVector(PlanningProfileLocal, "artifacts.nonexistent", base))
}

// 빈 프로파일은 standard 로 읽는다 (NormalizePlanningProfile 과 같은 규칙).
func TestPlanResourceVector_EmptyProfileFallsBackToStandard(t *testing.T) {
	base := planningBase["argocd"]
	assert.Equal(t, PlanResourceVector(PlanningProfileStandard, SlotCDTool, base),
		PlanResourceVector("", SlotCDTool, base))
}

// 0 인 축은 0 으로 남아야 한다. cert-manager 처럼 스토리지를 안 쓰는 도구에
// 0.5Gi 를 억지로 채우면 PVC 가 없는 차트에 값을 실어 보내게 된다.
func TestPlanResourceVector_ZeroAxisStaysZero(t *testing.T) {
	base := ResourceVector{CPURequest: 0.5, CPULimit: 1, MemoryRequestGi: 0.5, MemoryLimitGi: 1}
	got := PlanResourceVector(PlanningProfileLocal, SlotCDTool, base)
	assert.Zero(t, got.StorageRequestGi)
	assert.Zero(t, got.StorageLimitGi)
}

func TestPlanningMultipliers_ClampsToProfileCeiling(t *testing.T) {
	// 상한을 넘기는 입력을 준다 — enterprise CI/CD 슬롯의 CPU 상한은 4.5 다.
	options := DefaultPlanningOptions(PlanningProfileEnterprise, SlotCICDPlatform)
	options["concurrentRunners"] = 400 // 최대치

	m := PlanningMultipliers(PlanningProfileEnterprise, SlotCICDPlatform, options)
	assert.LessOrEqual(t, m.CPU, 4.5)
	assert.LessOrEqual(t, m.Memory, 4.5)
	assert.True(t, m.ClampedCPU, "상한에 걸렸으면 표시돼야 한다")
}

func TestPlanningMultipliers_HasFloor(t *testing.T) {
	// 로컬은 0.15, 나머지는 0.5 아래로 내려가지 않는다.
	options := DefaultPlanningOptions(PlanningProfileLocal, SlotMonitoringCollection)
	options["retentionDays"] = 1
	options["metricsTargets"] = 20
	options["scrapeIntervalSec"] = 120

	m := PlanningMultipliers(PlanningProfileLocal, SlotMonitoringCollection, options)
	assert.GreaterOrEqual(t, m.CPU, 0.15)
	assert.GreaterOrEqual(t, m.Memory, 0.15)
	assert.GreaterOrEqual(t, m.Storage, 0.15)

	standardOptions := DefaultPlanningOptions(PlanningProfileStandard, SlotMonitoringCollection)
	standardOptions["retentionDays"] = 1
	sm := PlanningMultipliers(PlanningProfileStandard, SlotMonitoringCollection, standardOptions)
	assert.GreaterOrEqual(t, sm.Storage, 0.5)
}

// 계획 대상 슬롯은 모두 옵션 정의를 가져야 한다. 하나라도 비면 그 슬롯의 도구는
// 프로파일과 무관하게 기준 벡터로 깔린다.
func TestPlanningOptionDefs_CoverEverySlot(t *testing.T) {
	for _, slot := range PlanningSlots() {
		defs := PlanningOptionDefs[slot]
		require.NotEmpty(t, defs, "slot %s 에 옵션 정의가 없다", slot)
		for _, def := range defs {
			assert.Greater(t, def.Baseline, 0.0, "%s.%s baseline", slot, def.Key)
			assert.LessOrEqual(t, def.Min, def.Baseline, "%s.%s min", slot, def.Key)
			assert.GreaterOrEqual(t, def.Max, def.Baseline, "%s.%s max", slot, def.Key)
			assert.False(t, math.IsNaN(def.Weight), "%s.%s weight", slot, def.Key)
		}
	}
}
