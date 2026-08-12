package helm

import (
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치 단계가 채우는 계획 슬롯.
//
// 설치 마법사의 "OSS별 Resource Planning" 은 행 키를 "<슬롯>:<도구키>" 로 만든다
// (예: "pipeline.cdTool:argocd"). 슬롯이 곧 그 단계가 설치하는 자리이므로 도구
// 이름이 아니라 슬롯으로 잇는다 — 이름은 gitlab / gitlab-ce 처럼 출처마다 흔들린다.
//
// 여기 없는 단계는 계획 대상이 아니다. cert-manager·runner 처럼 사용자가 고르는
// 자리가 아닌 것들과, Loki 처럼 계획 화면에 슬롯이 없는 것들이다.
var plannedSlotForStep = map[string]string{
	"installing_minio":         "artifacts.storageBackend",
	"installing_gitlab":        "artifacts.sourceRepository",
	"installing_argocd":        "pipeline.cdTool",
	"installing_prometheus":    "monitoring.collection",
	"installing_grafana":       "monitoring.visualization",
	"installing_log_search":    "logging.search",
	"installing_opentelemetry": "logging.traceLayer",
}

// plannedResourceFor 는 이 단계에 적용할 계획값을 찾는다.
//
// GitLab 은 소스·CI·컨테이너 레지스트리·패키지 저장소 슬롯을 겸할 수 있지만 Helm
// 릴리스는 하나다. 기준 슬롯 하나(소스 저장소)만 본다 — 겸업 슬롯을 더하면 릴리스
// 하나에 네 배가 실린다.
func plannedResourceFor(step string, cfg *domain.StackConfig, resourceKey string) *domain.ResourceVector {
	if cfg == nil || len(cfg.AppliedResourceOverrides) == 0 {
		return nil
	}
	slot, ok := plannedSlotForStep[step]
	if !ok {
		return nil
	}

	for key, vector := range cfg.AppliedResourceOverrides {
		keySlot, keyTool, found := strings.Cut(key, ":")
		if !found || keySlot != slot {
			continue
		}
		// 슬롯이 맞아도 다른 제품이면 쓰지 않는다. GitHub 을 소스로 고른 스택은
		// artifacts.sourceRepository 슬롯에 github 이 들어 있는데, 그 값을
		// installing_gitlab 에 실으면 엉뚱한 도구의 계획을 적용하게 된다.
		if !sameProductFamily(keyTool, resourceKey) {
			continue
		}
		if vector.CPURequest <= 0 && vector.CPULimit <= 0 &&
			vector.MemoryRequestGi <= 0 && vector.MemoryLimitGi <= 0 {
			// 아직 계산되지 않은 행은 0 으로 저장될 수 있다. 그대로 쓰면
			// requests 가 통째로 빠져 계획을 안 세운 것과 같아진다.
			return nil
		}
		return &vector
	}
	return nil
}

// 도구 키의 첫 토큰이 같으면 같은 제품군으로 본다 — gitlab 과 gitlab-ce 는 같고,
// github 과 gitlab-ce 는 다르다.
func sameProductFamily(a, b string) bool {
	base := func(v string) string {
		v = strings.ToLower(strings.TrimSpace(v))
		v = strings.ReplaceAll(v, "_", "-")
		head, _, _ := strings.Cut(v, "-")
		return head
	}
	return base(a) != "" && base(a) == base(b)
}

// withPlannedResources 는 관리자 기본값 위에 스택의 계획값을 덮어쓴다.
//
// 단계별 컴포넌트 비율(controller 0.24, gitaly 0.20 …)은 그대로 둔다. 바뀌는 것은
// 그 비율이 곱해질 기준 벡터뿐이다.
func withPlannedResources(item *domain.ResourceDefault, planned *domain.ResourceVector) *domain.ResourceDefault {
	if item == nil || planned == nil {
		return item
	}
	merged := *item
	merged.CPURequest = planned.CPURequest
	merged.CPULimit = planned.CPULimit
	merged.MemoryRequestGi = planned.MemoryRequestGi
	merged.MemoryLimitGi = planned.MemoryLimitGi
	if planned.StorageRequestGi > 0 {
		merged.StorageRequestGi = planned.StorageRequestGi
	}
	if planned.StorageLimitGi > 0 {
		merged.StorageLimitGi = planned.StorageLimitGi
	}
	return &merged
}
