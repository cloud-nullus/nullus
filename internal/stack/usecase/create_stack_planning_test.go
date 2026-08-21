package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func planningResourceDefaults(t *testing.T) *stackrepo.MemoryResourceDefaultRepository {
	t.Helper()
	repo := stackrepo.NewMemoryResourceDefaultRepository()
	for _, item := range []*domain.ResourceDefault{
		{ToolKey: "gitea", DisplayName: "Gitea", CPURequest: 1, CPULimit: 2, MemoryRequestGi: 2, MemoryLimitGi: 4, StorageRequestGi: 15, StorageLimitGi: 30},
		{ToolKey: "jenkins", DisplayName: "Jenkins", CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 20, StorageLimitGi: 40},
		{ToolKey: "harbor", DisplayName: "Harbor", CPURequest: 2, CPULimit: 4, MemoryRequestGi: 4, MemoryLimitGi: 8, StorageRequestGi: 40, StorageLimitGi: 80},
		{ToolKey: "argocd", DisplayName: "Argo CD", CPURequest: 2, CPULimit: 4, MemoryRequestGi: 3, MemoryLimitGi: 6, StorageRequestGi: 5, StorageLimitGi: 10},
	} {
		require.NoError(t, repo.Upsert(context.Background(), item))
	}
	return repo
}

func liteConfig() domain.StackConfig {
	return domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			SourceRepository:  domain.ToolSelection{Name: "Gitea", Enabled: true},
			ContainerRegistry: domain.ToolSelection{Name: "Harbor", Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Name: "Jenkins", Enabled: true},
			CDTool:     domain.ToolSelection{Name: "Argo CD", Enabled: true},
		},
	}
}

// 헤드리스 설치가 템플릿의 프로파일대로 깔리는지가 이 배선의 전부다.
// 이 코드가 없던 동안 API 설치는 golden_path_id 를 저장만 하고 프로파일은
// 읽지 않아, Lite 템플릿도 standard 크기로 설치됐다.
func TestCreateStack_PlansResourcesFromTemplateProfile(t *testing.T) {
	uc := NewCreateStack(
		stackrepo.NewMemoryStackRepository(),
		stackrepo.NewMemoryTemplateRepository(),
		WithResourcePlanning(planningResourceDefaults(t)),
	)

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:       "lite",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
		TemplateID: "gitea-jenkins-argocd-lite-v1",
		Config:     liteConfig(),
	})
	require.NoError(t, err)

	cfg, ok := out.Stack.Config.(domain.StackConfig)
	require.True(t, ok, "config 는 StackConfig 여야 한다")

	require.Len(t, cfg.AppliedResourceOverrides, 4)
	// 필드 이름을 적는다. 순서만으로 여섯 개를 맞추면 필드가 하나 끼어들 때
	// 조용히 다른 값을 검사하게 된다(go vet 이 지적하는 그것이다).
	assert.Equal(t, domain.ResourceVector{
		CPURequest: 0.5, CPULimit: 1,
		MemoryRequestGi: 1, MemoryLimitGi: 2,
		StorageRequestGi: 9, StorageLimitGi: 18,
	},
		cfg.AppliedResourceOverrides["artifacts.containerRegistry:harbor"],
		"Harbor 가 2코어/4Gi 그대로면 8Gi 노드에 들어가지 않는다")
	assert.Equal(t, domain.ResourceVector{
		CPURequest: 0.5, CPULimit: 0.5,
		MemoryRequestGi: 0.5, MemoryLimitGi: 1,
		StorageRequestGi: 3.5, StorageLimitGi: 7,
	},
		cfg.AppliedResourceOverrides["artifacts.sourceRepository:gitea"])
}

// 마법사가 계획한 값이 있으면 그것이 이긴다. 사용자가 화면에서 조정한 값을
// 서버가 덮어쓰면 계획 화면이 무의미해진다.
func TestCreateStack_KeepsCallerSuppliedPlan(t *testing.T) {
	uc := NewCreateStack(
		stackrepo.NewMemoryStackRepository(),
		stackrepo.NewMemoryTemplateRepository(),
		WithResourcePlanning(planningResourceDefaults(t)),
	)

	supplied := map[string]domain.ResourceVector{
		"artifacts.containerRegistry:harbor": {CPURequest: 7, CPULimit: 9},
	}
	cfg := liteConfig()
	cfg.AppliedResourceOverrides = supplied

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:       "lite-manual",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
		TemplateID: "gitea-jenkins-argocd-lite-v1",
		Config:     cfg,
	})
	require.NoError(t, err)

	stored, ok := out.Stack.Config.(domain.StackConfig)
	require.True(t, ok)
	assert.Equal(t, supplied, stored.AppliedResourceOverrides)
}

// 템플릿 없이 만든 스택은 계획할 근거가 없다. 임의로 standard 를 씌우면
// 관리자 기본값과 같은 값을 굳이 config 에 박아 두게 된다.
func TestCreateStack_NoTemplateLeavesPlanEmpty(t *testing.T) {
	uc := NewCreateStack(
		stackrepo.NewMemoryStackRepository(),
		stackrepo.NewMemoryTemplateRepository(),
		WithResourcePlanning(planningResourceDefaults(t)),
	)

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:      "no-template",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Config:    liteConfig(),
	})
	require.NoError(t, err)

	cfg, ok := out.Stack.Config.(domain.StackConfig)
	require.True(t, ok)
	assert.Empty(t, cfg.AppliedResourceOverrides)
}

// 계획은 부가 기능이다. 자원 기본값을 못 읽어도 스택 생성 자체는 막히지 않아야
// 한다 — 계획이 없으면 차트 기본값으로 깔릴 뿐이다.
func TestCreateStack_SucceedsWithoutPlanningDependency(t *testing.T) {
	uc := NewCreateStack(stackrepo.NewMemoryStackRepository(), stackrepo.NewMemoryTemplateRepository())

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:       "no-planner",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
		TemplateID: "gitea-jenkins-argocd-lite-v1",
		Config:     liteConfig(),
	})
	require.NoError(t, err)
	require.NotNil(t, out.Stack)
}

// 알 수 없는 템플릿 ID 도 생성은 통과해야 한다. 계획만 비운다.
func TestCreateStack_UnknownTemplateLeavesPlanEmpty(t *testing.T) {
	uc := NewCreateStack(
		stackrepo.NewMemoryStackRepository(),
		stackrepo.NewMemoryTemplateRepository(),
		WithResourcePlanning(planningResourceDefaults(t)),
	)

	out, err := uc.Execute(context.Background(), CreateStackInput{
		Name:       "unknown-template",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
		TemplateID: "does-not-exist-v1",
		Config:     liteConfig(),
	})
	require.NoError(t, err)

	cfg, ok := out.Stack.Config.(domain.StackConfig)
	require.True(t, ok)
	assert.Empty(t, cfg.AppliedResourceOverrides)
}
