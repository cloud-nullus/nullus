package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 설치가 실제로 쓰는 차트가 화면이 안내하는 버전과 같아야 한다.
// 갈라지면 "안내된 버전"과 "설치된 버전"이 달라진다.
func TestDefaultChartSpecForStep_Gitea(t *testing.T) {
	spec, ok := DefaultChartSpecForStep("installing_gitea")

	require.True(t, ok, "installing_gitea 에 차트 스펙이 없으면 스텝이 unknown step 으로 죽는다")
	assert.Equal(t, domain.GiteaReleaseName, spec.ReleaseName,
		"릴리스명이 곧 파드 접두사이자 Service 이름의 뿌리다")
	assert.Equal(t, "gitea", spec.ChartName)
	assert.Equal(t, "https://dl.gitea.com/charts", spec.RepoURL)
	assert.Equal(t, domain.GiteaChartVersion, spec.Version)
}

// Gitea 를 고른 스택에서만 스텝이 선다.
func TestIsGiteaSourceRepositorySelection(t *testing.T) {
	tests := []struct {
		name string
		sel  domain.ToolSelection
		want bool
	}{
		{"Gitea 선택", domain.ToolSelection{Enabled: true, Name: "Gitea"}, true},
		{"대소문자 무관", domain.ToolSelection{Enabled: true, Name: "gitea"}, true},
		{"꺼져 있으면 제외", domain.ToolSelection{Enabled: false, Name: "Gitea"}, false},
		{"외부 Gitea 는 설치하지 않는다", domain.ToolSelection{Enabled: true, Name: "Gitea", Version: "external"}, false},
		{"GitLab 은 Gitea 가 아니다", domain.ToolSelection{Enabled: true, Name: "GitLab CE"}, false},
		{"이름이 비면 기본값 GitLab 이라 Gitea 아님", domain.ToolSelection{Enabled: true}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isGiteaSourceRepositorySelection(tc.sel))
		})
	}
}

// Gitea 를 골랐다면 GitLab 스텝은 서면 안 된다 — 둘 다 서면 같은 슬롯에
// 두 제품이 올라가고 리소스도 두 배로 든다.
func TestGiteaAndGitLabStepsAreMutuallyExclusive(t *testing.T) {
	cfg := domain.StackConfig{}
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Enabled: true, Name: "Gitea"}

	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
	o.SetStackConfig(cfg)

	assert.True(t, o.IsStepEnabled("installing_gitea"))
	assert.False(t, o.IsStepEnabled("installing_gitlab"))
}

func TestGitLabSelection_DoesNotEnableGiteaStep(t *testing.T) {
	cfg := domain.StackConfig{}
	cfg.Artifacts.SourceRepository = domain.ToolSelection{Enabled: true, Name: "GitLab CE"}

	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus")
	o.SetStackConfig(cfg)

	assert.True(t, o.IsStepEnabled("installing_gitlab"))
	assert.False(t, o.IsStepEnabled("installing_gitea"))
}

// Gitea 는 스택 PostgreSQL 을 쓴다. 차트 기본값인 postgresql-ha 를 그대로 두면
// 스택 안에 두 번째 데이터베이스가 올라간다.
func TestGiteaValues_UsesStackPostgresAndDisablesBundledSubcharts(t *testing.T) {
	values := DefaultValues("installing_gitea")
	require.NotEmpty(t, values, "Gitea values 가 비면 차트 기본값(내장 DB·valkey)이 그대로 선다")

	for _, sub := range []string{"postgresql-ha", "postgresql", "valkey-cluster", "valkey"} {
		block, ok := values[sub].(map[string]any)
		require.Truef(t, ok, "%s 블록이 없다", sub)
		assert.Equalf(t, false, block["enabled"], "%s 는 꺼져야 한다", sub)
	}

	gitea, ok := values["gitea"].(map[string]any)
	require.True(t, ok)
	admin, ok := gitea["admin"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, domain.GiteaAdminSecret, admin["existingSecret"],
		"admin 비밀번호를 values 에 평문으로 두면 OpenBao 단일 출처 원칙이 깨진다")
}
