package domain

import (
	"regexp"
	"strings"
)

// 스택 구성 → 계획 맵.
//
// 설치 마법사는 사용자가 고른 도구마다 계획 행을 하나 만들고, 그 결과를
// "<슬롯>:<도구키>" 키로 stacks.config.applied_resource_overrides 에 담는다.
// 헤드리스 설치에는 그 화면이 없으므로 같은 맵을 여기서 만든다.

// 도구 표기명 → 자원 기본값 표(stack_resource_defaults)의 tool_key.
// web/src/features/stack/utils/template-overrides.ts 의 TOOL_ID_BY_NAME 과
// 같은 표다 — 한쪽만 고치면 그 도구의 계획이 조용히 빈다.
var resourceToolKeyByName = map[string]string{
	"gitlab ce":                 "gitlab",
	"gitlab package registry":   "gitlab",
	"gitlab registry":           "gitlab-registry",
	"gitlab ci":                 "gitlab-ci",
	"argo cd":                   "argocd",
	"jfrog artifactory":         "jfrog",
	"docker registry":           "docker-hub",
	"github actions":            "github-actions",
	"github container registry": "ghcr",
	"github packages":           "ghcr",
	"victoria metrics":          "victoriametrics",
	"opensearch dashboards":     "opensearch-dashboards",
	"opentelemetry collector":   "opentelemetry-collector",
}

var toolKeySeparators = regexp.MustCompile(`[\s_]+`)

// ResourceToolKey 는 도구 표기명을 자원 기본값 표의 키로 옮긴다.
func ResourceToolKey(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if key, ok := resourceToolKeyByName[normalized]; ok {
		return key
	}
	return toolKeySeparators.ReplaceAllString(normalized, "-")
}

// plannedSelection 은 계획 행 하나 — 어느 자리에 어떤 도구를 골랐는가.
type plannedSelection struct {
	Slot string
	Tool ToolSelection
}

func plannedSelections(cfg StackConfig) []plannedSelection {
	return []plannedSelection{
		{SlotPackageRegistry, cfg.Artifacts.PackageRegistry},
		{SlotSourceRepository, cfg.Artifacts.SourceRepository},
		{SlotContainerRegistry, cfg.Artifacts.ContainerRegistry},
		{SlotStorageBackend, cfg.Artifacts.StorageBackend},
		{SlotCICDPlatform, cfg.Pipeline.CIPlatform},
		{SlotCDTool, cfg.Pipeline.CDTool},
		{SlotMonitoringCollection, cfg.Monitoring.Collection},
		{SlotMonitoringVisualization, cfg.Monitoring.Visualization},
		{SlotLogSearch, cfg.Logging.Search},
		{SlotTraceLayer, cfg.Logging.TraceLayer},
		{SlotTraceExporter, cfg.Logging.TraceExporter},
	}
}

// PlanAppliedResources 는 프로파일과 고른 도구로 applied_resource_overrides 를
// 만든다. baseByToolKey 는 관리자 기본값(stack_resource_defaults)이다.
//
// 기준 벡터가 없는 도구는 건너뛴다 — 0 벡터로 계획하면 requests 가 0 이 되어
// 파드가 무제한으로 뜨므로, 계획을 안 세우고 차트 기본값에 맡기는 편이 낫다.
func PlanAppliedResources(profile string, cfg StackConfig, baseByToolKey map[string]ResourceVector) map[string]ResourceVector {
	planned := make(map[string]ResourceVector)

	for _, selection := range plannedSelections(cfg) {
		if !selection.Tool.Enabled || strings.TrimSpace(selection.Tool.Name) == "" {
			continue
		}
		toolKey := ResourceToolKey(selection.Tool.Name)
		base, ok := baseByToolKey[toolKey]
		if !ok {
			continue
		}
		planned[selection.Slot+":"+toolKey] = PlanResourceVector(profile, selection.Slot, base)
	}

	return planned
}
