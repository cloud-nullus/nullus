package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 계획을 세워도 그 단계가 계획을 찾아보지 않으면 아무 일도 일어나지 않는다.
//
// Lite 템플릿은 Gitea·Jenkins·Harbor·Argo CD 넷인데, 계획을 소비하는 단계는
// argocd 하나뿐이었다 — 나머지 셋은 슬롯 매핑도 자원 기본값 키도 없어 차트
// 기본값으로 깔렸다. 그래서 8Gi 노드용 프로파일이 실제로는 넷 중 하나에만
// 적용됐다.
func TestPlannedResources_CoverEveryToolInLiteTemplate(t *testing.T) {
	cases := []struct {
		step        string
		resourceKey string
		slot        string
	}{
		{"installing_gitea", "gitea", domain.SlotSourceRepository},
		{"installing_jenkins", "jenkins", domain.SlotCICDPlatform},
		{"installing_harbor", "harbor", domain.SlotContainerRegistry},
		{"installing_argocd", "argocd", domain.SlotCDTool},
		{"installing_nexus", "nexus", domain.SlotContainerRegistry},
	}

	for _, tc := range cases {
		t.Run(tc.step, func(t *testing.T) {
			o := &Orchestrator{namespace: "nullus"}
			assert.Equal(t, tc.resourceKey, o.resourceDefaultKeyForStep(tc.step, &domain.StackConfig{}),
				"%s 단계가 자원 기본값을 못 찾으면 계획 이전에 관리자 기본값도 실리지 않는다", tc.step)
			assert.Equal(t, tc.slot, plannedSlotForStep[tc.step],
				"%s 단계가 슬롯을 모르면 계획값을 찾을 수 없다", tc.step)
		})
	}
}

// 실제로 계획값이 Helm values 까지 내려가는지 확인한다. 매핑만 맞고 값이 안
// 실리면 위 테스트는 통과하면서 파드는 그대로 크게 뜬다.
func TestResourceValues_GiteaUsesPlannedValues(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "gitea", &domain.ResourceDefault{
		ToolKey: "gitea", CPURequest: 1, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4,
	})

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			// local 프로파일이 Gitea 에 내놓는 값.
			"artifacts.sourceRepository:gitea": {
				CPURequest: 0.5, CPULimit: 0.5, MemoryRequestGi: 0.5, MemoryLimitGi: 1,
			},
		},
	}

	values := o.resourceDefaultValuesForStep("installing_gitea", &cfg)
	requests := requestsFromValues(t, values)
	assert.Equal(t, "500m", requests["cpu"])
	assert.Equal(t, "512Mi", requests["memory"])
}

func TestResourceValues_HarborUsesPlannedValues(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "harbor", &domain.ResourceDefault{
		ToolKey: "harbor", CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8,
	})

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			"artifacts.containerRegistry:harbor": {
				CPURequest: 0.5, CPULimit: 1, MemoryRequestGi: 1, MemoryLimitGi: 2,
			},
		},
	}

	values := o.resourceDefaultValuesForStep("installing_harbor", &cfg)
	require.NotEmpty(t, values, "harbor 단계가 resources 를 한 줄도 내지 않는다")
}

func TestResourceValues_JenkinsUsesPlannedValues(t *testing.T) {
	o := &Orchestrator{namespace: "nullus"}
	withResourceDefault(o, "jenkins", &domain.ResourceDefault{
		ToolKey: "jenkins", CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8,
	})

	cfg := domain.StackConfig{
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			"pipeline.cicdPlatform:jenkins": {
				CPURequest: 0.5, CPULimit: 0.5, MemoryRequestGi: 0.5, MemoryLimitGi: 1.5,
			},
		},
	}

	values := o.resourceDefaultValuesForStep("installing_jenkins", &cfg)
	require.NotEmpty(t, values, "jenkins 단계가 resources 를 한 줄도 내지 않는다")
}

// GitLab CI 는 소스 저장소와 같은 릴리스다. CI 슬롯 계획을 GitLab 릴리스에
// 겹쳐 실으면 릴리스 하나에 두 번 계획이 걸린다.
func TestPlannedResources_GitlabKeepsSourceRepositorySlot(t *testing.T) {
	assert.Equal(t, domain.SlotSourceRepository, plannedSlotForStep["installing_gitlab"])
}

func requestsFromValues(t *testing.T, values map[string]any) map[string]any {
	t.Helper()
	resources, ok := values["resources"].(map[string]any)
	require.True(t, ok, "resources 블록이 있어야 한다: %v", values)
	requests, ok := resources["requests"].(map[string]any)
	require.True(t, ok)
	return requests
}
