package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// CRD 만 담은 릴리스는 파드를 만들지 않는다. 준비 검사가 파드를 전제해서
// 설치가 이렇게 죽었다:
//
//	runtime readiness failed for prometheus-operator-crds:
//	no pods found for release prometheus-operator-crds
//
// 목록으로 명시한다. "파드가 없으면 통과" 로 일반화하면, 정작 파드를 만들어야
// 하는 릴리스가 아무것도 못 만들었을 때 그것까지 조용히 통과시킨다.
func TestWorkloadlessRelease(t *testing.T) {
	assert.True(t, isWorkloadlessRelease("prometheus-operator-crds"),
		"CRD 차트는 기다릴 워크로드가 없다")
	assert.True(t, isWorkloadlessRelease("  prometheus-operator-crds "))

	for _, r := range []string{"cert-manager", "nullus-minio", "gitlab", "argo-cd"} {
		assert.Falsef(t, isWorkloadlessRelease(r),
			"%s 는 파드를 만든다 — 준비 검사를 건너뛰면 안 된다", r)
	}
}
