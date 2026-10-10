package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// stack 은 이 이름으로 SonarQube 를 설치하고, cicd 는 CI 잡이 붙을 주소를 넣어 준다.
// 규칙을 한 곳에 둔다 — 한쪽이 문자열을 흉내 내면 차트가 바뀔 때 조용히 어긋난다.
func TestSonarQubeServerEndpoint(t *testing.T) {
	assert.Equal(t, "http://sonarqube.sast-demo.svc.cluster.local:9000", SonarQubeServerEndpoint("sast-demo"))
	assert.Empty(t, SonarQubeServerEndpoint(" "), "네임스페이스가 비면 주소를 만들지 않는다")
}

// 분석 토큰의 OpenBao 경로는 스택 설치(쓰는 쪽)와 cicd(읽는 쪽)가 함께 쓴다.
func TestSonarQubeAnalysisTokenPath(t *testing.T) {
	assert.Equal(t, "security/sonarqube/analysis-token", SonarQubeAnalysisTokenPath)
}
