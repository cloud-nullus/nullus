package helm

import (
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 스택이 설치하는 OSS 가 자기 메트릭을 Prometheus 에 내주도록 켠다.
//
// 켜지 않으면 파드의 CPU·메모리(cadvisor 경유)만 보이고 도구 자신이 아는 것 —
// Argo CD 의 동기화 실패 수, Loki 의 수집 지연, MinIO 의 버킷 사용량 — 은 어디에도
// 남지 않는다. "모니터링을 설치했는데 정작 도구 상태를 모른다" 가 그 상태다.
//
// 두 가지가 함께 필요하다:
//
//  1. 차트가 ServiceMonitor 를 만들도록 켜기
//  2. 거기에 kube-prometheus-stack 의 release 라벨을 붙이기 —
//     그 차트는 기본값(serviceMonitorSelectorNilUsesHelmValues)에서 자기 라벨이
//     붙은 모니터만 고른다. 라벨이 없으면 리소스는 생기는데 스크랩은 안 된다.
//
// Prometheus 를 고르지 않은 스택에서는 아무것도 켜지 않는다. ServiceMonitor 는
// Prometheus Operator 의 CRD 라, 오퍼레이터가 없으면 설치가
// "no matches for kind ServiceMonitor" 로 통째로 멈춘다.

// prometheusReleaseLabels 는 kube-prometheus-stack 이 고르는 라벨이다.
func prometheusReleaseLabels() map[string]any {
	return map[string]any{"release": prometheusStackReleaseLabel}
}

// serviceMonitorValuesForStep 은 단계별로 자기 메트릭 노출을 켜는 values 를 돌려준다.
//
// 차트마다 키가 다르다 — 같은 "serviceMonitor" 라도 라벨 키가 additionalLabels
// 인 곳과 labels 인 곳이 갈린다. 값 경로는 각 차트의 values 에서 확인한 것이다.
func serviceMonitorValuesForStep(step string, cfg *domain.StackConfig) map[string]any {
	if cfg == nil || !cfg.Monitoring.Collection.Enabled {
		return nil
	}

	switch step {
	case "installing_argocd":
		// Argo CD 는 컴포넌트마다 metrics Service 와 ServiceMonitor 가 따로 있다.
		// 컨트롤러만 켜면 동기화 상태는 보이지만 저장소 서버·서버의 지표는 빠진다.
		component := map[string]any{
			"metrics": map[string]any{
				"enabled": true,
				"serviceMonitor": map[string]any{
					"enabled":          true,
					"additionalLabels": prometheusReleaseLabels(),
				},
			},
		}
		return map[string]any{
			"controller":     deepCopyMap(component),
			"server":         deepCopyMap(component),
			"repoServer":     deepCopyMap(component),
			"applicationSet": deepCopyMap(component),
			"notifications":  deepCopyMap(component),
		}

	case "installing_grafana":
		// grafana 차트만 라벨 키가 labels 다 (다른 차트는 additionalLabels).
		return map[string]any{
			"serviceMonitor": map[string]any{
				"enabled": true,
				"labels":  prometheusReleaseLabels(),
			},
		}

	case "installing_minio":
		return map[string]any{
			"metrics": map[string]any{
				"serviceMonitor": map[string]any{
					"enabled": true,
					// minio 차트의 템플릿 조건이 `and .enabled .includeNode` 다.
					// enabled 만 켜면 values 에는 실리는데 ServiceMonitor 는
					// 만들어지지 않는다 — 켠 줄 알고 넘어가기 딱 좋은 함정이라
					// 실제로 한 번 밟았다.
					"includeNode":      true,
					"additionalLabels": prometheusReleaseLabels(),
				},
			},
		}

	case "installing_logging", "installing_log_search":
		// installing_log_search 는 고른 제품에 따라 차트가 갈린다. Loki 를 골랐을
		// 때만 이 키가 있으므로 그 경우에만 켠다 — OpenSearch 차트에 넣으면
		// 모르는 값이라 무시될 뿐이지만, 켜지지도 않은 것을 켰다고 오해하게 된다.
		if step == "installing_log_search" && !lokiSelected(cfg.Logging.Search) {
			return nil
		}
		return map[string]any{
			"serviceMonitor": map[string]any{
				"enabled":          true,
				"additionalLabels": prometheusReleaseLabels(),
			},
		}

	case "installing_opentelemetry":
		// 추적 계층은 Tempo 를 골랐을 때만 이 키를 갖는다.
		if normalizeToolName(cfg.Logging.TraceLayer.Name) == "jaeger" {
			return nil
		}
		return map[string]any{
			"serviceMonitor": map[string]any{
				"enabled":          true,
				"additionalLabels": prometheusReleaseLabels(),
			},
		}
	}

	return nil
}
