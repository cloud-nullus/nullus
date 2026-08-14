package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 소스 저장소 슬롯의 파드 접두사는 고른 제품에 따라 달라야 한다.
//
// 과거에는 "gitlab" 이 하드코딩돼 있어, Gitea 를 고르면 설치는 정상인데 화면만
// gitlab-* 파드를 찾다가 "0 파드 warning" 으로 남았다 — 로그 저장소 슬롯이 같은
// 이유로 먼저 동적화됐다(logStoreNamePrefixes).
func TestInstalledToolWorkloads_Gitea_UsesGiteaPrefix(t *testing.T) {
	cfg := StackConfig{}
	cfg.Artifacts.SourceRepository = ToolSelection{Enabled: true, Name: "Gitea", Version: "1.27.0"}

	workloads := InstalledToolWorkloads(cfg)

	found := findWorkload(t, workloads, "source_repository")
	assert.Equal(t, "Gitea", found.Name)
	assert.Equal(t, []string{GiteaReleaseName}, found.NamePrefixes,
		"Gitea 를 골랐는데 gitlab-* 파드를 찾으면 화면에서만 죽은 것처럼 보인다")
}

// 기존 GitLab 경로는 접두사 동적화 이후에도 그대로여야 한다.
func TestInstalledToolWorkloads_GitLab_KeepsGitLabPrefix(t *testing.T) {
	cfg := StackConfig{}
	cfg.Artifacts.SourceRepository = ToolSelection{Enabled: true, Name: "GitLab CE", Version: "v17.7.0"}

	workloads := InstalledToolWorkloads(cfg)

	found := findWorkload(t, workloads, "source_repository")
	assert.Equal(t, []string{"gitlab"}, found.NamePrefixes)
}

// 외부 SCM(GitHub 등)은 클러스터에 파드가 없다. 항목을 만들면 영구히
// "0 파드 warning" 으로 남으므로 목록에서 빠져야 한다 — 레지스트리 슬롯이
// version=external 을 빼는 것과 같은 이유다.
func TestInstalledToolWorkloads_ExternalSourceRepository_Excluded(t *testing.T) {
	cfg := StackConfig{}
	cfg.Artifacts.SourceRepository = ToolSelection{Enabled: true, Name: "GitHub", Version: "external"}

	workloads := InstalledToolWorkloads(cfg)

	for _, w := range workloads {
		assert.NotEqual(t, "source_repository", w.Key,
			"클러스터 밖 SCM 은 설치 워크로드가 아니다")
	}
}

func findWorkload(t *testing.T, workloads []ToolWorkload, key string) ToolWorkload {
	t.Helper()
	for _, w := range workloads {
		if w.Key == key {
			return w
		}
	}
	require.Failf(t, "워크로드 없음", "key=%q 가 InstalledToolWorkloads 결과에 없다", key)
	return ToolWorkload{}
}
