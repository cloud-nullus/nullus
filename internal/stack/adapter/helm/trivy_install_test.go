package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestDefaultChartSpecForStep_Trivy(t *testing.T) {
	spec, ok := DefaultChartSpecForStep("installing_trivy")

	require.True(t, ok, "installing_trivy 에 차트 스펙이 없으면 스텝이 unknown step 으로 죽는다")
	assert.Equal(t, domain.TrivyReleaseName, spec.ReleaseName)
	assert.Equal(t, "trivy", spec.ChartName)
	assert.Equal(t, "https://aquasecurity.github.io/helm-charts/", spec.RepoURL)
	assert.Equal(t, domain.TrivyChartVersion, spec.Version)
}

// 스캐너는 선택이다. 고르지 않은 스택에 서면 아무도 고르지 않은 워크로드가 뜬다.
func TestTrivyStep_OnlyWhenImageScannerSelected(t *testing.T) {
	tests := []struct {
		name string
		sel  domain.ToolSelection
		want bool
	}{
		{"고르면 선다", domain.ToolSelection{Enabled: true, Name: "Trivy"}, true},
		{"고르지 않으면 서지 않는다", domain.ToolSelection{}, false},
		{"꺼져 있으면 서지 않는다", domain.ToolSelection{Enabled: false, Name: "Trivy"}, false},
		{"외부 스캐너는 설치하지 않는다", domain.ToolSelection{Enabled: true, Name: "Trivy", Version: "external"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := domain.StackConfig{}
			cfg.Security.ImageScanner = tc.sel

			o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
			o.SetStackConfig(cfg)

			assert.Equal(t, tc.want, o.IsStepEnabled("installing_trivy"))
		})
	}
}

// 계획한 크기가 설치에 실리려면 스텝과 슬롯이 이어져 있어야 한다.
// 끊기면 계산은 맞는데 한 줄도 적용되지 않는다.
func TestTrivyStep_MapsToImageScannerSlot(t *testing.T) {
	assert.Equal(t, domain.SlotImageScanner, plannedSlotForStep["installing_trivy"])
}

// server 모드로 서야 한다. standalone 이면 CI 잡이 DB 를 직접 받아야 하고,
// 그러면 파이프라인마다 DB 를 감당하는 구조로 되돌아간다.
func TestTrivyDefaultValues_RunsAsServerWithMirroredDB(t *testing.T) {
	values := DefaultValues("installing_trivy")
	require.NotNil(t, values)

	trivy, ok := values["trivy"].(map[string]any)
	require.True(t, ok, "trivy 설정 블록이 없으면 DB 경로를 지정할 수 없다")

	// 내부 미러를 가리켜야 에어갭에서 DB 를 받을 수 있다.
	assert.NotEmpty(t, trivy["dbRepository"])

	// skipDBUpdate 를 켜면 미러를 갱신해도 서버가 새 DB 를 쓰지 않는다.
	assert.Equal(t, false, trivy["skipDBUpdate"],
		"미러에서 정상 갱신 경로를 타야 DB 를 갱신했을 때 저절로 반영된다")

	// CI 잡이 붙을 주소는 domain.TrivyServicePort 로 조립된다. 차트 기본값이
	// 바뀌면 잡은 옛 포트로 붙어 스캔이 전부 실패한다.
	service, ok := values["service"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, domain.TrivyServicePort, service["port"])
}
