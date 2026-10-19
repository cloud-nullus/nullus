package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 스택은 클러스터 공용 구성요소(cert-manager · metrics-server · External Secrets)를 쓴다. 스택 네임스페이스
// 밖에 있어 스캔에서 빠졌다 — 그 이미지의 CVE 도 스택의 위험이다.
func TestSharedComponents(t *testing.T) {
	assert.Equal(t, []SharedComponent{
		{Release: CertManagerReleaseName, Namespace: "cert-manager"},
		{Release: MetricsServerReleaseName, Namespace: "kube-system"},
		{Release: ESOReleaseName, Namespace: ESONamespace},
	}, SharedComponents())
}
