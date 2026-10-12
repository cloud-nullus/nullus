package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ESO 는 클러스터 범위 리소스를 CRD 말고도 만든다.
//
// 하드코딩한 CRD 2개만 인수하던 시절, 스택을 지우고 다른 네임스페이스에 새로
// 만들면 Helm 이 나머지에서 "invalid ownership metadata" 로 죽었다. 실측(2026-09-10,
// ESO 차트 2.7.0)에서 남아 있던 것은 CRD 24개 · ClusterRole 5개 ·
// ClusterRoleBinding 2개 · ValidatingWebhookConfiguration 2개였다.
//
// 종류를 손으로 적는 대신 Helm 이 소유권을 판정하는 바로 그 애노테이션
// (meta.helm.sh/release-name)으로 찾는다. 차트가 리소스를 늘려도 따라간다.
func TestESOClusterScopedKinds_CoverWhatTheChartCreates(t *testing.T) {
	assert.ElementsMatch(t, []string{
		"crd",
		"clusterrole",
		"clusterrolebinding",
		"validatingwebhookconfiguration",
		"mutatingwebhookconfiguration",
	}, esoClusterScopedKinds,
		"차트가 만드는 클러스터 범위 종류가 빠지면 그 종류에서 설치가 막힌다")
}

// 웹훅 설정은 이름에 external-secrets 가 없다 — externalsecret-validate,
// secretstore-validate 다. 이름으로 찾으면 놓치고, 놓치면 네임스페이스 삭제가
// 영구 Terminating 으로 막힌다.
func TestESOClusterScopedKinds_IncludeWebhookConfigurations(t *testing.T) {
	assert.Contains(t, esoClusterScopedKinds, "validatingwebhookconfiguration")
}
