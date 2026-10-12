package helm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 복구 뒤 재설정은 설치가 만든 values 를 그 자리에서 고친다. 고친 결과가 새 주소로 설치했을 때와
// 같아야 한다 — 설치의 TOML 형식이 바뀌어 재설정 규칙과 어긋나면 여기서 걸린다.
func TestRepinRoundTrip_MatchesFreshInstallValues(t *testing.T) {
	const domainName = "nullus.local"
	hosts := domain.GatewayPinnedHostnames(domainName)
	assert.Equal(t, runnerCIHostnames(domainName), hosts, "설치와 재설정이 같은 이름을 본다")

	for _, trustCA := range []bool{false, true} {
		runner := gitLabRunnerHostAliasValues("10.96.45.47", domainName, "helper:arm64", trustCA)
		changed, old := domain.RepinRunnerHostAliases(runner, hosts, "10.96.200.9")
		require.True(t, changed)
		assert.Equal(t, []string{"10.96.45.47"}, old)
		assert.Equal(t, gitLabRunnerHostAliasValues("10.96.200.9", domainName, "helper:arm64", trustCA), runner)
	}

	argo := argoCDGatewayHostAliasValues("10.96.45.47", domainName)
	// helm 이 돌려주는 values 는 JSON 을 거친 꼴이다 — 문자열 목록이 []any 로 온다.
	argo["global"].(map[string]any)["hostAliases"].([]any)[0].(map[string]any)["hostnames"] = []any{hosts[0], hosts[1]}
	changed, _ := domain.RepinArgoCDHostAliases(argo, hosts, "10.96.200.9")
	require.True(t, changed)
	assert.Equal(t, "10.96.200.9", argo["global"].(map[string]any)["hostAliases"].([]any)[0].(map[string]any)["ip"])
}

func gatewayService(ns, ip string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "envoy-gw", Namespace: ns,
			Labels: map[string]string{gatewayServiceSelector: "nullus-local-gate"}},
		Spec: corev1.ServiceSpec{ClusterIP: ip},
	}
}

func TestGatewayAddressReader_ReadsGatewayClusterIP(t *testing.T) {
	c := fake.NewSimpleClientset(
		gatewayService("stack-a", "10.96.200.9"),
		gatewayService("stack-b", "10.96.1.1"),
	)
	r := newGatewayAddressReader(c, time.Millisecond, 50*time.Millisecond)

	ip, err := r.GatewayClusterIP(context.Background(), "stack-a")
	require.NoError(t, err)
	assert.Equal(t, "10.96.200.9", ip)
}

// 복구 직후에는 게이트웨이 Service 가 아직 없을 수 있다(컨트롤러가 다시 만든다). 기다리다 없으면 알린다.
func TestGatewayAddressReader_TimesOutWhenMissing(t *testing.T) {
	r := newGatewayAddressReader(fake.NewSimpleClientset(), time.Millisecond, 20*time.Millisecond)

	_, err := r.GatewayClusterIP(context.Background(), "stack-a")
	assert.Error(t, err)
}
