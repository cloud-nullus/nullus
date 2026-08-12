package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 클러스터에 하나만 존재하는 애드온은 스택 네임스페이스에 깔면 안 된다.
//
// metrics-server 가 그랬다. ChartSpec 에 Namespace 가 없어 스택 네임스페이스로
// 들어갔는데, 이 차트가 만드는 APIService v1beta1.metrics.k8s.io 는 cluster-scoped
// 다. 스택을 지우면 Service 만 사라지고 APIService 는 죽은 대상을 계속 가리켜
// API discovery 가 실패하고, 그러면 **클러스터의 모든 네임스페이스 삭제가 교착**된다.
//
// 실제로 nullus-harbor 를 지운 뒤 무관한 nullus-gitlab / nullus-nexus /
// nullus-github-e2e 까지 이틀 넘게 Terminating 에 갇혀 있었다. 진단 메시지는
// "metrics.k8s.io/v1beta1: stale GroupVersion discovery" 였다.
//
// 같은 뿌리로 ClusterRole system:metrics-server 등에 release-namespace 가 스택
// 네임스페이스로 박혀, 두 번째 스택은 설치가 반드시 실패한다(OpenBao 와 같은 패턴).
//
// cert-manager 는 이미 Namespace 를 못박아 이 문제를 피하고 있었다 — 같은 규칙을
// 적용한다.
func TestClusterSingletonChartsPinTheirNamespace(t *testing.T) {
	// isSharedClusterScopedStep 이 "클러스터에 하나뿐" 이라고 보는 단계들이다.
	for _, step := range []string{stepInstallingCertManager, "installing_metrics_server"} {
		t.Run(step, func(t *testing.T) {
			require.True(t, isSharedClusterScopedStep(step), "이 테스트의 전제")

			spec, ok := defaultChartSpecForStep(step)
			require.True(t, ok)

			assert.NotEmpty(t, spec.Namespace,
				"Namespace 가 비면 스택 네임스페이스로 설치된다 — 스택을 지울 때 클러스터가 망가진다")
		})
	}
}

func TestMetricsServerInstallsIntoKubeSystem(t *testing.T) {
	spec, ok := defaultChartSpecForStep("installing_metrics_server")
	require.True(t, ok)

	assert.Equal(t, "kube-system", spec.Namespace)
}
