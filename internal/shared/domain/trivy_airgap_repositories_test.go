package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 에어갭에서 Trivy 가 받는 DB 두 가지의 내부 미러 경로.
//
// 서버(stack 모듈)는 취약점 DB 를, CI 잡의 client(cicd 모듈)는 Java DB 를 받는다.
// 두 모듈이 서로 import 할 수 없어 각자 경로를 조립하면, 반입 스크립트
// (14-push-oci-artifacts.sh 의 compute_target)와 한쪽만 어긋나도 모른다.
func TestTrivyAirgapRepositories(t *testing.T) {
	for _, registry := range []string{"kind-registry:5000/charts", "kind-registry:5000", " kind-registry:5000/charts/ "} {
		assert.Equal(t, "kind-registry:5000", AirgapRegistryHost(registry), registry)
		assert.Equal(t, "kind-registry:5000/aquasecurity/trivy-db", TrivyDBRepository(registry), registry)
		assert.Equal(t, "kind-registry:5000/aquasecurity/trivy-java-db", TrivyJavaDBRepository(registry), registry)
	}
	// 에어갭이 아니면 빈 값이다 — 호출부가 업스트림 기본값을 그대로 쓴다.
	assert.Empty(t, AirgapRegistryHost(""))
	assert.Empty(t, TrivyDBRepository(""))
	assert.Empty(t, TrivyJavaDBRepository("  "))
}
