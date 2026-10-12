package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 의존성 스캔(SCA)만 골라도 Trivy 서버가 선다 — 파이프라인의 trivy fs 가 이 서버에 client 로 붙는다.
// 서버가 없으면 의존성 스캔 단계가 없는 서버에 붙어 모든 배포가 막힌다(nullus-plan#79).
func TestTrivyStep_AlsoWhenSCASelected(t *testing.T) {
	tests := []struct {
		name  string
		image domain.ToolSelection
		sca   domain.ToolSelection
		want  bool
	}{
		{"의존성 스캔만", domain.ToolSelection{}, domain.ToolSelection{Enabled: true, Name: "Trivy"}, true},
		{"둘 다 — 서버는 하나", domain.ToolSelection{Enabled: true, Name: "Trivy"}, domain.ToolSelection{Enabled: true, Name: "Trivy"}, true},
		{"둘 다 고르지 않음", domain.ToolSelection{}, domain.ToolSelection{}, false},
		{"외부 의존성 스캔은 설치하지 않는다", domain.ToolSelection{}, domain.ToolSelection{Enabled: true, Name: "Trivy", Version: "external"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cfg domain.StackConfig
			cfg.Security.ImageScanner = tc.image
			cfg.Security.SCA = tc.sca
			o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
			o.SetStackConfig(cfg)
			assert.Equal(t, tc.want, o.IsStepEnabled("installing_trivy"))
		})
	}
}
