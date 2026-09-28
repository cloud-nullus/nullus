package helm

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 게이트웨이 데이터 플레인 Service 는 Gateway 를 적용해야 생긴다. 컨트롤러 차트만
// 깔린 시점에는 없으므로, 그 주소를 읽는 재적용은 매니페스트 적용 뒤여야 한다.
//
// 앞에 두면 조회가 언제나 빈 값을 받아 경고만 남기고 넘어간다 — 설치는 completed
// 로 끝나고 파드도 전부 Running 이라, CI 잡을 돌려 "Could not resolve host" 를
// 볼 때까지 아무도 모른다. 실제로 그렇게 새어 나갔다(2026-09-25 kind 실측).
//
// 호출 순서를 코드에서 직접 본다. 두 동작 다 클러스터를 건드리므로 단위 테스트로는
// 가려낼 수 없고, 이 순서가 이 수정의 전부다.
func TestExecuteStep_GatewayHostAliasesReconcileRunsAfterManifestApply(t *testing.T) {
	src, err := os.ReadFile("orchestrator.go")
	require.NoError(t, err)
	text := string(src)

	manifestApply := strings.Index(text, `if step == "installing_gateway" && hasManifest {`)
	require.Positive(t, manifestApply, "게이트웨이 매니페스트 적용 블록을 찾지 못했다")

	reconcile := strings.Index(text, "o.reconcileGatewayHostAliases(ctx, stackID, namespace, phase)")
	require.Positive(t, reconcile, "재적용 호출을 찾지 못했다")

	require.Greater(t, reconcile, manifestApply,
		"재적용은 Gateway 매니페스트를 적용한 뒤에 불려야 한다 — 앞에 두면 데이터 플레인 Service 가 아직 없다")
}
