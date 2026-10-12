package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ESO 검증 웹훅은 이름에 external-secrets 가 없다.
//
// 실측(2026-09-10, 차트 2.7.0)에서 이름은 externalsecret-validate /
// secretstore-validate 였다. 이름으로 훑는 정리 코드가 놓치는 이유다.
func TestESOWebhookConfigurationNames_MatchChartNames(t *testing.T) {
	require.NotEmpty(t, esoWebhookConfigurationNames)
	assert.ElementsMatch(t,
		[]string{"externalsecret-validate", "secretstore-validate"},
		esoWebhookConfigurationNames)
}

// 웹훅은 ESO 커스텀 리소스보다 **먼저** 지워야 한다.
//
// 웹훅이 살아 있는데 그것을 서빙하던 서비스가 사라지면, 남은 ExternalSecret 을
// 지우려는 요청이 webhook 호출 실패로 거부된다. 네임스페이스는 그 리소스를
// 회수하지 못해 영구 Terminating 이 된다 — 사람이 웹훅을 손으로 지워야만 풀린다.
func TestESOTeardown_DeletesWebhooksBeforeCustomResources(t *testing.T) {
	src := readSourceFile(t, "delete_stack.go")

	webhookAt := indexOfCall(src, "bestEffortDeleteExternalSecretsWebhooks")
	resourcesAt := indexOfCall(src, "bestEffortDeleteExternalSecretResources")

	require.Positive(t, webhookAt, "웹훅 정리 호출이 없다")
	require.Positive(t, resourcesAt)
	assert.Less(t, webhookAt, resourcesAt,
		"웹훅을 나중에 지우면 그 사이에 CR 삭제가 막혀 네임스페이스가 영구 Terminating 이 된다")
}

// ESO CRD 도 Argo CD·Gateway 와 같은 취급을 받아야 한다.
// 남기면 다음 스택 설치가 "invalid ownership metadata" 로 막힌다.
func TestESOTeardown_DeletesCRDs(t *testing.T) {
	src := readSourceFile(t, "delete_stack.go")
	assert.Positive(t, indexOfCall(src, "bestEffortDeleteExternalSecretsCRDs"),
		"ESO CRD 를 남기면 다음 설치가 소유권 충돌로 막힌다")
}
