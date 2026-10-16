package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Prometheus Operator CRD 릴리스는 ServiceMonitor·Probe·PrometheusRule 을 담은 릴리스(argo-cd·nullus-minio·
// kube-prometheus-stack·grafana 등)보다 늦게 지워야 한다. 먼저 지우면 helm uninstall 이 그 릴리스의
// 매니페스트를 객체로 만들지 못해("no matches for kind ServiceMonitor") 리소스를 하나도 지우지 않고
// 멈춘다 — 네임스페이스 회수가 안쪽은 치워 주지만 kube-system 의 Service·ClusterRole 은 남아 다음
// 설치를 Helm 소유권 충돌로 막는다(2026-10-12 실측).
func TestHelmReleaseNamesInUninstallOrder_PrometheusCRDsLast(t *testing.T) {
	order := HelmReleaseNamesInUninstallOrder()

	assert.Equal(t, PrometheusOperatorCRDsReleaseName, order[len(order)-1])
	assert.ElementsMatch(t, AllHelmReleaseNames(), order, "순서만 바꾸고 빠뜨리거나 겹치면 안 된다")
}

func TestHelmReleaseNamesInUninstallOrder_KeepsOtherReleasesInPlace(t *testing.T) {
	var want []string
	for _, name := range AllHelmReleaseNames() {
		if name != PrometheusOperatorCRDsReleaseName {
			want = append(want, name)
		}
	}

	order := HelmReleaseNamesInUninstallOrder()

	assert.Equal(t, want, order[:len(order)-1])
}
