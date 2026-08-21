package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 게이트웨이 이름은 스택이 정한다. 여기서는 그 규칙을 따라갈 뿐이다.
//
// 스택 설치는 게이트웨이를 `<스택 이름>-gateway` 로 만든다
// (web/src/features/stack/utils/install-manifest-builders.ts 의 buildGatewayManifest).
// 그런데 이쪽은 **접근 도메인**에서 이름을 만들고 있었다 — nullus.io 에서
// "nullus-io-gateway" 가 나왔고, 그런 게이트웨이는 존재하지 않는다.
//
// 없는 게이트웨이를 가리키는 HTTPRoute 는 어느 컨트롤러도 집지 않는다. 그래서
// status.parents 가 아예 비고, Accepted=False 조차 남지 않는다 — 라우트는
// 만들어졌는데 주소만 열리지 않고, 어디에도 오류가 없다.
func TestGatewayNameForStack_MatchesWhatTheStackCreates(t *testing.T) {
	assert.Equal(t, "nullus-devsecops-stack-gateway",
		gatewayNameForStack("nullus-devsecops-stack"))
}

// 이름을 모르면 지어내지 않는다. 빈 값이면 스캐폴딩이 HTTPRoute 자체를 만들지
// 않으므로(renderHTTPRoute), 없는 게이트웨이를 가리키는 라우트가 생기지 않는다.
func TestGatewayNameForStack_EmptyWhenStackNameUnknown(t *testing.T) {
	assert.Equal(t, "", gatewayNameForStack(""))
	assert.Equal(t, "", gatewayNameForStack("   "))
}
