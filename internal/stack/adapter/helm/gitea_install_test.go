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

// Gitea 의 DB 호스트는 실제 스택 네임스페이스를 가리켜야 한다.
//
// DefaultValues 는 네임스페이스를 모르므로 기본값을 쓸 수밖에 없는데, 그대로
// 설치하면 init 컨테이너가 nullus-postgresql.nullus.svc 를 찾다가
// "no such host" 로 CrashLoopBackOff 에 빠진다 — 실제로 그렇게 실패했다.
// GitLab 이 gitlabExternalSharedServiceValues 로 같은 문제를 푸는 것과 같은
// 방식으로 valuesForStep 에서 실제 네임스페이스를 채운다.
func TestGiteaSharedServiceValues_UsesActualNamespace(t *testing.T) {
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus-gjdemo")

	values := o.giteaSharedServiceValues()

	gitea, ok := values["gitea"].(map[string]any)
	require.True(t, ok)
	config, ok := gitea["config"].(map[string]any)
	require.True(t, ok)
	database, ok := config["database"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t,
		"nullus-postgresql.nullus-gjdemo.svc.cluster.local:5432",
		database["HOST"],
		"기본 네임스페이스를 그대로 두면 init 컨테이너가 DB 를 찾지 못해 CrashLoopBackOff 에 빠진다")
}

func TestGiteaSharedServiceValues_FallsBackWhenNamespaceEmpty(t *testing.T) {
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "")

	database := o.giteaSharedServiceValues()["gitea"].(map[string]any)["config"].(map[string]any)["database"].(map[string]any)
	assert.Contains(t, database["HOST"], defaultStackNamespace)
}

// Gitea 의 ROOT_URL 은 클론 주소의 출처다.
//
// 차트 기본값은 http://git.example.com 이라 그대로 두면 Argo CD 와 Jenkins 가
// 존재하지 않는 호스트를 클론하려 한다 — 리포는 만들어지는데 동기화와 빌드만
// 조용히 실패한다. GitLab 이 global.hosts.domain 으로 같은 문제를 푸는 것과
// 같이 접근 도메인을 쓴다.
func TestGiteaSharedServiceValues_SetsRootURLFromAccessDomain(t *testing.T) {
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus-gj3")
	cfg := domain.StackConfig{AccessDomain: "gj3.internal"}
	o.SetStackConfig(cfg)

	server := o.giteaSharedServiceValues()["gitea"].(map[string]any)["config"].(map[string]any)["server"].(map[string]any)

	assert.Equal(t, "http://gitea.gj3.internal/", server["ROOT_URL"])
	assert.Equal(t, "gitea.gj3.internal", server["DOMAIN"])
}

// 접근 도메인이 없으면 클러스터 내부 주소로 떨어뜨린다 — 최소한 in-cluster
// 소비자(Argo CD·Jenkins)는 클론할 수 있다.
func TestGiteaSharedServiceValues_FallsBackToInClusterRootURL(t *testing.T) {
	o := NewOrchestrator(nil, []byte("not-a-kubeconfig"), "nullus-gj3")

	server := o.giteaSharedServiceValues()["gitea"].(map[string]any)["config"].(map[string]any)["server"].(map[string]any)

	assert.Contains(t, server["ROOT_URL"], "gitea-http.nullus-gj3.svc:3000")
}
