package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 스캐너 주소 조립 규칙을 shared 가 갖는 이유는 OTLP 와 같다 — stack 은 이
// 이름으로 차트를 설치하고 cicd 는 CI 잡이 붙을 주소를 넣어 준다. 한쪽이
// 문자열을 흉내 내면 차트가 바뀔 때 조용히 어긋난다.
func TestTrivyServerEndpoint(t *testing.T) {
	assert.Equal(t, "http://trivy.scan-demo.svc.cluster.local:4954",
		TrivyServerEndpoint("scan-demo"))
}

// 네임스페이스가 비면 주소를 만들지 않는다. 빈 자리를 채운 주소를 주면
// CI 잡이 엉뚱한 곳에 붙어 스캔이 전부 실패한다.
func TestTrivyServerEndpoint_EmptyNamespace(t *testing.T) {
	assert.Empty(t, TrivyServerEndpoint(""))
	assert.Empty(t, TrivyServerEndpoint("   "))
}
