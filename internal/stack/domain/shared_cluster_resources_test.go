package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 설치 표시를 단 뒤의 cert-manager. 수치는 관리자 기본값으로 바뀌므로 표시만 본다.
func TestIsNullusInstalledRelease_CertManagerWithMarker(t *testing.T) {
	values := map[string]any{
		"global": map[string]any{
			"commonLabels": map[string]any{LabelInstalledBy: InstalledByNullus},
		},
	}

	assert.True(t, IsNullusInstalledRelease(CertManagerReleaseName, values))
}

// 표시를 달기 전 Nullus 가 설치한 cert-manager. kind-nullus-platform 에 실제로 남아 있던 values 그대로다
// — webhook 수치가 DefaultValues(250m)와 다르다. 관리자 기본값이 수치를 바꾸므로 구조로 본다.
func TestIsNullusInstalledRelease_LegacyCertManagerFingerprint(t *testing.T) {
	resources := func() map[string]any {
		return map[string]any{
			"limits":   map[string]any{"cpu": "1", "memory": "1Gi"},
			"requests": map[string]any{"cpu": "500m", "memory": "512Mi"},
		}
	}
	values := map[string]any{
		"installCRDs": true,
		"resources":   resources(),
		"webhook":     map[string]any{"resources": resources()},
		"cainjector":  map[string]any{"resources": resources()},
	}

	assert.True(t, IsNullusInstalledRelease(CertManagerReleaseName, values))
}

// 고객이 흔히 쓰는 설치 모양은 Nullus 것이 아니다.
func TestIsNullusInstalledRelease_CustomerCertManager(t *testing.T) {
	cases := map[string]map[string]any{
		"crds.enabled 만":         {"crds": map[string]any{"enabled": true}},
		"installCRDs 만":          {"installCRDs": true},
		"installCRDs + 컨트롤러 자원만": {"installCRDs": true, "resources": map[string]any{}},
		"values 없음":              {},
		"다른 값의 표시": {
			"installCRDs": true,
			"global":      map[string]any{"commonLabels": map[string]any{LabelInstalledBy: "someone-else"}},
		},
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			assert.False(t, IsNullusInstalledRelease(CertManagerReleaseName, values))
		})
	}
}

func TestIsNullusInstalledRelease_MetricsServerWithMarker(t *testing.T) {
	values := map[string]any{
		"commonLabels": map[string]any{LabelInstalledBy: InstalledByNullus},
	}

	assert.True(t, IsNullusInstalledRelease(MetricsServerReleaseName, values))
}

// Helm 이 돌려주는 values 는 JSON 을 거쳐 문자열 배열이 []any 다.
func TestIsNullusInstalledRelease_LegacyMetricsServerFingerprint(t *testing.T) {
	values := map[string]any{
		"args": []any{
			"--kubelet-insecure-tls",
			"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
		},
		"resources": map[string]any{"requests": map[string]any{"cpu": "250m"}},
	}

	assert.True(t, IsNullusInstalledRelease(MetricsServerReleaseName, values))
}

// kind 사용자는 --kubelet-insecure-tls 하나만 다는 경우가 많다. 그것만으로는 Nullus 것이 아니다.
func TestIsNullusInstalledRelease_CustomerMetricsServer(t *testing.T) {
	values := map[string]any{
		"args":      []any{"--kubelet-insecure-tls"},
		"resources": map[string]any{},
	}

	assert.False(t, IsNullusInstalledRelease(MetricsServerReleaseName, values))
}

func TestIsNullusInstalledRelease_UnknownRelease(t *testing.T) {
	values := map[string]any{"commonLabels": map[string]any{LabelInstalledBy: InstalledByNullus}}

	assert.False(t, IsNullusInstalledRelease("argo-cd", values))
}

func TestInstallMarkerValuePath(t *testing.T) {
	path, ok := InstallMarkerValuePath(CertManagerReleaseName)
	assert.True(t, ok)
	assert.Equal(t, []string{"global", "commonLabels", LabelInstalledBy}, path)

	path, ok = InstallMarkerValuePath(MetricsServerReleaseName)
	assert.True(t, ok)
	assert.Equal(t, []string{"commonLabels", LabelInstalledBy}, path)

	_, ok = InstallMarkerValuePath("gitlab")
	assert.False(t, ok)
}

func TestIsGatewayCRD(t *testing.T) {
	for _, name := range []string{
		"gateways.gateway.networking.k8s.io",
		"backendtlspolicies.gateway.networking.k8s.io",
		"xlistenersets.gateway.networking.x-k8s.io",
		"backends.gateway.envoyproxy.io",
	} {
		assert.True(t, IsGatewayCRD(name), name)
	}
	for _, name := range []string{
		"certificates.cert-manager.io",
		"servicemonitors.monitoring.coreos.com",
		"gateway.networking.k8s.io",
	} {
		assert.False(t, IsGatewayCRD(name), name)
	}
}
