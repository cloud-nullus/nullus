package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// kubectl 호출을 그대로 받아 적는다. 실제 클러스터 없이 "무엇을 지우려 했는가"를
// 검사하려면 인자 그대로가 필요하다.
type kubectlRecorder struct {
	calls  []string
	stubs  map[string]string
	failed map[string]bool
}

func newKubectlRecorder() *kubectlRecorder {
	return &kubectlRecorder{stubs: map[string]string{}, failed: map[string]bool{}}
}

func (r *kubectlRecorder) run(_ context.Context, _ []byte, args ...string) (string, error) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	return r.stubs[call], nil
}

func (r *kubectlRecorder) has(substrings ...string) bool {
	for _, call := range r.calls {
		matched := true
		for _, s := range substrings {
			if !strings.Contains(call, s) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func (r *kubectlRecorder) indexOf(substrings ...string) int {
	for i, call := range r.calls {
		matched := true
		for _, s := range substrings {
			if !strings.Contains(call, s) {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

func newGatewayDeleteStack(t *testing.T, rec *kubectlRecorder, stack *domain.Stack) *DeleteStack {
	t.Helper()
	uc := NewDeleteStack(
		newFakeStackRepo(stack),
		&fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")},
		func([]byte) port.HelmInstaller { return &fakeHelmInstaller{} },
		&captureStreamer{},
	)
	uc.runKubectlFunc = rec.run
	return uc
}

// 스택을 지워도 Gateway 커스텀 리소스가 남으면, Envoy Gateway 컨트롤러가 그것을
// 보고 데이터플레인 Deployment 를 곧바로 다시 만든다. 그래서 envoy 파드를 지우는
// 단계가 있어도 파드는 계속 떠 있고, helm list 는 깨끗하게 나온다(실측 확인:
// 삭제 2시간 뒤에도 envoy-<stack>-gateway 파드가 2/2 Running).
//
// Gateway 를 먼저 지워야 관리 리소스 삭제가 의미를 갖는다.
func TestDeleteStack_DeletesGatewayAPIResources(t *testing.T) {
	rec := newKubectlRecorder()
	rec.stubs["get gateways.gateway.networking.k8s.io -n nullus-lite -o name"] =
		"gateway.gateway.networking.k8s.io/nullus-lite-gateway\n"

	uc := newGatewayDeleteStack(t, rec, &domain.Stack{
		ID:        "stk-gw",
		Name:      "nullus-lite",
		ClusterID: "cluster-gw",
		Namespace: "nullus-lite",
		State:     domain.StateCompleted,
	})

	require.NoError(t, uc.Execute(context.Background(), "stk-gw"))

	assert.True(t, rec.has("delete", "gateways.gateway.networking.k8s.io", "-n nullus-lite"),
		"Gateway 를 지우지 않으면 envoy 데이터플레인이 다시 살아난다.\n호출: %v", rec.calls)
	assert.True(t, rec.has("delete", "httproutes.gateway.networking.k8s.io", "-n nullus-lite"),
		"HTTPRoute 도 함께 지워야 한다.\n호출: %v", rec.calls)
}

// 순서가 핵심이다. Gateway 를 나중에 지우면 그 사이 컨트롤러가 데이터플레인을
// 복구해, 앞서 지운 Deployment 가 되살아난 채로 삭제가 끝난다.
func TestDeleteStack_DeletesGatewayBeforeItsManagedResources(t *testing.T) {
	rec := newKubectlRecorder()
	rec.stubs["get gateways.gateway.networking.k8s.io -n nullus-lite -o name"] =
		"gateway.gateway.networking.k8s.io/nullus-lite-gateway\n"

	uc := newGatewayDeleteStack(t, rec, &domain.Stack{
		ID:        "stk-gw-order",
		Name:      "nullus-lite",
		ClusterID: "cluster-gw",
		Namespace: "nullus-lite",
		State:     domain.StateCompleted,
	})

	require.NoError(t, uc.Execute(context.Background(), "stk-gw-order"))

	gatewayDelete := rec.indexOf("delete", "gateways.gateway.networking.k8s.io", "-n nullus-lite")
	managedDelete := rec.indexOf("delete deploy", "owning-gateway-name=nullus-lite-gateway")
	require.GreaterOrEqual(t, gatewayDelete, 0, "Gateway 삭제 호출이 없다")
	require.GreaterOrEqual(t, managedDelete, 0, "관리 리소스 삭제 호출이 없다")
	assert.Less(t, gatewayDelete, managedDelete,
		"Gateway 를 먼저 지워야 컨트롤러가 데이터플레인을 복구하지 않는다")
}

// 다른 스택의 Gateway 는 건드리지 않는다. 네임스페이스를 좁히지 않으면 한
// 클러스터에 스택이 둘일 때 남의 게이트웨이를 지운다.
func TestDeleteStack_DoesNotDeleteGatewaysOutsideStackNamespaces(t *testing.T) {
	rec := newKubectlRecorder()

	uc := newGatewayDeleteStack(t, rec, &domain.Stack{
		ID:        "stk-gw-scope",
		Name:      "nullus-lite",
		ClusterID: "cluster-gw",
		Namespace: "nullus-lite",
		State:     domain.StateCompleted,
	})

	require.NoError(t, uc.Execute(context.Background(), "stk-gw-scope"))

	for _, call := range rec.calls {
		// CRD 삭제(delete crd ...)는 다른 이야기다 — 클러스터 스코프이고, 어디에도
		// Gateway 가 남지 않았을 때만 도는 별도 가드가 이미 있다.
		if strings.HasPrefix(call, "delete crd ") {
			continue
		}
		if strings.HasPrefix(call, "delete") && strings.Contains(call, "gateways.gateway.networking.k8s.io") {
			assert.Contains(t, call, "-n nullus-lite",
				"Gateway 인스턴스 삭제는 스택 네임스페이스로 한정해야 한다: %s", call)
			assert.NotContains(t, call, " -A",
				"클러스터 전체 Gateway 삭제는 다른 스택을 깬다: %s", call)
		}
	}
}
