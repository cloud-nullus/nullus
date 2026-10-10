package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SAST(SonarQube)는 이미지 스캐너와 같은 security 계열의 선택 슬롯이다.
//
// 슬롯이 없으면 템플릿 편집기의 "예: Trivy, SonarQube" 처럼 화면은 선언하는데
// 설치는 없는 간극이 그대로 남는다(000070 이 되돌린 것과 같다).
func TestPlanningSlots_IncludesSAST(t *testing.T) {
	assert.Contains(t, PlanningSlots(), SlotSAST)
}

func TestPlanAppliedResources_PlansSAST(t *testing.T) {
	cfg := StackConfig{Security: SecurityConfig{SAST: ToolSelection{Name: "SonarQube", Enabled: true}}}
	base := map[string]ResourceVector{
		"sonarqube": {CPURequest: 0.5, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4},
	}

	got := PlanAppliedResources(PlanningProfileStandard, cfg, base)

	assert.Contains(t, got, "security.sast:sonarqube")
}

func TestPlanAppliedResources_SkipsDisabledSAST(t *testing.T) {
	cfg := StackConfig{Security: SecurityConfig{SAST: ToolSelection{Name: "SonarQube", Enabled: false}}}

	got := PlanAppliedResources(PlanningProfileStandard, cfg,
		map[string]ResourceVector{"sonarqube": {CPURequest: 1}})

	assert.Empty(t, got, "고르지 않은 SonarQube 의 자원을 예약하면 설치하지도 않은 도구가 자리를 차지한다")
}

// 외부(external) 선택은 스택 안에 서지 않는다. 이미지 스캐너와 같은 규칙이다.
func TestHasSAST(t *testing.T) {
	assert.True(t, HasSAST(ToolSelection{Name: "SonarQube", Enabled: true}))
	assert.False(t, HasSAST(ToolSelection{Name: "SonarQube", Enabled: false}))
	assert.False(t, HasSAST(ToolSelection{Name: "SonarQube", Version: "external", Enabled: true}))
}

// InstalledToolWorkloads 에 없으면 파드가 떠 있어도 어느 화면에도 뜨지 않는다.
func TestInstalledToolWorkloads_IncludesSAST(t *testing.T) {
	cfg := StackConfig{Security: SecurityConfig{SAST: ToolSelection{Name: "SonarQube", Version: SonarQubeAppVersion, Enabled: true}}}

	var found *ToolWorkload
	for _, w := range InstalledToolWorkloads(cfg) {
		if w.Key == "sast" {
			w := w
			found = &w
		}
	}

	require.NotNil(t, found, "SonarQube 가 설치 도구 목록에 없다")
	assert.Equal(t, "SonarQube", found.Name)
	assert.Contains(t, found.NamePrefixes, SonarQubeReleaseName)
}

func TestInstalledToolWorkloads_SkipsDisabledSAST(t *testing.T) {
	for _, w := range InstalledToolWorkloads(StackConfig{}) {
		assert.NotEqual(t, "sast", w.Key)
	}
}

// SonarQube 는 화면이 있는 도구다. 주소 규칙이 한 곳에 있어야 스택 상세·연결정보가
// 같은 주소를 안내한다.
func TestToolAccessURL_SonarQube(t *testing.T) {
	assert.Equal(t, "https://sonarqube.nullus.local", ToolAccessURL("SonarQube", "nullus.local"))
}

// 마법사가 만든 게이트웨이 매니페스트를 서버가 이 표로 바로잡는다. 빠지면 라우트가
// 없는 서비스를 가리켜 설치는 성공했는데 주소만 열리지 않는다.
func TestGatewayBackendForTool_SonarQube(t *testing.T) {
	backend, ok := GatewayBackendForTool("SonarQube")

	require.True(t, ok)
	assert.Equal(t, GatewayBackend{Service: SonarQubeServiceName, Port: SonarQubeServicePort}, backend)
}

// 설치 단계는 공유 PostgreSQL 과 SSO 클라이언트가 선 뒤에 온다. 관리자 비밀번호를
// 바꾸는 프로비저닝은 설치 바로 다음이다.
func TestInstallStepOrder_PlacesSonarQubeAfterSSOAndPostgres(t *testing.T) {
	index := func(step string) int {
		for i, s := range InstallStepOrder {
			if s == step {
				return i
			}
		}
		return -1
	}

	install := index("installing_sonarqube")
	provision := index("provisioning_sonarqube")
	require.GreaterOrEqual(t, install, 0, "installing_sonarqube 가 설치 순서에 없다")
	require.GreaterOrEqual(t, provision, 0, "provisioning_sonarqube 가 설치 순서에 없다")
	assert.Greater(t, install, index("installing_postgresql"))
	assert.Greater(t, install, index("provisioning_sso"))
	assert.Equal(t, install+1, provision)
}

// 삭제는 무엇이 설치됐는지 모른 채 릴리스 목록을 훑는다. 빠지면 스택을 지워도
// SonarQube 파드와 볼륨이 남는다.
func TestAllHelmReleaseNames_IncludesSonarQube(t *testing.T) {
	assert.Contains(t, AllHelmReleaseNames(), SonarQubeReleaseName)
}
