package helm

import (
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// OpenTelemetry Collector 를 스택의 관측 백엔드에 붙이는 values 를 만든다.
//
// 수집기는 추적 저장소(installing_opentelemetry)와 다른 역할이다 — 애플리케이션이
// 보낸 OTLP 를 받아 추적은 Tempo/Jaeger 로, 메트릭은 Prometheus 로, 로그는 Loki 로
// 나눠 보낸다. 그래서 설치 단계도 릴리스도 따로 둔다.
//
// 여기서 만드는 값은 차트 기본 config 위에 병합된다. 수신기(otlp/jaeger/zipkin)와
// 프로세서(memory_limiter/batch)는 차트 기본값을 그대로 쓰고, 어디로 내보낼지만 정한다.

const (
	// 수집기 자체 텔레메트리 포트. 차트가 metrics 라는 이름으로 노출한다.
	otelCollectorSelfMetricsPort = "metrics"

	// prometheus exporter 가 여는 포트. 수집기가 OTLP 로 받은 메트릭을 여기로 낸다.
	// 자체 텔레메트리(8888)와 섞이지 않도록 포트를 따로 쓴다.
	otelCollectorExporterPortName = "prom-exporter"
	otelCollectorExporterPort     = 8889

	// contrib 배포판이어야 loki 처럼 코어에 없는 exporter 를 쓸 수 있다.
	otelCollectorImageRepository = "otel/opentelemetry-collector-contrib"

	// 차트 기본 config 에 이미 선언된 exporter. 보낼 곳이 없을 때 쓴다 —
	// 없는 주소를 가리키면 수집기가 영원히 내보내기 실패 로그만 쌓는다.
	otelDebugExporter = "debug"

	// kube-prometheus-stack 은 기본값(serviceMonitorSelectorNilUsesHelmValues)에서
	// 자기 release 라벨이 붙은 모니터만 고른다. 라벨이 없으면 ServiceMonitor 를
	// 만들어도 스크랩되지 않아 "메트릭이 안 보인다"로 끝난다.
	prometheusStackReleaseLabel = "kube-prometheus-stack"
)

func otelCollectorValues(cfg *domain.StackConfig) map[string]any {
	traceExporter, traceConfig := otelTraceExporter(cfg)
	logExporter, logConfig := otelLogExporter(cfg)

	exporters := map[string]any{
		"prometheus": map[string]any{
			"endpoint": fmt.Sprintf("0.0.0.0:%d", otelCollectorExporterPort),
		},
	}
	if traceConfig != nil {
		exporters[traceExporter] = traceConfig
	}
	if logConfig != nil {
		exporters[logExporter] = logConfig
	}

	// Loki 는 라벨로만 스트림을 가른다. 힌트를 주지 않으면 모든 로그가
	// {exporter="OTLP"} 한 덩어리로 들어가 네임스페이스나 컨테이너로 좁힐 수
	// 없고, 전문 검색만 남는다.
	//
	// 파드 이름은 라벨로 올리지 않는다 — 파드가 재생성될 때마다 새 스트림이
	// 생겨 카디널리티가 폭발한다. 본문 속성으로는 그대로 남아 검색된다.
	logProcessors := []any{"memory_limiter", "batch"}
	processors := map[string]any{}
	if logExporter == "loki" {
		processors["resource/loki"] = map[string]any{
			"attributes": []any{
				map[string]any{
					"key":    "loki.resource.labels",
					"value":  "k8s.namespace.name, k8s.container.name",
					"action": "insert",
				},
			},
		}
		logProcessors = []any{"memory_limiter", "resource/loki", "batch"}
	}

	values := map[string]any{
		"mode":         "deployment",
		"replicaCount": 1,
		"image": map[string]any{
			"repository": otelCollectorImageRepository,
		},
		"ports": map[string]any{
			otelCollectorSelfMetricsPort: map[string]any{
				// 차트 기본값은 꺼짐이다. 켜지 않으면 ServiceMonitor 가 가리킬
				// 포트가 Service 에 없어 스크랩 대상이 사라진다.
				"enabled": true,
			},
			otelCollectorExporterPortName: map[string]any{
				"enabled":       true,
				"containerPort": otelCollectorExporterPort,
				"servicePort":   otelCollectorExporterPort,
				"protocol":      "TCP",
			},
		},
		"config": map[string]any{
			"exporters":  exporters,
			"processors": processors,
			"service": map[string]any{
				"pipelines": map[string]any{
					"traces":  map[string]any{"exporters": []any{traceExporter}},
					"metrics": map[string]any{"exporters": []any{"prometheus"}},
					"logs": map[string]any{
						"exporters":  []any{logExporter},
						"processors": logProcessors,
					},
				},
			},
		},
	}

	// ServiceMonitor 는 Prometheus Operator 가 세우는 CRD 다. 오퍼레이터 없는
	// 스택에 만들면 "no matches for kind ServiceMonitor" 로 설치가 통째로 멈춘다.
	if cfg != nil && cfg.Monitoring.Collection.Enabled {
		values["serviceMonitor"] = map[string]any{
			"enabled": true,
			"extraLabels": map[string]any{
				"release": prometheusStackReleaseLabel,
			},
			"metricsEndpoints": []any{
				map[string]any{"port": otelCollectorSelfMetricsPort},
				map[string]any{"port": otelCollectorExporterPortName},
			},
		}
	}

	return values
}

// otelAgentValues 는 노드마다 서는 수집 에이전트의 values 를 만든다.
//
// Loki 는 저장소일 뿐 파드 로그를 스스로 긁어오지 않는다. 컨테이너 로그는 노드의
// /var/log/pods 아래 파일이므로 그것을 읽는 주체가 노드마다 있어야 한다 —
// 그래서 게이트웨이(Deployment)와 별개로 DaemonSet 을 하나 더 세운다.
//
// 에이전트는 저장소로 직접 보내지 않고 게이트웨이로 넘긴다. 출구를 하나로 모아야
// 저장소를 바꿀 때 고칠 곳이 한 군데로 남는다.
func otelAgentValues(namespace string) map[string]any {
	// 에이전트는 받는 쪽이 아니다. DaemonSet 모드에서 차트는 이 포트들에
	// hostPort 를 붙이므로, 쓰지도 않을 노드 포트를 물지 않도록 전부 끈다.
	disabledPort := map[string]any{"enabled": false}

	return map[string]any{
		"mode": "daemonset",
		"image": map[string]any{
			"repository": otelCollectorImageRepository,
		},
		"presets": map[string]any{
			"logsCollection": map[string]any{
				"enabled": true,
				// 수집기 자신의 로그는 담지 않는다 — 자기 로그를 자기가 실어
				// 보내면 오류가 날 때 되먹임으로 증폭된다.
				"includeCollectorLogs": false,
			},
			// 파드·네임스페이스·워크로드 이름을 붙인다. 없으면 로그가 어느
			// 워크로드 것인지 알 수 없어 검색이 무의미해진다.
			"kubernetesAttributes": map[string]any{"enabled": true},
		},
		"ports": map[string]any{
			"otlp":           disabledPort,
			"otlp-http":      disabledPort,
			"jaeger-compact": disabledPort,
			"jaeger-thrift":  disabledPort,
			"jaeger-grpc":    disabledPort,
			"zipkin":         disabledPort,
		},
		"config": map[string]any{
			"exporters": map[string]any{
				"otlp/gateway": map[string]any{
					"endpoint": domain.OTelCollectorOTLPGRPCEndpoint(namespace),
					"tls":      map[string]any{"insecure": true},
				},
			},
			"service": map[string]any{
				"pipelines": map[string]any{
					"logs": map[string]any{
						// 수신기를 명시하지 않으면 차트 기본 파이프라인이 우리가
						// 쓰지 않는 otlp 를 계속 참조해 기동하다 죽는다.
						"receivers": []any{"filelog"},
						"exporters": []any{"otlp/gateway"},
					},
					// 메트릭·추적은 게이트웨이가 애플리케이션에게서 직접 받는다.
					// 에이전트까지 거치게 하면 경로만 길어진다.
					"traces":  nil,
					"metrics": nil,
				},
			},
		},
	}
}

// otelTraceExporter 는 추적을 어디로 보낼지 고른다.
//
// 고르지 않은 백엔드로 보내는 exporter 를 만들면 안 된다 — 주소가 풀리지 않아
// 수집기는 뜨는데 추적만 조용히 사라지는, 원인을 찾기 어려운 상태가 된다.
func otelTraceExporter(cfg *domain.StackConfig) (string, map[string]any) {
	if cfg == nil || !cfg.Logging.TraceLayer.Enabled {
		return otelDebugExporter, nil
	}

	insecureOTLP := func(endpoint string) map[string]any {
		return map[string]any{
			"endpoint": endpoint,
			// 클러스터 내부 평문 통신이다. 기본값(TLS 사용)을 두면 핸드셰이크에서
			// 막혀 추적이 한 건도 저장되지 않는다.
			"tls": map[string]any{"insecure": true},
		}
	}

	switch normalizeToolName(cfg.Logging.TraceLayer.Name) {
	case "jaeger":
		// jaeger 차트의 수집기 Service 이름이다. 릴리스명(jaeger)에 -collector 가 붙는다.
		return "otlp/jaeger", insecureOTLP("jaeger-collector:4317")
	case "", "tempo":
		// 이름이 비면 추적 저장소의 표준 기본값인 Tempo 로 본다
		// (domain.canonicalToolNameByKey 와 같은 판단).
		return "otlp/tempo", insecureOTLP("tempo:4317")
	default:
		return otelDebugExporter, nil
	}
}

// otelLogExporter 는 로그를 어디로 보낼지 고른다.
//
// Loki 만 수집기가 직접 밀어 넣을 수 있다. OpenSearch 계열은 수집 경로가 달라
// (Fluent Bit/Promtail) 여기서 다루지 않는다.
//
// collection 과 search 를 모두 본다 — 설치 마법사는 Loki 를 search 칸에서
// 고르게 하고, 설정을 직접 쓰는 경로는 collection 에 둔다. 한쪽만 보면
// 같은 Loki 를 두고 로그 파이프라인이 붙기도 하고 안 붙기도 한다.
func otelLogExporter(cfg *domain.StackConfig) (string, map[string]any) {
	if cfg == nil {
		return otelDebugExporter, nil
	}

	if !lokiSelected(cfg.Logging.Collection) && !lokiSelected(cfg.Logging.Search) {
		return otelDebugExporter, nil
	}

	return "loki", map[string]any{
		"endpoint": "http://loki:3100/loki/api/v1/push",
	}
}

// lokiSelected 는 선택이 Loki 를 가리키는지 본다.
// 로그 수집 칸은 이름이 비면 Loki 가 기본값이다.
func lokiSelected(sel domain.ToolSelection) bool {
	if !sel.Enabled {
		return false
	}
	switch normalizeToolName(sel.Name) {
	case "", "loki":
		return true
	default:
		return false
	}
}
