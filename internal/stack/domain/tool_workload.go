package domain

import (
	"encoding/json"
	"slices"
	"strings"
)

// ToolWorkload 는 스택 설정이 클러스터에 실제로 띄우는 OSS 하나를 가리킨다.
//
// NamePrefixes 는 그 OSS 가 만드는 파드·워크로드 이름의 접두사다. 릴리스명이 곧
// 이름 접두사가 되므로 설치 경로가 쓰는 릴리스 상수와 같은 값을 쓴다.
type ToolWorkload struct {
	Key          string
	Name         string
	Version      string
	NamePrefixes []string
}

// canonicalToolNameByKey 는 선택에 이름이 비어 있을 때 쓸 기본 표기다.
var canonicalToolNameByKey = map[string]string{
	"source_repository":  "gitlab",
	"cd_tool":            "argocd",
	"collection":         "prometheus",
	"visualization":      "grafana",
	"logging_collection": "loki",
	"logging_search":     "opensearch",
	"trace_layer":        "tempo",
	"trace_exporter":     "opentelemetry-collector",
	"storage_backend":    "minio",
	"authentication":     "openbao",
}

// InstalledToolWorkloads 는 스택 설정에서 "이 스택이 클러스터에 설치하는 OSS" 목록을
// 뽑는다. 모니터링이 무엇을 보여줄지 정하는 단일 기준점이다 — 스택 상세 화면과
// 플랫폼 대시보드가 같은 답을 보게 하려면 양쪽 다 이 함수를 거쳐야 한다.
//
// 여기에 없는 OSS 는 파드가 멀쩡히 떠 있어도 어느 화면에도 나오지 않는다.
func InstalledToolWorkloads(cfg StackConfig) []ToolWorkload {
	workload := func(key string, sel ToolSelection, prefixes ...string) ToolWorkload {
		name := strings.TrimSpace(sel.Name)
		if name == "" {
			if name = canonicalToolNameByKey[key]; name == "" {
				name = key
			}
		}
		version := strings.TrimSpace(sel.Version)
		if version == "" {
			version = "-"
		}
		return ToolWorkload{Key: key, Name: name, Version: version, NamePrefixes: prefixes}
	}

	type candidate struct {
		workload ToolWorkload
		enabled  bool
	}

	authProvider := ""
	if cfg.Authentication != nil {
		authProvider = strings.TrimSpace(cfg.Authentication.Provider)
	}

	candidates := []candidate{
		{
			// OpenBao 만 스택 네임스페이스에 설치된다. 다른 IdP 는 외부 서비스다.
			workload: workload("authentication",
				ToolSelection{Name: authProvider, Version: "latest"}, "openbao"),
			enabled: strings.EqualFold(authProvider, "openbao"),
		},
		{workload("source_repository", cfg.Artifacts.SourceRepository, "gitlab"), cfg.Artifacts.SourceRepository.Enabled},
		{workload("cd_tool", cfg.Pipeline.CDTool, "argo-cd-argocd"), cfg.Pipeline.CDTool.Enabled},
		{workload("collection", cfg.Monitoring.Collection,
			"prometheus", "kube-prometheus-stack", "alertmanager-kube-prometheus-stack"), cfg.Monitoring.Collection.Enabled},
		{workload("visualization", cfg.Monitoring.Visualization, "grafana"), cfg.Monitoring.Visualization.Enabled},
		{workload("logging_collection", cfg.Logging.Collection, loggingCollectionPrefixes...), cfg.Logging.Collection.Enabled},
		{workload("trace_layer", cfg.Logging.TraceLayer, "tempo", "jaeger"), cfg.Logging.TraceLayer.Enabled},
		// 릴리스명이 곧 파드 이름 접두사다. 차트 이름(opentelemetry-collector)이
		// 아니라 릴리스명을 써야 실제 파드와 맞는다.
		{workload("trace_exporter", cfg.Logging.TraceExporter, OTelCollectorReleaseName), cfg.Logging.TraceExporter.Enabled},
		{workload("storage_backend", cfg.Artifacts.StorageBackend, MinIOServiceName, "minio"), cfg.Artifacts.StorageBackend.Enabled},
	}

	// 로그 저장소도 고른 제품에 따라 파드 이름이 달라진다. 접두사를 정적으로
	// 적어 두면 Loki 를 골랐을 때 opensearch 파드를 찾다가 "0 파드 warning" 으로
	// 남는다 — 설치는 정상인데 화면에서만 죽은 것처럼 보인다.
	searchPrefixes := logStoreNamePrefixes(cfg.Logging.Search)
	if searchPrefixes != nil &&
		// 수집 항목은 loki-* 파드를 센다. 검색까지 같은 파드를 가리키면 같은
		// 릴리스를 두 번 계상한다 (Nexus 가 두 슬롯을 겸할 때와 같은 이유).
		!(cfg.Logging.Collection.Enabled && slices.Equal(searchPrefixes, loggingCollectionPrefixes)) {
		candidates = append(candidates, candidate{
			workload("logging_search", cfg.Logging.Search, searchPrefixes...), true,
		})
	}

	// 레지스트리는 고른 제품에 따라 파드 접두사가 달라지고, 아예 설치되지 않는
	// 선택도 있어 다른 항목처럼 정적으로 적을 수 없다.
	containerPrefixes := registryNamePrefixes(cfg.Artifacts.ContainerRegistry)
	if containerPrefixes != nil {
		candidates = append(candidates, candidate{
			workload("container_registry", cfg.Artifacts.ContainerRegistry, containerPrefixes...), true,
		})
	}
	// Nexus 는 컨테이너 레지스트리와 패키지 저장소를 겸할 수 있다. 같은 파드를
	// 가리키는 항목을 둘로 만들면 OSS 목록과 파드 커버리지가 이중 계상된다.
	if packagePrefixes := registryNamePrefixes(cfg.Artifacts.PackageRegistry); packagePrefixes != nil &&
		!slices.Equal(packagePrefixes, containerPrefixes) {
		candidates = append(candidates, candidate{
			workload("package_registry", cfg.Artifacts.PackageRegistry, packagePrefixes...), true,
		})
	}

	out := make([]ToolWorkload, 0, len(candidates))
	for _, c := range candidates {
		if c.enabled {
			out = append(out, c.workload)
		}
	}
	return out
}

// loggingCollectionPrefixes 는 로그 수집 슬롯이 띄우는 워크로드 이름 접두사다.
// 이 슬롯은 Loki 전용이라(installing_logging 이 loki 차트를 고정으로 쓴다)
// 선택 이름과 무관하게 고정이다.
var loggingCollectionPrefixes = []string{"loki"}

// logStoreNamePrefixes 는 로그 검색 저장소 선택이 띄우는 워크로드 이름 접두사를
// 돌려준다. 고르지 않았으면 nil 이다.
//
// 릴리스명이 곧 파드 이름 접두사이므로 설치 경로가 고르는 차트와 같은 기준을
// 쓴다 (helm 어댑터의 resolveChartSpecForStep).
func logStoreNamePrefixes(sel ToolSelection) []string {
	if !sel.Enabled || strings.EqualFold(strings.TrimSpace(sel.Version), "external") {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(sel.Name)) {
	case "loki":
		return []string{"loki"}
	case "elasticsearch":
		return []string{"elasticsearch"}
	default:
		// 이름이 비면 검색 저장소의 기본값은 OpenSearch 다.
		return []string{"opensearch"}
	}
}

// registryNamePrefixes 는 레지스트리 선택이 띄우는 워크로드 이름 접두사를 돌려준다.
// 별도 워크로드가 생기지 않는 선택은 nil 을 돌려 대상에서 뺀다:
//
//   - 외부 서비스(version=external): 클러스터에 파드가 없다. 항목을 만들면
//     영구히 "0 파드 warning" 으로 남는다.
//   - GitLab 내장 레지스트리 / Docker Registry: gitlab 차트가 gitlab-registry-*
//     로 함께 올리므로 source_repository 의 "gitlab" 접두사가 이미 포함한다.
func registryNamePrefixes(sel ToolSelection) []string {
	if !sel.Enabled || strings.EqualFold(strings.TrimSpace(sel.Version), "external") {
		return nil
	}

	switch name := strings.ToLower(strings.TrimSpace(sel.Name)); {
	case strings.HasPrefix(name, "harbor"):
		return []string{HarborReleaseName}
	case strings.HasPrefix(name, "nexus"), strings.HasPrefix(name, "sonatype"):
		return []string{NexusReleaseName}
	default:
		return nil
	}
}

// DecodeStackConfig 는 Stack.Config 에 실려오는 값을 StackConfig 로 되돌린다.
// 저장소에서 읽으면 JSONB 라 map 으로, 메모리에서 오면 구조체로 들어온다.
// 해석에 실패하면 빈 설정으로 떨어뜨린다 — 모니터링은 최선 노력 경로다.
func DecodeStackConfig(raw any) StackConfig {
	switch cfg := raw.(type) {
	case nil:
		return StackConfig{}
	case StackConfig:
		return cfg
	case *StackConfig:
		if cfg == nil {
			return StackConfig{}
		}
		return *cfg
	default:
		b, err := json.Marshal(cfg)
		if err != nil {
			return StackConfig{}
		}
		var decoded StackConfig
		if err := json.Unmarshal(b, &decoded); err != nil {
			return StackConfig{}
		}
		return decoded
	}
}
