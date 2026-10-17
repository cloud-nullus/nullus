package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ESO 검증 웹훅은 닿지 않을 때 통과시켜야 한다 — 막으면 네임스페이스가 지워지지 않는다.
//
// 차트 2.7.0 의 externalsecret-validate / secretstore-validate 는
// operations: ["CREATE","UPDATE","DELETE"] 에 failurePolicy: Fail 로 붙는다.
// DELETE 까지 웹훅을 호출하므로, 웹훅 파드가 먼저 죽으면 ExternalSecret 삭제가
// 거부되고 그 네임스페이스는 Terminating 에서 끝나지 않는다.
//
//	NamespaceDeletionContentFailure: failed calling webhook
//	  "validate.externalsecret.external-secrets.io": Post
//	  "https://external-secrets-webhook.<삭제 중인 NS>.svc:443/..."
//
// ESO 는 스택 네임스페이스에 깔리므로 웹훅 Service 가 가리키는 곳이 바로
// 지우려는 네임스페이스다 — 순환 의존이다.
//
// Ignore 로 잃는 것은 거의 없다. Nullus 의 ExternalSecret·SecretStore 는 사람이
// 쓰지 않고 제품이 템플릿으로 생성하며(secret-provisioning.go), 설치 경로는
// 웹훅 rollout 을 기다린 뒤에 apply 한다(InstallExternalSecrets). 웹훅이 내려간
// 동안 잘못된 스펙이 들어오더라도 조용히 통과하는 것이 아니라 ESO 가
// status.conditions 에 실패를 적고 Secret 을 만들지 않는다.
//
// 차트에 conversion 웹훅은 없으므로(2.7.0 실측) 이 한 값으로 삭제 의존이 사라진다.
func TestESOWebhookFailsOpen(t *testing.T) {
	values := externalSecretsValues()

	webhook, ok := values["webhook"].(map[string]any)
	require.True(t, ok, "webhook 값 블록이 있어야 한다")

	assert.Equal(t, "Ignore", webhook["failurePolicy"],
		"Fail 이면 웹훅이 죽은 뒤 ExternalSecret 삭제가 거부돼 네임스페이스가 교착된다")
}
