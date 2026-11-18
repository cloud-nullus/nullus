package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// Nexus 차트(64.2.0)는 지원이 끊겨 appVersion 이 3.64.0 에 멈춰 있다. values 가 태그를
// 지정하지 않으면 차트 기본값인 3.64.0 — 지원이 끝나고 비인증 경로 탐색(CVE-2024-4956)이
// 남은 버전 — 이 뜬다. 설치하는 버전은 domain.NexusAppVersion 이 정한다.
func TestNexusValues_PinsImageToDomainVersion(t *testing.T) {
	values := DefaultValues("installing_nexus")
	require.NotNil(t, values)

	image, ok := values["image"].(map[string]any)
	require.True(t, ok, "image 를 지정하지 않으면 차트 기본 태그(3.64.0)가 뜬다")
	assert.Equal(t, "sonatype/nexus3", image["repository"])
	assert.Equal(t, domain.NexusAppVersion, image["tag"])
}

func nexusRelease(valuesTag, chartAppVersion string) *release.Release {
	cfg := map[string]any{}
	if valuesTag != "" {
		cfg["image"] = map[string]any{"repository": "sonatype/nexus3", "tag": valuesTag}
	}
	return &release.Release{
		Name:   domain.NexusReleaseName,
		Config: cfg,
		Chart:  &chart.Chart{Metadata: &chart.Metadata{AppVersion: chartAppVersion}},
	}
}

// 3.71.0 부터 OrientDB 를 열지 못한다. 3.64.0 으로 떠 있던 릴리스를 재시도 · 도구 추가가
// 다시 설치하면서 새 태그로 올리면 파드가 데이터베이스를 열지 못해 서지 않는다.
func TestKeepNexusDataFormat_KeepsOrientDBEraTag(t *testing.T) {
	values := DefaultValues("installing_nexus")

	// 예전 설치는 values 에 태그가 없고 차트 기본값(3.64.0)으로 떴다.
	got := keepNexusDataFormat(domain.NexusReleaseName, values, nexusRelease("", "3.64.0"))

	assert.Equal(t, "3.64.0", got["image"].(map[string]any)["tag"])
	assert.Equal(t, domain.NexusAppVersion, values["image"].(map[string]any)["tag"],
		"기본 values 를 고쳐 쓰면 다음 새 설치까지 옛 태그로 뜬다 — 사본을 고친다")
}

func TestKeepNexusDataFormat_UpgradesWithinH2Era(t *testing.T) {
	values := DefaultValues("installing_nexus")

	got := keepNexusDataFormat(domain.NexusReleaseName, values, nexusRelease("3.71.0", "3.64.0"))

	assert.Equal(t, domain.NexusAppVersion, got["image"].(map[string]any)["tag"])
}

func TestKeepNexusDataFormat_NewInstallAndOtherReleasesUntouched(t *testing.T) {
	values := DefaultValues("installing_nexus")

	assert.Equal(t, values, keepNexusDataFormat(domain.NexusReleaseName, values, nil), "새 설치")
	other := map[string]any{"image": map[string]any{"tag": "1.0.0"}}
	assert.Equal(t, other, keepNexusDataFormat(domain.HarborReleaseName, other, nexusRelease("", "3.64.0")),
		"Nexus 가 아닌 릴리스")
}
