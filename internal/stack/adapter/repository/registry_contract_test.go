package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/adapter/helm"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 화면이 안내하는 버전과 설치가 쓰는 버전은 같아야 한다.
//
// 호환성 매트릭스는 "검증된 조합"으로 화면에 노출되고, 실제 설치는 차트 스펙의
// 버전을 쓴다. 둘이 갈라지면 사용자는 1.14 를 봤는데 클러스터에는 1.19 가 서는
// 상황이 되고, 아무도 그 사실을 모른다. 실제로 Harbor 는 매트릭스가 1.14.0 을
// 말하는 동안 설치 경로가 아예 없었다.
func TestChartVersionsMatchCompatibilityMatrix(t *testing.T) {
	matrices, err := NewMemoryCompatibilityRepository().GetAll(context.Background())
	require.NoError(t, err)

	// 클러스터 안에 세우는 도구는 모두 대상이다. Harbor/Nexus 두 개만 걸어 두는
	// 동안 나머지는 조용히 갈라져 있었다 — 매트릭스는 Argo CD 6.8.0 을 말하는데
	// 설치는 7.7.16 을, Prometheus 는 67.0.0 인데 설치는 69.3.0 을 썼다.
	cases := []struct {
		matrixID string
		category string
		step     string
	}{
		{matrixID: "gitlab-harbor-v1", category: "container_registry", step: "installing_harbor"},
		{matrixID: "gitlab-nexus-v1", category: "container_registry", step: "installing_nexus"},
		{matrixID: "gitlab-allinone-v1", category: "source_repository", step: "installing_gitlab"},
		{matrixID: "gitlab-allinone-v1", category: "ci_platform", step: "installing_gitlab"},
		{matrixID: "gitlab-allinone-v1", category: "container_registry", step: "installing_gitlab"},
		{matrixID: "gitlab-allinone-v1", category: "storage_backend", step: "installing_minio"},
		{matrixID: "gitlab-allinone-v1", category: "cd_tool", step: "installing_argocd"},
		{matrixID: "gitlab-allinone-v1", category: "monitoring_collection", step: "installing_prometheus"},
		{matrixID: "gitlab-allinone-v1", category: "monitoring_visualization", step: "installing_grafana"},
		{matrixID: "gitlab-harbor-v1", category: "cd_tool", step: "installing_argocd"},
		{matrixID: "gitlab-nexus-v1", category: "cd_tool", step: "installing_argocd"},
	}

	for _, tc := range cases {
		t.Run(tc.step, func(t *testing.T) {
			var declared string
			for _, m := range matrices {
				if m.ID != tc.matrixID {
					continue
				}
				tool, ok := m.Tools[tc.category]
				require.Truef(t, ok, "matrix %s has no %s", tc.matrixID, tc.category)
				declared = tool.HelmVersion
			}
			require.NotEmptyf(t, declared, "matrix %s not found", tc.matrixID)

			spec, ok := helm.DefaultChartSpecForStep(tc.step)
			require.Truef(t, ok, "no chart spec for %s", tc.step)
			assert.Equalf(t, declared, spec.Version,
				"%s 설치 버전이 호환성 매트릭스 선언과 다르다", tc.step)
		})
	}
}

// 레지스트리 도구를 고른 템플릿에는 그것을 세우는 설치 단계가 있어야 한다.
//
// 템플릿이 약속만 하고 설치 경로가 없으면, 설치는 성공했는데 CI 가 이미지를
// 올릴 곳이 없는 상태가 된다 — Harbor 가 정확히 그 상태였다.
func TestRegistryToolsInTemplatesHaveInstallStep(t *testing.T) {
	templates, err := NewMemoryTemplateRepository().List(context.Background())
	require.NoError(t, err)

	// 클러스터 안에 세우는 도구만 대상이다. 외부 서비스는 설치 단계가 없다.
	stepForTool := map[string]string{
		"harbor": "installing_harbor",
		"nexus":  "installing_nexus",
	}

	for _, tmpl := range templates {
		for _, tool := range tmpl.Tools {
			if tool.Category != "container_registry" && tool.Category != "package_registry" {
				continue
			}
			step, ok := stepForTool[normalizeForTest(tool.Name)]
			if !ok {
				continue // GitLab Registry 는 GitLab 설치가 겸한다.
			}
			_, found := helm.DefaultChartSpecForStep(step)
			assert.Truef(t, found,
				"template %s uses %s but there is no %s chart spec", tmpl.ID, tool.Name, step)
		}
	}
}

func normalizeForTest(name string) string {
	switch name {
	case "Harbor":
		return "harbor"
	case "Nexus":
		return "nexus"
	}
	return name
}

// domain 상수가 매트릭스 상수의 출처여야 한다.
func TestDomainOwnsRegistryChartVersions(t *testing.T) {
	assert.Equal(t, domain.HarborChartVersion, baselineHarborHelmVersion)
	assert.Equal(t, domain.NexusChartVersion, baselineNexusHelmVersion)
}
