package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치 이미지 스캔은 External Secrets 를 domain 의 이름으로 찾는다. 설치가 다른 곳에 깔면 스캔에서 빠진다.
func TestESONamespace_MatchesDomain(t *testing.T) {
	assert.Equal(t, domain.ESONamespace, ESONamespace)
	spec, ok := defaultChartSpecForStep("installing_cert_manager")
	if ok {
		assert.Equal(t, "cert-manager", spec.Namespace)
	}
	ms, ok := defaultChartSpecForStep("installing_metrics_server")
	assert.True(t, ok)
	assert.Equal(t, "kube-system", ms.Namespace)
}
