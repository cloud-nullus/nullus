// 차트로 AI 어시스턴트를 켤 수 있는지 본다(nullus-plan#94). 코드가 환경 변수를 읽어도 차트가 넣지
// 않으면 배포 환경에서는 켤 방법이 없다 — 백업 때 실제로 그랬다(chart_backup_test.go).
package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChart_어시스턴트는_키가_없으면_꺼진다(t *testing.T) {
	out := render(t)
	assert.NotContains(t, out, "NULLUS_ASSISTANT_API_KEY")
	assert.NotContains(t, out, "assistant-api-key")
}

func TestChart_어시스턴트_키는_Secret_으로_넣는다(t *testing.T) {
	out := render(t,
		"--set", "secrets.assistantApiKey=up_secret_key",
		"--set", "config.assistant.model=solar-mini",
		"--set", "config.assistant.baseURL=http://vllm.internal:8000/v1",
	)
	assert.Contains(t, out, "assistant-api-key:")
	assert.NotContains(t, out, "up_secret_key", "키 원문은 렌더 결과(ConfigMap · env value)에 없다 — Secret 에 base64 로만")
	assert.Contains(t, out, "name: NULLUS_ASSISTANT_API_KEY")
	assert.Contains(t, out, "key: assistant-api-key")
	assert.Contains(t, out, "name: NULLUS_ASSISTANT_MODEL")
	assert.Contains(t, out, `value: "solar-mini"`)
	assert.Contains(t, out, `value: "http://vllm.internal:8000/v1"`)
}
