package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// selectiveHelmInstaller 는 고른 릴리스만 지워진 것으로 답한다 — 나머지는 "release: not found" 다.
type selectiveHelmInstaller struct {
	fakeHelmInstaller
	present map[string]bool
}

func (f *selectiveHelmInstaller) Uninstall(ctx context.Context, releaseName, namespace string) error {
	_ = f.fakeHelmInstaller.Uninstall(ctx, releaseName, namespace)
	if f.present[releaseName+"@"+namespace] {
		return nil
	}
	return errors.New("uninstall: Release not loaded: " + releaseName + ": release: not found")
}

func newUninstallOrderDeleteStack(t *testing.T, rec *kubectlRecorder, installer port.HelmInstaller) *DeleteStack {
	t.Helper()
	uc := NewDeleteStack(
		newFakeStackRepo(&domain.Stack{
			ID:        "stk-order",
			Name:      "nullus-gitlab-sca",
			ClusterID: "cluster-order",
			Namespace: "nullus-gitlab-sca",
			State:     domain.StateCompleted,
		}),
		&fakeDeleteKubeconfigProvider{config: []byte("apiVersion: v1\nclusters:\n- name: kind\n")},
		func([]byte) port.HelmInstaller { return installer },
		&captureStreamer{},
	)
	uc.runKubectlFunc = rec.run
	return uc
}

func uninstallCallIndex(calls []string, call string) int {
	for i, c := range calls {
		if c == call {
			return i
		}
	}
	return -1
}

// 2026-10-12 실측: 스택을 지우자 prometheus-operator-crds 가 두 번째로 지워졌고, 그 뒤 argo-cd·nullus-minio·
// kube-prometheus-stack·grafana 의 uninstall 이 모두 "failed to delete release" 로 멈췄다. kube-system 의
// kube-prometheus-stack-coredns 등 Service 5개와 grafana ClusterRole 이 남아 다음 스택 설치가
// installing_prometheus 에서 소유권 충돌로 실패했다.
func TestDeleteStack_UninstallsPrometheusCRDsAfterMonitoringConsumers(t *testing.T) {
	installer := &fakeHelmInstaller{}
	uc := newUninstallOrderDeleteStack(t, newKubectlRecorder(), installer)

	require.NoError(t, uc.Execute(context.Background(), "stk-order"))

	crds := uninstallCallIndex(installer.uninstallCalls, domain.PrometheusOperatorCRDsReleaseName+"@nullus-gitlab-sca")
	require.NotEqual(t, -1, crds, "호출: %v", installer.uninstallCalls)
	for _, consumer := range []string{"argo-cd", "nullus-minio", "kube-prometheus-stack", "grafana", "loki", domain.TrivyReleaseName} {
		i := uninstallCallIndex(installer.uninstallCalls, consumer+"@nullus-gitlab-sca")
		require.NotEqual(t, -1, i, "%s 를 지우지 않았다", consumer)
		assert.Less(t, i, crds, "%s 는 Prometheus CRD 가 살아 있을 때 지워야 한다", consumer)
	}
}

// Prometheus Operator 는 kubelet 을 긁기 위해 kube-system 에 <릴리스>-kubelet Service 를 스스로 만든다.
// 릴리스가 소유하지 않아 helm uninstall 로는 사라지지 않는다 — 스택의 kube-prometheus-stack 을 지웠으면
// 그것도 지운다.
func TestDeleteStack_DeletesOperatorKubeletServiceAfterPrometheusUninstall(t *testing.T) {
	rec := newKubectlRecorder()
	installer := &selectiveHelmInstaller{present: map[string]bool{"kube-prometheus-stack@nullus-gitlab-sca": true}}
	uc := newUninstallOrderDeleteStack(t, rec, installer)

	require.NoError(t, uc.Execute(context.Background(), "stk-order"))

	assert.True(t, rec.has("delete", "-n kube-system", "service/kube-prometheus-stack-kubelet",
		"endpoints/kube-prometheus-stack-kubelet", "--ignore-not-found"), "호출: %v", rec.calls)
	assert.True(t, rec.has("delete", "endpointslices.discovery.k8s.io", "-n kube-system",
		"kubernetes.io/service-name=kube-prometheus-stack-kubelet", "--ignore-not-found"), "호출: %v", rec.calls)
}

// 이 스택이 kube-prometheus-stack 을 지우지 않았으면(깔지 않았거나 다른 자리의 것) 그 Service 는 이 스택
// 것이 아닐 수 있다 — 건드리지 않는다.
func TestDeleteStack_KeepsKubeletServiceWhenPrometheusWasNotUninstalled(t *testing.T) {
	rec := newKubectlRecorder()
	installer := &selectiveHelmInstaller{present: map[string]bool{}}
	uc := newUninstallOrderDeleteStack(t, rec, installer)

	require.NoError(t, uc.Execute(context.Background(), "stk-order"))

	for _, call := range rec.calls {
		assert.False(t, strings.Contains(call, "kube-prometheus-stack-kubelet"), "건드리면 안 된다: %s", call)
	}
}
