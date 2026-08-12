package helm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치 마법사의 "OSS별 Resource Planning" 이 고른 값이 실제 Helm 릴리스에 실려야 한다.
//
// 이 배선이 없던 동안 클러스터의 파드에는 requests/limits 가 아예 없었다 —
// 차트 기본값에 없는 ArgoCD·kube-prometheus-stack 은 0m/0Mi 로 떴다.
// (helm get values argo-cd 결과에 resources 가 한 줄도 없었다.)
// loadResourceDefault 는 repo 가 nil 이면 캐시를 보기도 전에 빠져나간다.
// 캐시만 채워도 되도록 빈 스텁을 함께 넣는다.
type stubResourceDefaultRepo struct{}

func (stubResourceDefaultRepo) List(context.Context) ([]*domain.ResourceDefault, error) {
	return nil, nil
}

func (stubResourceDefaultRepo) Upsert(context.Context, *domain.ResourceDefault) error {
	return nil
}

func withResourceDefault(o *Orchestrator, key string, item *domain.ResourceDefault) {
	o.resourceDefaultRepo = stubResourceDefaultRepo{}
	o.resourceDefaults = map[string]*domain.ResourceDefault{key: item}
	o.defaultsLoaded = true
}

func argocdDefault() *domain.ResourceDefault {
	return &domain.ResourceDefault{
		ToolKey:         "argocd",
		CPURequest:      1,
		CPULimit:        2,
		MemoryRequestGi: 2,
		MemoryLimitGi:   4,
	}
}

func TestResourceValues_UsesPlannedValuesOverAdminDefault(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "argocd", argocdDefault())

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			// 설치 화면의 rowKey 형식: "<slot>:<toolKey>"
			"pipeline.cdTool:argocd": {
				CPURequest:      4,
				CPULimit:        8,
				MemoryRequestGi: 8,
				MemoryLimitGi:   16,
			},
		},
	}

	values := o.resourceDefaultValuesForStep("installing_argocd", &cfg)

	// controller 비율 0.24 — 관리자 기본(1 core)이 아니라 계획값(4 core)에 걸려야 한다.
	controller, ok := values["controller"].(map[string]any)
	require.True(t, ok, "controller 블록이 있어야 한다")
	resources, ok := controller["resources"].(map[string]any)
	require.True(t, ok)
	requests, ok := resources["requests"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "960m", requests["cpu"], "4 core × 0.24")
	assert.Equal(t, "1.92Gi", requests["memory"], "8Gi × 0.24")
}

// 계획값이 없으면 관리자 기본값을 그대로 쓴다 — 기존 동작이다.
func TestResourceValues_FallsBackToAdminDefault(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "argocd", argocdDefault())

	values := o.resourceDefaultValuesForStep("installing_argocd", &domain.StackConfig{})

	controller := values["controller"].(map[string]any)
	requests := controller["resources"].(map[string]any)["requests"].(map[string]any)
	assert.Equal(t, "240m", requests["cpu"], "1 core × 0.24")
}

// 다른 슬롯의 계획값을 끌어다 쓰면 안 된다. 슬롯이 곧 그 단계가 설치하는 자리다.
func TestResourceValues_IgnoresPlanForAnotherSlot(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "argocd", argocdDefault())

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			"monitoring.collection:prometheus": {CPURequest: 9, CPULimit: 9, MemoryRequestGi: 9, MemoryLimitGi: 9},
		},
	}

	values := o.resourceDefaultValuesForStep("installing_argocd", &cfg)

	controller := values["controller"].(map[string]any)
	requests := controller["resources"].(map[string]any)["requests"].(map[string]any)
	assert.Equal(t, "240m", requests["cpu"], "관리자 기본값 그대로")
}

// GitLab 은 소스·CI·레지스트리 슬롯을 겸한다. Helm 릴리스는 하나이므로 기준 슬롯을
// 하나로 정한다 — 소스 저장소 자리다. 겸업 슬롯의 값을 더하면 릴리스 하나에 네 배가 실린다.
func TestResourceValues_GitlabUsesSourceRepositorySlot(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "gitlab-ce", &domain.ResourceDefault{
		ToolKey:         "gitlab-ce",
		CPURequest:      1,
		CPULimit:        2,
		MemoryRequestGi: 2,
		MemoryLimitGi:   4,
	})

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			"artifacts.sourceRepository:gitlab": {
				CPURequest: 5, CPULimit: 10, MemoryRequestGi: 10, MemoryLimitGi: 20,
			},
			"pipeline.cicdPlatform:gitlab-ci": {
				CPURequest: 3, CPULimit: 6, MemoryRequestGi: 6, MemoryLimitGi: 12,
			},
		},
	}

	values := o.resourceDefaultValuesForStep("installing_gitlab", &cfg)

	gitaly := values["gitlab"].(map[string]any)["gitaly"].(map[string]any)
	requests := gitaly["resources"].(map[string]any)["requests"].(map[string]any)
	assert.Equal(t, "1", requests["cpu"], "5 core × 0.20 — CI 슬롯 3 core 를 더하지 않는다")
}

// 슬롯이 맞아도 다른 제품이면 쓰지 않는다. GitHub 을 소스로 고른 스택은
// artifacts.sourceRepository 에 github 이 들어 있고, 그 값을 GitLab 설치에
// 실으면 엉뚱한 도구의 계획이 적용된다.
func TestResourceValues_IgnoresPlanOfAnotherProductInSameSlot(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "gitlab-ce", &domain.ResourceDefault{
		ToolKey:         "gitlab-ce",
		CPURequest:      1,
		CPULimit:        2,
		MemoryRequestGi: 2,
		MemoryLimitGi:   4,
	})

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			"artifacts.sourceRepository:github": {
				CPURequest: 9, CPULimit: 9, MemoryRequestGi: 9, MemoryLimitGi: 9,
			},
		},
	}

	values := o.resourceDefaultValuesForStep("installing_gitlab", &cfg)

	gitaly := values["gitlab"].(map[string]any)["gitaly"].(map[string]any)
	requests := gitaly["resources"].(map[string]any)["requests"].(map[string]any)
	assert.Equal(t, "200m", requests["cpu"], "관리자 기본값 1 core × 0.20")
}

// 계획값이 0 이면 무시한다. 화면에서 아직 계산되지 않은 행이 0 으로 저장될 수 있는데,
// 그대로 쓰면 requests 가 사라져 지금과 같은 빈 값으로 되돌아간다.
func TestResourceValues_IgnoresZeroPlan(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "argocd", argocdDefault())

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			"pipeline.cdTool:argocd": {},
		},
	}

	values := o.resourceDefaultValuesForStep("installing_argocd", &cfg)

	controller := values["controller"].(map[string]any)
	requests := controller["resources"].(map[string]any)["requests"].(map[string]any)
	assert.Equal(t, "240m", requests["cpu"], "관리자 기본값으로 되돌아간다")
}
