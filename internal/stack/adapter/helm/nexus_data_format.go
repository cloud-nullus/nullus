package helm

import (
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// Nexus 는 3.71.0 부터 OrientDB 를 열지 못한다. 설치 단계는 릴리스가 이미 있으면
// 새 values 로 upgrade 하므로(재시도 · 도구 추가), 3.64.0 으로 떠 있던 Nexus 가 새
// 기본 태그로 올라가면 파드가 데이터베이스를 열지 못하고 멈춘다. 그런 릴리스는
// 떠 있는 태그를 그대로 쓴다 — 옮기려면 Sonatype 마이그레이터로 H2 로 바꾼 뒤 다시
// 설치해야 한다(domain.NexusImageTagForRelease).

// keepDeployedNexusDataFormat 은 Nexus 릴리스가 이미 있으면 그 태그 규칙을 values 에 적용한다.
// 릴리스를 읽지 못하면(새 설치 포함) values 를 그대로 돌려준다.
func (h *HelmInstaller) keepDeployedNexusDataFormat(req port.HelmInstallRequest) map[string]any {
	if strings.TrimSpace(req.ReleaseName) != domain.NexusReleaseName {
		return req.Values
	}
	cfg, err := h.newActionConfig(nil, req.Namespace)
	if err != nil {
		return req.Values
	}
	deployed, err := action.NewGet(cfg).Run(req.ReleaseName)
	if err != nil {
		return req.Values
	}
	return keepNexusDataFormat(req.ReleaseName, req.Values, deployed)
}

// keepNexusDataFormat 은 deployed 의 태그가 OrientDB 시대면 values 의 이미지 태그를 그 태그로
// 바꾼 사본을 돌려준다. values 는 고치지 않는다 — 기본 values 를 고쳐 쓰면 다음 새 설치까지
// 옛 태그로 뜬다.
func keepNexusDataFormat(releaseName string, values map[string]any, deployed *release.Release) map[string]any {
	if strings.TrimSpace(releaseName) != domain.NexusReleaseName || deployed == nil {
		return values
	}
	tag := domain.NexusImageTagForRelease(deployedNexusTag(deployed))
	image, _ := values["image"].(map[string]any)
	if current, _ := image["tag"].(string); current == tag {
		return values
	}

	out := make(map[string]any, len(values)+1)
	for k, v := range values {
		out[k] = v
	}
	nextImage := make(map[string]any, len(image)+1)
	for k, v := range image {
		nextImage[k] = v
	}
	nextImage["tag"] = tag
	out["image"] = nextImage
	return out
}

// deployedNexusTag 는 떠 있는 릴리스가 쓰는 태그다. values 에 태그가 없으면(예전 설치)
// 차트 기본값, 곧 차트의 appVersion 으로 떴다.
func deployedNexusTag(rel *release.Release) string {
	if img, ok := rel.Config["image"].(map[string]any); ok {
		if tag, ok := img["tag"].(string); ok && strings.TrimSpace(tag) != "" {
			return tag
		}
	}
	if rel.Chart != nil && rel.Chart.Metadata != nil {
		return rel.Chart.Metadata.AppVersion
	}
	return ""
}
