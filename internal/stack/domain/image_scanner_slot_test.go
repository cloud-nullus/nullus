package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 이미지 스캐너는 스택 구성에서 고르는 선택 항목이다.
//
// 슬롯이 없으면 템플릿 편집기가 "Security / Scanner / Trivy" 를 안내해도 실제로는
// 아무것도 설치되지 않는다 — 화면은 선언하는데 설치는 없는, 000070 이 되돌린
// 것과 같은 간극이다.
func TestPlanningSlots_IncludesImageScanner(t *testing.T) {
	assert.Contains(t, PlanningSlots(), SlotImageScanner,
		"이미지 스캐너 슬롯이 계획 대상에 없으면 크기 계획이 서지 않는다")
}

// 슬롯을 늘렸으면 계획 맵도 그 자리를 만들어야 한다. plannedSelections 에
// 빠지면 사용자가 골라도 applied_resource_overrides 에 한 줄도 남지 않는다.
func TestPlanAppliedResources_PlansImageScanner(t *testing.T) {
	cfg := StackConfig{
		Security: SecurityConfig{
			ImageScanner: ToolSelection{Name: "Trivy", Enabled: true},
		},
	}
	base := map[string]ResourceVector{
		"trivy": {CPURequest: 1, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4},
	}

	got := PlanAppliedResources(PlanningProfileStandard, cfg, base)

	// 키는 "<슬롯>:<도구키>" — 설치 단계가 슬롯으로 계획을 찾으므로 형태가
	// 어긋나면 계산은 맞아도 한 줄도 적용되지 않는다.
	assert.Contains(t, got, "security.imageScanner:trivy")
}

// 고르지 않은 자리는 계획하지 않는다. 스캐너는 선택이므로 이 경로가 기본값이다.
func TestPlanAppliedResources_SkipsDisabledImageScanner(t *testing.T) {
	cfg := StackConfig{
		Security: SecurityConfig{
			ImageScanner: ToolSelection{Name: "Trivy", Enabled: false},
		},
	}

	got := PlanAppliedResources(PlanningProfileStandard, cfg,
		map[string]ResourceVector{"trivy": {CPURequest: 1}})

	assert.Empty(t, got, "고르지 않은 스캐너의 자원을 예약하면 설치하지도 않은 도구가 자리를 차지한다")
}

// InstalledToolWorkloads 에 없으면 파드가 멀쩡히 떠 있어도 어느 화면에도 뜨지 않는다.
func TestInstalledToolWorkloads_IncludesImageScanner(t *testing.T) {
	cfg := StackConfig{
		Security: SecurityConfig{
			ImageScanner: ToolSelection{Name: "Trivy", Version: "0.58.0", Enabled: true},
		},
	}

	got := InstalledToolWorkloads(cfg)

	require.Len(t, got, 1)
	assert.Equal(t, "image_scanner", got[0].Key)
	assert.Equal(t, "Trivy", got[0].Name)
	assert.Equal(t, "0.58.0", got[0].Version)
	assert.Equal(t, []string{TrivyReleaseName}, got[0].NamePrefixes,
		"릴리스명이 곧 파드 이름 접두사다 — 어긋나면 '0 파드 warning' 으로 남는다")
}

func TestInstalledToolWorkloads_OmitsDisabledImageScanner(t *testing.T) {
	cfg := StackConfig{
		Security: SecurityConfig{
			ImageScanner: ToolSelection{Name: "Trivy", Enabled: false},
		},
	}

	assert.Empty(t, InstalledToolWorkloads(cfg))
}

// 이름을 비워 보내는 경로가 있다(마법사가 자리만 켜는 경우).
// 다른 슬롯과 같은 규칙으로 기본 표기를 채운다.
func TestInstalledToolWorkloads_ImageScannerFallsBackToCanonicalName(t *testing.T) {
	cfg := StackConfig{
		Security: SecurityConfig{
			ImageScanner: ToolSelection{Enabled: true},
		},
	}

	got := InstalledToolWorkloads(cfg)

	require.Len(t, got, 1)
	assert.Equal(t, "trivy", got[0].Name)
}

// 설정은 stacks.config JSONB 로 오간다. 받는 칸이 없으면 마법사가 보낸 선택이
// 조용히 버려진다 — TraceExporter 가 실제로 그랬다.
func TestDecodeStackConfig_ReadsImageScanner(t *testing.T) {
	raw := map[string]any{
		"security": map[string]any{
			"image_scanner": map[string]any{
				"name":    "Trivy",
				"version": "0.58.0",
				"enabled": true,
			},
		},
	}

	got := DecodeStackConfig(raw)

	assert.Equal(t,
		ToolSelection{Name: "Trivy", Version: "0.58.0", Enabled: true},
		got.Security.ImageScanner)
}

// 슬롯마다 "얼마나 크게 깔지" 를 묻는 질문이 있어야 한다
// (TestPlanningOptionDefs_CoverEverySlot 가 이미 강제한다).
//
// 스캐너의 부하는 매칭 요청 수에 비례하고 저장은 DB 크기라 거의 고정이므로,
// 다른 슬롯과 달리 보관 기간을 묻지 않는다.
func TestPlanningOptionDefs_ImageScannerAsksScanLoad(t *testing.T) {
	defs := PlanningOptionDefs[SlotImageScanner]
	require.NotEmpty(t, defs)

	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.Key)
	}
	assert.ElementsMatch(t, []string{"scansPerDay", "concurrentScans"}, keys)

	for _, def := range defs {
		assert.LessOrEqual(t, def.Impact.Storage, 0.2,
			"%s: 취약점 DB 크기는 거의 고정이라 저장 영향이 크면 안 된다", def.Key)
	}
}

// 스캔 부하는 처리량이다 — 프로파일이 작아지면 함께 줄어야 한다.
// 어휘 패턴에 걸리지 않으면 "기타" 계수를 받아 local 에서 덜 줄어든다.
func TestProfileFactorByOption_TreatsScanLoadAsThroughput(t *testing.T) {
	for _, key := range []string{"scansPerDay", "concurrentScans"} {
		t.Run(key, func(t *testing.T) {
			assert.Equal(t, 0.25, ProfileFactorByOption(PlanningProfileLocal, key))
			assert.Equal(t, 0.45, ProfileFactorByOption(PlanningProfileStartup, key))
			assert.Equal(t, 1.7, ProfileFactorByOption(PlanningProfileEnterprise, key))
		})
	}
}
