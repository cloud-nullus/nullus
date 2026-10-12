package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// DAG 와 domain.InstallStepOrder 는 같은 단계 집합이어야 한다.
//
// 둘은 쓰임이 다르다 — DAG 는 무엇을 언제 실행할지, InstallStepOrder 는
// 오케스트레이터의 ensureOrder 가 순서를 확인할 근거다. 한쪽에만 단계를 넣으면
// 설치가 그 단계까지 정상 진행하다 "step order not defined" 로 죽는다.
//
// 실제로 installing_trivy 를 DAG 에만 넣었을 때 그렇게 실패했다. 단위 테스트가
// 전부 통과한 채로 실물 설치에서만 드러났다.
func TestInstallDAG_AndInstallStepOrder_CoverSameSteps(t *testing.T) {
	inOrder := map[string]bool{}
	for _, s := range domain.InstallStepOrder {
		inOrder[s] = true
	}

	for _, name := range dagStepNames() {
		assert.Truef(t, inOrder[name],
			"DAG 의 %q 가 domain.InstallStepOrder 에 없다 — 설치가 그 단계에서 "+
				"'step order not defined' 로 죽는다", name)
	}

	inDAG := map[string]bool{}
	for _, name := range dagStepNames() {
		inDAG[name] = true
	}
	for _, s := range domain.InstallStepOrder {
		assert.Truef(t, inDAG[s],
			"InstallStepOrder 의 %q 를 DAG 가 실행하지 않는다 — 아무도 돌리지 않는 "+
				"단계를 순서에 세워 두면 진행률만 어긋난다", s)
	}
}
