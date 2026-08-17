package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func liteStackConfig() StackConfig {
	return StackConfig{
		Artifacts: ArtifactsConfig{
			SourceRepository:  ToolSelection{Name: "Gitea", Version: "1.27.0", Enabled: true},
			ContainerRegistry: ToolSelection{Name: "Harbor", Version: "2.11.0", Enabled: true},
		},
		Pipeline: PipelineConfig{
			CIPlatform: ToolSelection{Name: "Jenkins", Version: "2.568.2", Enabled: true},
			CDTool:     ToolSelection{Name: "Argo CD", Version: "v2.13.3", Enabled: true},
		},
	}
}

// 헤드리스 설치의 핵심 경로. 템플릿의 프로파일과 고른 도구만으로 계획이 서야
// 한다 — 이 함수가 없던 동안 API 설치는 프로파일을 읽고도 아무것도 하지 않았다.
func TestPlanAppliedResources_LiteProfileShrinksEveryTool(t *testing.T) {
	base := map[string]ResourceVector{
		"gitea":   {CPURequest: 1, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4, StorageRequestGi: 15, StorageLimitGi: 30},
		"jenkins": {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 20, StorageLimitGi: 40},
		"harbor":  {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 40, StorageLimitGi: 80},
		"argocd":  {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 3, MemoryLimitGi: 6, StorageRequestGi: 5, StorageLimitGi: 10},
	}

	got := PlanAppliedResources(PlanningProfileLocal, liteStackConfig(), base)

	// 키는 "<슬롯>:<도구키>" — 설치 단계가 슬롯으로 계획을 찾으므로 형태가
	// 어긋나면 계산은 맞아도 한 줄도 적용되지 않는다.
	require.Len(t, got, 4)
	assert.Equal(t, ResourceVector{0.5, 0.5, 0.5, 1, 3.5, 7}, got["artifacts.sourceRepository:gitea"])
	assert.Equal(t, ResourceVector{0.5, 0.5, 0.5, 1.5, 4.5, 9.5}, got["pipeline.cicdPlatform:jenkins"])
	assert.Equal(t, ResourceVector{0.5, 1, 1, 2, 9, 18}, got["artifacts.containerRegistry:harbor"])
	assert.Equal(t, ResourceVector{0.5, 1, 0.5, 1.5, 1, 2.5}, got["pipeline.cdTool:argocd"])

	// 8Gi 노드에 들어가는지가 이 프로파일의 존재 이유다.
	var totalMemoryRequest float64
	for _, vector := range got {
		totalMemoryRequest += vector.MemoryRequestGi
	}
	assert.LessOrEqual(t, totalMemoryRequest, 3.0,
		"Lite 네 도구의 memory request 합이 3Gi 를 넘으면 고정비를 빼고 8Gi 에 들어가지 않는다")
}

// 고르지 않은 자리는 계획하지 않는다. 빈 슬롯에 값을 만들어 두면 설치하지도
// 않은 도구의 자원을 예약한 것처럼 보인다.
func TestPlanAppliedResources_SkipsDisabledSlots(t *testing.T) {
	cfg := liteStackConfig()
	cfg.Artifacts.ContainerRegistry.Enabled = false

	got := PlanAppliedResources(PlanningProfileLocal, cfg,
		map[string]ResourceVector{"gitea": {CPURequest: 1}, "jenkins": {CPURequest: 2}, "argocd": {CPURequest: 2}})

	assert.NotContains(t, got, "artifacts.containerRegistry:harbor")
	assert.Len(t, got, 3)
}

// 기준 벡터가 없는 도구는 건너뛴다. 0 벡터로 계획하면 requests 가 0 이 되어
// 파드가 무제한으로 뜬다 — 계획을 안 세우느니만 못하다.
func TestPlanAppliedResources_SkipsToolsWithoutDefaults(t *testing.T) {
	got := PlanAppliedResources(PlanningProfileLocal, liteStackConfig(),
		map[string]ResourceVector{"gitea": {CPURequest: 1, MemoryRequestGi: 2}})

	assert.Contains(t, got, "artifacts.sourceRepository:gitea")
	assert.NotContains(t, got, "pipeline.cicdPlatform:jenkins")
	assert.Len(t, got, 1)
}

// standard 는 관리자 기본값 그대로다. 프로파일을 고르지 않은 기존 스택이
// 조용히 다른 크기로 깔리면 안 된다.
func TestPlanAppliedResources_StandardMatchesAdminDefaults(t *testing.T) {
	base := map[string]ResourceVector{
		"gitea":   {CPURequest: 1, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4, StorageRequestGi: 15, StorageLimitGi: 30},
		"jenkins": {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 20, StorageLimitGi: 40},
		"harbor":  {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 40, StorageLimitGi: 80},
		"argocd":  {CPURequest: 2, CPULimit: 4, MemoryRequestGi: 3, MemoryLimitGi: 6, StorageRequestGi: 5, StorageLimitGi: 10},
	}

	got := PlanAppliedResources(PlanningProfileStandard, liteStackConfig(), base)

	assert.Equal(t, base["gitea"], got["artifacts.sourceRepository:gitea"])
	assert.Equal(t, base["harbor"], got["artifacts.containerRegistry:harbor"])
}

// 관측 슬롯까지 포함한 전체 배선. 슬롯 하나가 빠지면 그 도구만 조용히 계획에서
// 새므로 모아서 고정한다.
func TestPlanAppliedResources_CoversEverySelectableSlot(t *testing.T) {
	cfg := StackConfig{
		Artifacts: ArtifactsConfig{
			PackageRegistry:   ToolSelection{Name: "Nexus", Enabled: true},
			SourceRepository:  ToolSelection{Name: "GitLab CE", Enabled: true},
			ContainerRegistry: ToolSelection{Name: "Harbor", Enabled: true},
			StorageBackend:    ToolSelection{Name: "MinIO", Enabled: true},
		},
		Pipeline: PipelineConfig{
			CIPlatform: ToolSelection{Name: "GitLab CI", Enabled: true},
			CDTool:     ToolSelection{Name: "Argo CD", Enabled: true},
		},
		Monitoring: MonitoringConfig{
			Collection:    ToolSelection{Name: "Prometheus", Enabled: true},
			Visualization: ToolSelection{Name: "Grafana", Enabled: true},
		},
		Logging: LoggingConfig{
			Search:        ToolSelection{Name: "OpenSearch", Enabled: true},
			TraceLayer:    ToolSelection{Name: "Tempo", Enabled: true},
			TraceExporter: ToolSelection{Name: "OpenTelemetry Collector", Enabled: true},
		},
	}

	base := map[string]ResourceVector{}
	for _, key := range []string{"nexus", "gitlab", "harbor", "minio", "gitlab-ci", "argocd",
		"prometheus", "grafana", "opensearch", "tempo", "opentelemetry-collector"} {
		base[key] = ResourceVector{CPURequest: 1, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4}
	}

	got := PlanAppliedResources(PlanningProfileStandard, cfg, base)

	assert.Len(t, got, 11, "고를 수 있는 슬롯 11개가 모두 계획돼야 한다")
	for _, key := range []string{
		"artifacts.packageRegistry:nexus",
		"artifacts.sourceRepository:gitlab",
		"artifacts.containerRegistry:harbor",
		"artifacts.storageBackend:minio",
		"pipeline.cicdPlatform:gitlab-ci",
		"pipeline.cdTool:argocd",
		"monitoring.collection:prometheus",
		"monitoring.visualization:grafana",
		"logging.search:opensearch",
		"logging.traceLayer:tempo",
		"logging.traceExporter:opentelemetry-collector",
	} {
		assert.Contains(t, got, key)
	}
}

// 도구 이름은 표기가 흔들린다 — "Argo CD" 와 "GitLab CE" 는 자원 기본값 표의
// argocd / gitlab 과 철자가 다르다. 여기서 흡수하지 않으면 기준 벡터를 못 찾아
// 계획이 통째로 빈다.
func TestResourceToolKey_NormalizesDisplayNames(t *testing.T) {
	cases := map[string]string{
		"Argo CD":                 "argocd",
		"argo cd":                 "argocd",
		"GitLab CE":               "gitlab",
		"GitLab CI":               "gitlab-ci",
		"GitLab Registry":         "gitlab-registry",
		"Gitea":                   "gitea",
		"Harbor":                  "harbor",
		"Jenkins":                 "jenkins",
		"MinIO":                   "minio",
		"OpenTelemetry Collector": "opentelemetry-collector",
		"Victoria Metrics":        "victoriametrics",
		"OpenSearch Dashboards":   "opensearch-dashboards",
		"JFrog Artifactory":       "jfrog",
		"GHCR":                    "ghcr",
		// 표에 없는 이름은 소문자·하이픈으로만 정리해 그대로 쓴다.
		"Some New Tool": "some-new-tool",
	}

	for name, want := range cases {
		assert.Equal(t, want, ResourceToolKey(name), "name=%s", name)
	}
}
