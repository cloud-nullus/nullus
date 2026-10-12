package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var trivySel = ToolSelection{Name: "Trivy", Version: "0.69.3", Enabled: true}

// 의존성 스캔은 선택이다. 외부 선택은 스택 안에 서버를 세우지 않는다.
func TestHasSCA(t *testing.T) {
	assert.True(t, HasSCA(trivySel))
	assert.False(t, HasSCA(ToolSelection{}))
	assert.False(t, HasSCA(ToolSelection{Name: "Trivy", Enabled: false}))
	assert.False(t, HasSCA(ToolSelection{Name: "Trivy", Version: "external", Enabled: true}))
}

// 의존성 스캔만 골라도 Trivy 서버가 선다 — 파이프라인의 trivy fs 가 서버에 client 로 붙는다.
// 둘 다 고르면 서버는 하나다.
func TestHasTrivyServer(t *testing.T) {
	var none StackConfig
	assert.False(t, HasTrivyServer(none), "아무것도 고르지 않으면 서지 않는다")

	var scaOnly StackConfig
	scaOnly.Security.SCA = trivySel
	assert.True(t, HasTrivyServer(scaOnly))

	var both StackConfig
	both.Security.ImageScanner = trivySel
	both.Security.SCA = trivySel
	assert.True(t, HasTrivyServer(both))
}

func TestTrivyServerSelection(t *testing.T) {
	var scaOnly StackConfig
	scaOnly.Security.SCA = ToolSelection{Version: "0.69.3", Enabled: true}
	assert.Equal(t, "Trivy", TrivyServerSelection(scaOnly).Name, "이름이 비어도 계획이 도구를 찾을 수 있어야 한다")

	var both StackConfig
	both.Security.ImageScanner = ToolSelection{Name: "Trivy", Version: "image", Enabled: true}
	both.Security.SCA = ToolSelection{Name: "Trivy", Version: "sca", Enabled: true}
	assert.Equal(t, "image", TrivyServerSelection(both).Version, "이미지 스캐너가 있으면 그것이 서버를 대표한다")

	assert.False(t, TrivyServerSelection(StackConfig{}).Enabled)
}

// 의존성 스캔만 고른 스택도 서버 크기 계획을 받는다 — 설치 단계가 이미지 스캐너 슬롯으로 찾는다.
func TestPlanAppliedResources_PlansTrivyServerForSCAOnly(t *testing.T) {
	var cfg StackConfig
	cfg.Security.SCA = trivySel
	got := PlanAppliedResources(PlanningProfileStandard, cfg,
		map[string]ResourceVector{"trivy": {CPURequest: 1, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4}})
	assert.Contains(t, got, "security.imageScanner:trivy")
}

// 의존성 스캔만 고른 스택도 Trivy 서버 파드가 모니터링 목록에 뜬다. 둘 다 골라도 한 번만 뜬다.
func TestInstalledToolWorkloads_TrivyServerForSCA(t *testing.T) {
	count := func(cfg StackConfig) int {
		n := 0
		for _, w := range InstalledToolWorkloads(cfg) {
			if w.Key == "image_scanner" {
				n++
			}
		}
		return n
	}
	var scaOnly StackConfig
	scaOnly.Security.SCA = trivySel
	assert.Equal(t, 1, count(scaOnly))

	var both StackConfig
	both.Security.ImageScanner = trivySel
	both.Security.SCA = trivySel
	assert.Equal(t, 1, count(both))

	assert.Equal(t, 0, count(StackConfig{}))
}
