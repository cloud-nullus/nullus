package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// cert-manager 가 이미 깔려 있는 클러스터에서 이 단계가 매번 2분씩 걸렸다.
//
// startupapicheck 은 Helm 훅 Job 이라 설치 직후 사라진다. 재설치 때는 영원히
// 없는데, waitForKubectlGet 이 60회 × 2초를 꼬박 돈 뒤에야 "없으니 건너뛴다" 고
// 판단했다. 설치할 때마다 그 시간을 통째로 버린 것이다.
//
// 여기서는 "생길 때까지 기다린다" 가 아니라 "지금 있는가" 를 물어야 한다.
// waitForKubectlGet 자체는 그대로 둔다 — CRD 처럼 곧 만들어질 리소스를 기다리는
// 경로가 여럿이고, 거기서 "없음" 을 즉시 실패로 보면 설치가 깨진다.

func TestCertManagerStartupCheck_UsesExistenceNotWait(t *testing.T) {
	src := readSourceFile(t, "cert-manager.go")

	fn := functionBody(t, src, "func (o *Orchestrator) waitForCertManagerStartupAPICheck")
	assert.NotContains(t, fn, "waitForKubectlGet",
		"없는 Job 을 기다리면 설치마다 재시도 시간을 통째로 버린다")
	assert.Contains(t, fn, "kubectlResourceExists",
		"존재 여부는 한 번만 물어야 한다")
}
