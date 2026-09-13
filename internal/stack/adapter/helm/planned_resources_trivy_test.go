package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 스캐너 단계도 자원 기본값과 계획값을 찾아야 한다. 키가 없으면 trivy 시드
// (000079)를 넣어도 한 줄도 실리지 않고, 차트 기본값(메모리 상한 1Gi)으로 깔려
// 취약점 DB 캐시가 밀려난다.
func TestPlannedResources_TrivyStepFindsDefaultAndSlot(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	assert.Equal(t, "trivy", o.resourceDefaultKeyForStep("installing_trivy", &domain.StackConfig{}))
	assert.Equal(t, domain.SlotImageScanner, plannedSlotForStep["installing_trivy"])
}

// Trivy 서버는 파드 하나다(StatefulSet trivy-0). 벡터를 나누지 않고 그대로 싣는다.
func TestResourceValues_TrivyUsesAdminDefault(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "trivy", &domain.ResourceDefault{
		ToolKey: "trivy", CPURequest: 0.25, CPULimit: 1, MemoryRequestGi: 0.5, MemoryLimitGi: 2,
	})

	values := o.resourceDefaultValuesForStep("installing_trivy", &domain.StackConfig{})
	requests := requestsFromValues(t, values)
	assert.Equal(t, "250m", requests["cpu"])
	assert.Equal(t, "512Mi", requests["memory"])
	limits := values["resources"].(map[string]any)["limits"].(map[string]any)
	assert.Equal(t, "2Gi", limits["memory"])
}

func TestResourceValues_TrivyUsesPlannedValues(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "trivy", &domain.ResourceDefault{
		ToolKey: "trivy", CPURequest: 0.25, CPULimit: 1, MemoryRequestGi: 0.5, MemoryLimitGi: 2,
	})

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			"security.imageScanner:trivy": {
				CPURequest: 1, CPULimit: 2, MemoryRequestGi: 1, MemoryLimitGi: 3,
			},
		},
	}

	requests := requestsFromValues(t, o.resourceDefaultValuesForStep("installing_trivy", &cfg))
	assert.Equal(t, "1", requests["cpu"])
	assert.Equal(t, "1Gi", requests["memory"])
}
