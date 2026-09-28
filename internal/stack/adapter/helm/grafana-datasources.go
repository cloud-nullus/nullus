package helm

import (
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// Grafana 는 스택이 함께 깐 백엔드에 연결돼야 쓸모가 있다.
//
// 설치는 Prometheus·Loki·Tempo 를 세우고 OTel Collector 가 셋으로 내보내도록
// 배선하는데, 정작 그것들을 보는 화면인 Grafana 에는 데이터소스가 한 줄도
// 들어가지 않았다. 스택은 completed 이고 파드도 전부 Running 인데 Grafana 를
// 열면 비어 있다 — "메트릭·로깅·트레이싱 템플릿" 이 이름값을 못 한다.
//
// 주소는 차트가 만드는 서비스 이름이다. 같은 네임스페이스라 짧은 이름으로 닿는다.
const (
	prometheusDatasourceURL    = "http://kube-prometheus-stack-prometheus:9090"
	lokiDatasourceURL          = "http://loki:3100"
	tempoDatasourceURL         = "http://tempo:3100"
	jaegerDatasourceURL        = "http://jaeger-query:16686"
	opensearchDatasourceURL    = "http://opensearch-cluster-master:9200"
	elasticsearchDatasourceURL = "http://elasticsearch-master:9200"
)

// grafanaDatasourceValues 는 이 스택이 실제로 설치한 백엔드만 데이터소스로 건다.
//
// 고르지 않은 도구를 걸면 Grafana 시작부터 연결 오류가 뜨고, 사용자는 무엇이
// 진짜 문제인지 가려낼 수 없게 된다.
func grafanaDatasourceValues(cfg *domain.StackConfig) map[string]any {
	if cfg == nil {
		return nil
	}

	var datasources []any
	add := func(name, dsType, url string, isDefault bool) {
		ds := map[string]any{
			"name":      name,
			"type":      dsType,
			"url":       url,
			"access":    "proxy",
			"isDefault": isDefault,
		}
		datasources = append(datasources, ds)
	}

	// 메트릭. kube-prometheus-stack 은 모니터링 수집을 고르면 함께 깔린다.
	if cfg.Monitoring.Collection.Enabled {
		add("Prometheus", "prometheus", prometheusDatasourceURL, true)
	}

	// 로그. 어느 검색 계층을 골랐는지에 따라 타입도 주소도 다르다.
	if cfg.Logging.Search.Enabled {
		switch normalizeToolName(cfg.Logging.Search.Name) {
		case "loki":
			add("Loki", "loki", lokiDatasourceURL, false)
		case "opensearch":
			add("OpenSearch", "elasticsearch", opensearchDatasourceURL, false)
		case "elasticsearch":
			add("Elasticsearch", "elasticsearch", elasticsearchDatasourceURL, false)
		}
	}

	// 추적.
	if cfg.Logging.TraceLayer.Enabled {
		switch normalizeToolName(cfg.Logging.TraceLayer.Name) {
		case "tempo":
			add("Tempo", "tempo", tempoDatasourceURL, false)
		case "jaeger":
			add("Jaeger", "jaeger", jaegerDatasourceURL, false)
		}
	}

	if len(datasources) == 0 {
		return nil
	}

	return map[string]any{
		"datasources": map[string]any{
			"datasources.yaml": map[string]any{
				"apiVersion":  1,
				"datasources": datasources,
			},
		},
	}
}
