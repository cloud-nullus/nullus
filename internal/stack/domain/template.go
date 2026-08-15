package domain

import (
	"strings"
	"time"
)

// 설치 규모 프로파일. 설치 마법사의 리소스 계획이 이 값에서 시작한다.
//
// 템플릿이 이 값을 들고 다니는 이유는, 도구 선택만으로는 스택이 몇 Gi 를
// 먹을지 정해지지 않기 때문이다 — 같은 Gitea+Jenkins+Argo CD 라도 standard 로
// 깔면 8Gi 노드에 들어가지 않는다. "무엇을 깔지"와 "얼마나 크게 깔지"는 함께
// 정해져야 한다.
const (
	PlanningProfileLocal      = "local"
	PlanningProfileStartup    = "startup"
	PlanningProfileStandard   = "standard"
	PlanningProfileEnterprise = "enterprise"
)

// NormalizePlanningProfile 은 공백·대소문자를 흡수하고, 빈 값을 기본 프로파일로
// 채운다. 아는 값이 아니면 빈 문자열을 돌려준다 — 오타를 조용히 standard 로
// 바꾸면 8Gi 를 노린 템플릿이 그 두 배로 설치되므로, 판단은 호출자에게 남긴다.
func NormalizePlanningProfile(profile string) string {
	switch normalized := strings.ToLower(strings.TrimSpace(profile)); normalized {
	case "":
		return PlanningProfileStandard
	case PlanningProfileLocal, PlanningProfileStartup, PlanningProfileStandard, PlanningProfileEnterprise:
		return normalized
	default:
		return ""
	}
}

// ToolConfig describes a tool included in a template.
type ToolConfig struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	HelmVersion string `json:"helm_version"`
	AppVersion  string `json:"app_version"`
	Tool        string `json:"tool,omitempty"`
	Version     string `json:"version,omitempty"`
}

// Template represents a Golden Path template for stack deployment.
type Template struct {
	ID                   string        `json:"id"`
	Name                 string        `json:"name"`
	Description          string        `json:"description"`
	Tools                []ToolConfig  `json:"tools"`
	EstimatedInstallTime time.Duration `json:"estimated_install_time"`
	RecommendedUseCase   string        `json:"recommended_use_case"`
	MinResources         string        `json:"min_resources"`
	PlanningProfile      string        `json:"planning_profile,omitempty"`
	CreatedBy            string        `json:"created_by,omitempty"`
}
