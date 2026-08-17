package domain

import (
	"math"
	"regexp"
	"strings"
)

// 설치 규모 계획.
//
// "무엇을 깔지"(템플릿의 도구 목록)와 "얼마나 크게 깔지"는 다른 결정이다. 같은
// Gitea + Jenkins + Argo CD 라도 enterprise 로 계획하면 8Gi 노드에 들어가지
// 않는다. 이 파일은 후자를 계산한다 — 프로파일과 슬롯을 받아 관리자 기본값
// 벡터에 곱할 배수를 내놓는다.
//
// 계산은 원래 설치 마법사(web/src/features/stack/utils/install-planning-utils.ts)
// 에만 있었다. 그래서 API 로 설치하면 프로파일이 아무 일도 하지 않았고, 템플릿이
// planning_profile 을 들고 있어도 헤드리스 설치는 언제나 standard 크기로 깔렸다.
// 여기로 옮겨 UI 설치와 API 설치가 같은 크기를 내게 한다.
//
// 프론트의 값이 기준이다 — 숫자가 갈리면 두 경로가 다른 크기로 깔린다.
// (고정: TestPlanResourceVector_MatchesInstallWizard)

// 계획 슬롯. 도구 이름이 아니라 자리로 잇는다 — 이름은 gitlab / gitlab-ce 처럼
// 출처마다 흔들리지만 자리는 흔들리지 않는다.
const (
	SlotPackageRegistry         = "artifacts.packageRegistry"
	SlotSourceRepository        = "artifacts.sourceRepository"
	SlotContainerRegistry       = "artifacts.containerRegistry"
	SlotStorageBackend          = "artifacts.storageBackend"
	SlotCICDPlatform            = "pipeline.cicdPlatform"
	SlotCDTool                  = "pipeline.cdTool"
	SlotMonitoringCollection    = "monitoring.collection"
	SlotMonitoringVisualization = "monitoring.visualization"
	SlotLogSearch               = "logging.search"
	SlotTraceLayer              = "logging.traceLayer"
	SlotTraceExporter           = "logging.traceExporter"
)

// PlanningSlots 는 계획 대상 슬롯을 마법사 화면과 같은 순서로 돌려준다.
func PlanningSlots() []string {
	return []string{
		SlotPackageRegistry,
		SlotSourceRepository,
		SlotContainerRegistry,
		SlotStorageBackend,
		SlotCICDPlatform,
		SlotCDTool,
		SlotMonitoringCollection,
		SlotMonitoringVisualization,
		SlotLogSearch,
		SlotTraceLayer,
		SlotTraceExporter,
	}
}

// ResourceImpact 는 옵션 하나가 각 자원에 미치는 영향도다. 음수면 값이 커질수록
// 부하가 준다 (스크랩 주기처럼).
type ResourceImpact struct {
	CPU     float64
	Memory  float64
	Storage float64
}

// PlanningOptionDef 는 계획 행 하나가 묻는 질문이다 (예: "동시 러너 수").
type PlanningOptionDef struct {
	Key      string
	Baseline float64
	Min      float64
	Max      float64
	Weight   float64
	Impact   ResourceImpact
}

// ResourceMultipliers 는 기준 벡터에 곱할 배수다. Clamped* 는 상·하한에 걸려
// 잘렸는지를 알린다 — 잘린 값을 그대로 보여주면 계획이 반영된 것처럼 읽힌다.
type ResourceMultipliers struct {
	CPU     float64
	Memory  float64
	Storage float64

	RawCPU     float64
	RawMemory  float64
	RawStorage float64

	ClampedCPU     bool
	ClampedMemory  bool
	ClampedStorage bool
}

// PlanningOptionDefs 는 슬롯별 옵션 정의다.
var PlanningOptionDefs = map[string][]PlanningOptionDef{
	SlotPackageRegistry: {
		{Key: "registryCallsPerDay", Baseline: 3000, Min: 500, Max: 50000, Weight: 0.45, Impact: ResourceImpact{CPU: 1, Memory: 0.8, Storage: 0.3}},
		{Key: "avgArtifactSizeMb", Baseline: 120, Min: 10, Max: 2000, Weight: 0.25, Impact: ResourceImpact{CPU: 0.1, Memory: 0.2, Storage: 1}},
		{Key: "retentionDays", Baseline: 30, Min: 1, Max: 365, Weight: 0.30, Impact: ResourceImpact{CPU: 0, Memory: 0.1, Storage: 1}},
	},
	SlotSourceRepository: {
		{Key: "activeRepoUsers", Baseline: 20, Min: 5, Max: 500, Weight: 0.4, Impact: ResourceImpact{CPU: 0.8, Memory: 0.6, Storage: 0.3}},
		{Key: "repoCount", Baseline: 60, Min: 5, Max: 3000, Weight: 0.35, Impact: ResourceImpact{CPU: 0.2, Memory: 0.4, Storage: 1}},
		{Key: "dailyPushEvents", Baseline: 250, Min: 20, Max: 20000, Weight: 0.25, Impact: ResourceImpact{CPU: 1, Memory: 0.5, Storage: 0.4}},
	},
	SlotContainerRegistry: {
		{Key: "imagePullsPerDay", Baseline: 2000, Min: 200, Max: 100000, Weight: 0.4, Impact: ResourceImpact{CPU: 0.8, Memory: 0.7, Storage: 0.4}},
		{Key: "newImagePushesPerDay", Baseline: 180, Min: 10, Max: 8000, Weight: 0.35, Impact: ResourceImpact{CPU: 0.9, Memory: 0.6, Storage: 0.8}},
		{Key: "avgImageSizeGb", Baseline: 1.2, Min: 0.1, Max: 20, Weight: 0.25, Impact: ResourceImpact{CPU: 0.1, Memory: 0.2, Storage: 1}},
	},
	SlotStorageBackend: {
		{Key: "objectOpsPerDay", Baseline: 10000, Min: 1000, Max: 200000, Weight: 0.45, Impact: ResourceImpact{CPU: 0.9, Memory: 0.8, Storage: 0.4}},
		{Key: "storedDataTb", Baseline: 1.5, Min: 0.1, Max: 100, Weight: 0.35, Impact: ResourceImpact{CPU: 0.1, Memory: 0.2, Storage: 1}},
		{Key: "backupFrequencyPerWeek", Baseline: 7, Min: 1, Max: 30, Weight: 0.20, Impact: ResourceImpact{CPU: 0.4, Memory: 0.3, Storage: 0.7}},
	},
	SlotCICDPlatform: {
		{Key: "developers", Baseline: 20, Min: 1, Max: 1000, Weight: 0.2, Impact: ResourceImpact{CPU: 0.4, Memory: 0.4, Storage: 0.2}},
		{Key: "concurrentRunners", Baseline: 4, Min: 1, Max: 400, Weight: 0.55, Impact: ResourceImpact{CPU: 1.8, Memory: 1.6, Storage: 0.7}},
		{Key: "dailyCommits", Baseline: 120, Min: 10, Max: 10000, Weight: 0.25, Impact: ResourceImpact{CPU: 0.8, Memory: 0.6, Storage: 0.3}},
	},
	SlotCDTool: {
		{Key: "deploymentsPerDay", Baseline: 40, Min: 1, Max: 2000, Weight: 0.5, Impact: ResourceImpact{CPU: 0.8, Memory: 0.6, Storage: 0.2}},
		{Key: "environmentsCount", Baseline: 4, Min: 1, Max: 30, Weight: 0.25, Impact: ResourceImpact{CPU: 0.4, Memory: 0.5, Storage: 0.3}},
		{Key: "rollbackRatePercent", Baseline: 8, Min: 0, Max: 80, Weight: 0.25, Impact: ResourceImpact{CPU: 0.5, Memory: 0.6, Storage: 0.2}},
	},
	SlotMonitoringCollection: {
		{Key: "metricsTargets", Baseline: 150, Min: 20, Max: 5000, Weight: 0.45, Impact: ResourceImpact{CPU: 0.7, Memory: 0.9, Storage: 0.4}},
		{Key: "scrapeIntervalSec", Baseline: 30, Min: 5, Max: 120, Weight: 0.30, Impact: ResourceImpact{CPU: -0.6, Memory: -0.7, Storage: -0.2}},
		{Key: "retentionDays", Baseline: 15, Min: 1, Max: 365, Weight: 0.25, Impact: ResourceImpact{CPU: 0, Memory: 0.2, Storage: 1}},
	},
	SlotMonitoringVisualization: {
		{Key: "dashboardUsers", Baseline: 30, Min: 5, Max: 2000, Weight: 0.45, Impact: ResourceImpact{CPU: 0.5, Memory: 0.5, Storage: 0.1}},
		{Key: "dashboardCount", Baseline: 40, Min: 5, Max: 1500, Weight: 0.30, Impact: ResourceImpact{CPU: 0.4, Memory: 0.6, Storage: 0.2}},
		{Key: "refreshIntervalSec", Baseline: 30, Min: 5, Max: 300, Weight: 0.25, Impact: ResourceImpact{CPU: -0.5, Memory: -0.4, Storage: -0.1}},
	},
	SlotLogSearch: {
		{Key: "logGbPerDay", Baseline: 100, Min: 5, Max: 10000, Weight: 0.5, Impact: ResourceImpact{CPU: 0.6, Memory: 0.7, Storage: 1}},
		{Key: "retentionDays", Baseline: 30, Min: 1, Max: 365, Weight: 0.3, Impact: ResourceImpact{CPU: 0, Memory: 0.2, Storage: 1}},
		{Key: "queryUsers", Baseline: 20, Min: 1, Max: 1000, Weight: 0.2, Impact: ResourceImpact{CPU: 0.7, Memory: 0.6, Storage: 0.2}},
	},
	SlotTraceLayer: {
		{Key: "traceSpansPerMin", Baseline: 50000, Min: 1000, Max: 3000000, Weight: 0.5, Impact: ResourceImpact{CPU: 0.8, Memory: 0.7, Storage: 0.5}},
		{Key: "serviceCount", Baseline: 40, Min: 5, Max: 2000, Weight: 0.3, Impact: ResourceImpact{CPU: 0.4, Memory: 0.5, Storage: 0.3}},
		{Key: "traceRetentionDays", Baseline: 7, Min: 1, Max: 90, Weight: 0.2, Impact: ResourceImpact{CPU: 0, Memory: 0.2, Storage: 1}},
	},
	SlotTraceExporter: {
		{Key: "traceSpansPerMin", Baseline: 50000, Min: 1000, Max: 3000000, Weight: 0.6, Impact: ResourceImpact{CPU: 0.9, Memory: 0.7, Storage: 0.2}},
		{Key: "serviceCount", Baseline: 40, Min: 5, Max: 2000, Weight: 0.4, Impact: ResourceImpact{CPU: 0.5, Memory: 0.4, Storage: 0.1}},
	},
}

// 처리량 성격의 옵션 어휘. 프로파일이 작아지면 이 값들이 함께 줄어든다.
// developers 처럼 여기 걸리지 않는 키는 "기타" 계수를 받는다.
var throughputOptionPattern = regexp.MustCompile(`(?i)(calls|events|pulls|pushes|ops|deployments|commits|targets|spans|query|users|count)`)

// ProfileFactorByOption 은 프로파일이 옵션 기본값을 몇 배로 조정하는지 돌려준다.
//
// 성격별로 방향이 다르다 — 보관 기간·처리량은 작은 프로파일에서 줄지만, 주기
// (interval)는 늘어난다. 주기가 길수록 부하가 줄기 때문이다.
func ProfileFactorByOption(profile, optionKey string) float64 {
	profile = NormalizePlanningProfile(profile)
	if profile == PlanningProfileStandard {
		return 1
	}

	lower := strings.ToLower(optionKey)
	isRetention := strings.Contains(lower, "retention")
	isInterval := strings.Contains(lower, "interval")
	isConcurrency := optionKey == "concurrentRunners"
	isThroughput := throughputOptionPattern.MatchString(optionKey)

	switch profile {
	case PlanningProfileLocal:
		switch {
		case isRetention:
			return 0.2
		case isInterval:
			return 2.5
		case isConcurrency:
			return 0.25
		case isThroughput:
			return 0.25
		default:
			return 0.3
		}
	case PlanningProfileStartup:
		switch {
		case isRetention:
			return 0.45
		case isInterval:
			return 1.7
		case isConcurrency:
			return 0.35
		case isThroughput:
			return 0.45
		default:
			return 0.55
		}
	}

	// enterprise
	switch {
	case isRetention:
		return 1.8
	case isInterval:
		return 0.7
	case isConcurrency:
		return 1.8
	case isThroughput:
		return 1.7
	default:
		return 1.45
	}
}

// ProfileAdjustedBaseline 은 프로파일이 적용된 옵션 기본값이다. 마법사가 화면을
// 열 때 채워 넣는 값과 같다.
func ProfileAdjustedBaseline(profile string, def PlanningOptionDef) float64 {
	value := ceil2(def.Baseline * ProfileFactorByOption(profile, def.Key))
	return math.Min(def.Max, math.Max(def.Min, value))
}

// DefaultPlanningOptions 는 사용자가 아무것도 만지지 않은 상태의 옵션값이다.
// 헤드리스 설치는 언제나 이 상태로 계획한다.
func DefaultPlanningOptions(profile, slot string) map[string]float64 {
	defs := PlanningOptionDefs[slot]
	if len(defs) == 0 {
		return nil
	}
	values := make(map[string]float64, len(defs))
	for _, def := range defs {
		values[def.Key] = ProfileAdjustedBaseline(profile, def)
	}
	return values
}

var profileDamping = map[string]float64{
	PlanningProfileLocal:      0.2,
	PlanningProfileStartup:    0.35,
	PlanningProfileStandard:   0.7,
	PlanningProfileEnterprise: 0.9,
}

// 프로파일 자체가 기준 벡터를 줄이거나 늘리는 배수. 옵션 조정과 별개로 걸린다.
var profileResourceScale = map[string]float64{
	PlanningProfileLocal:      0.25,
	PlanningProfileStartup:    0.8,
	PlanningProfileStandard:   1,
	PlanningProfileEnterprise: 1.15,
}

type clampCeiling struct{ CPU, Memory, Storage float64 }

// CI/CD 슬롯만 상한이 따로다. 러너는 동시성에 따라 실제로 더 크게 튀는 자리라
// 다른 슬롯과 같은 천장을 씌우면 계획이 늘 잘린다.
var profileClampMax = map[string]struct{ CICD, Default clampCeiling }{
	PlanningProfileLocal: {
		CICD:    clampCeiling{CPU: 1.35, Memory: 1.35, Storage: 1.25},
		Default: clampCeiling{CPU: 1.25, Memory: 1.25, Storage: 1.25},
	},
	PlanningProfileStartup: {
		CICD:    clampCeiling{CPU: 1.9, Memory: 1.9, Storage: 1.6},
		Default: clampCeiling{CPU: 1.6, Memory: 1.6, Storage: 1.6},
	},
	PlanningProfileStandard: {
		CICD:    clampCeiling{CPU: 3.2, Memory: 3.2, Storage: 2.4},
		Default: clampCeiling{CPU: 2.4, Memory: 2.4, Storage: 2.2},
	},
	PlanningProfileEnterprise: {
		CICD:    clampCeiling{CPU: 4.5, Memory: 4.5, Storage: 3.2},
		Default: clampCeiling{CPU: 3.2, Memory: 3.2, Storage: 2.8},
	},
}

// 러너 동시성은 선형이 아니다 — 러너를 2배로 늘려도 컨트롤 플레인이 2배로
// 커지지는 않으므로 지수를 1 미만으로 두고 상한을 씌운다.
var runnerCPUExponent = map[string]float64{
	PlanningProfileLocal: 0.15, PlanningProfileStartup: 0.2,
	PlanningProfileStandard: 0.35, PlanningProfileEnterprise: 0.45,
}

var runnerMemoryExponent = map[string]float64{
	PlanningProfileLocal: 0.12, PlanningProfileStartup: 0.18,
	PlanningProfileStandard: 0.3, PlanningProfileEnterprise: 0.4,
}

var runnerCPUBoostCap = map[string]float64{
	PlanningProfileLocal: 1.08, PlanningProfileStartup: 1.15,
	PlanningProfileStandard: 1.45, PlanningProfileEnterprise: 1.75,
}

var runnerMemoryBoostCap = map[string]float64{
	PlanningProfileLocal: 1.06, PlanningProfileStartup: 1.12,
	PlanningProfileStandard: 1.4, PlanningProfileEnterprise: 1.65,
}

// PlanningMultipliers 는 프로파일과 옵션값으로 기준 벡터에 곱할 배수를 낸다.
func PlanningMultipliers(profile, slot string, optionValues map[string]float64) ResourceMultipliers {
	profile = NormalizePlanningProfile(profile)
	defs := PlanningOptionDefs[slot]
	if len(defs) == 0 {
		// 계획 대상이 아닌 슬롯. 1 을 돌려 기준 벡터를 그대로 통과시킨다.
		return ResourceMultipliers{CPU: 1, Memory: 1, Storage: 1, RawCPU: 1, RawMemory: 1, RawStorage: 1}
	}

	damping := profileDamping[profile]
	var weightedCPU, weightedMemory, weightedStorage float64
	for _, def := range defs {
		value := def.Baseline
		if v, ok := optionValues[def.Key]; ok {
			value = v
		}
		delta := (value - def.Baseline) / def.Baseline
		weightedCPU += delta * def.Weight * def.Impact.CPU * damping
		weightedMemory += delta * def.Weight * def.Impact.Memory * damping
		weightedStorage += delta * def.Weight * def.Impact.Storage * damping
	}

	rawCPU := 1 + weightedCPU
	rawMemory := 1 + weightedMemory
	rawStorage := 1 + weightedStorage

	if slot == SlotCICDPlatform {
		for _, def := range defs {
			if def.Key != "concurrentRunners" {
				continue
			}
			runners := def.Baseline
			if v, ok := optionValues["concurrentRunners"]; ok {
				runners = v
			}
			ratio := math.Max(0.25, runners/def.Baseline)
			rawCPU *= math.Min(runnerCPUBoostCap[profile], math.Pow(ratio, runnerCPUExponent[profile]))
			rawMemory *= math.Min(runnerMemoryBoostCap[profile], math.Pow(ratio, runnerMemoryExponent[profile]))
			break
		}
	}

	scale := profileResourceScale[profile]
	rawCPU *= scale
	rawMemory *= scale
	rawStorage *= scale

	ceiling := profileClampMax[profile].Default
	if slot == SlotCICDPlatform {
		ceiling = profileClampMax[profile].CICD
	}

	// 로컬 kind 클러스터는 호스팅 환경보다 훨씬 작은 requests 를 요구한다.
	// 0.5 하한만 두면 GitLab 과 모니터링이 스케줄되지 않는다.
	floor := 0.5
	if profile == PlanningProfileLocal {
		floor = 0.15
	}
	clamp := func(value, max float64) float64 {
		return math.Min(max, math.Max(floor, value))
	}

	cpu := clamp(rawCPU, ceiling.CPU)
	memory := clamp(rawMemory, ceiling.Memory)
	storage := clamp(rawStorage, ceiling.Storage)

	return ResourceMultipliers{
		CPU: cpu, Memory: memory, Storage: storage,
		RawCPU: rawCPU, RawMemory: rawMemory, RawStorage: rawStorage,
		ClampedCPU:     cpu != rawCPU,
		ClampedMemory:  memory != rawMemory,
		ClampedStorage: storage != rawStorage,
	}
}

// ApplyPlanningMultipliers 는 기준 벡터에 배수를 곱해 0.5 단위로 정리한다.
func ApplyPlanningMultipliers(base ResourceVector, m ResourceMultipliers) ResourceVector {
	return ResourceVector{
		CPURequest:       roundRecommendedHalf(base.CPURequest * m.CPU),
		CPULimit:         roundRecommendedHalf(base.CPULimit * m.CPU),
		MemoryRequestGi:  roundRecommendedHalf(base.MemoryRequestGi * m.Memory),
		MemoryLimitGi:    roundRecommendedHalf(base.MemoryLimitGi * m.Memory),
		StorageRequestGi: roundRecommendedHalf(base.StorageRequestGi * m.Storage),
		StorageLimitGi:   roundRecommendedHalf(base.StorageLimitGi * m.Storage),
	}
}

// PlanResourceVector 는 프로파일 기본 옵션으로 계획한 최종 벡터다. 헤드리스
// 설치가 쓰는 진입점.
func PlanResourceVector(profile, slot string, base ResourceVector) ResourceVector {
	options := DefaultPlanningOptions(profile, slot)
	return ApplyPlanningMultipliers(base, PlanningMultipliers(profile, slot, options))
}

// 추천값은 0.5 단위로 끊는다. 0 인 축은 0 으로 남긴다 — 스토리지를 쓰지 않는
// 도구에 0.5Gi 를 채우면 PVC 가 없는 차트에 값을 실어 보내게 된다.
func roundRecommendedHalf(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Max(0.5, math.Round(value*2)/2)
}

func ceil2(value float64) float64 {
	return math.Ceil(value*100) / 100
}
