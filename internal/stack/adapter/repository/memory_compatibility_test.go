package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

func TestMemoryCompatibilityRepository_GetAll(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	matrices, err := repo.GetAll(context.Background())
	require.NoError(t, err)

	ids := make([]string, 0, len(matrices))
	for _, m := range matrices {
		ids = append(ids, m.ID)
	}
	assert.Subset(t, ids, []string{
		"gitlab-allinone-v1",
		"gitlab-argocd-v1",
		"gitlab-harbor-v1",
		"gitlab-nexus-v1",
		"github-argocd-v1",
	})
}

func TestMemoryCompatibilityRepository_GetByID(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	m, err := repo.GetByID(context.Background(), "gitlab-allinone-v1")
	require.NoError(t, err)
	assert.Equal(t, "GitLab All-in-One", m.Name)
	assert.Equal(t, "verified", m.Status)
}

func TestMemoryCompatibilityRepository_GetByID_NotFound(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	_, err := repo.GetByID(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMemoryCompatibilityRepository_Validate_Match(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	m, err := repo.Validate(context.Background(), map[string]string{
		"source_repository": "GitLab CE",
		"cd_tool":           "Argo CD",
	})
	require.NoError(t, err)
	assert.NotNil(t, m)
}

func TestMemoryCompatibilityRepository_Validate_NoMatch(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	// Jenkins 는 이제 검증된 조합(gitea-jenkins-argocd-v1)에 들어 있으므로
	// "지원하지 않는 도구" 예시로 쓸 수 없다. 실제로 어느 매트릭스에도 없는
	// 조합을 쓴다.
	_, err := repo.Validate(context.Background(), map[string]string{
		"ci_platform": "Spinnaker",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no compatible matrix found")
}

func TestMemoryCompatibilityRepository_KubernetesCompat(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()

	m, err := repo.GetByID(context.Background(), "github-argocd-v1")
	require.NoError(t, err)
	assert.Equal(t, "1.27", m.Kubernetes.Min)
	assert.Equal(t, "1.35", m.Kubernetes.Max)
	assert.Equal(t, "1.35", m.Kubernetes.Recommended)
}

// TestMemoryCompatibilityRepository_ToolV2Fields pins the per-tool v2
// metadata (MinK8sVersion / ArchSupport / Tier) introduced in migration
// 000041. If this drifts, the memory repo and Postgres seed will
// disagree and the Pre-Deploy Gate will produce inconsistent verdicts
// between test environments.
func TestMemoryCompatibilityRepository_ToolV2Fields(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()
	ctx := context.Background()

	t.Run("verified matrix has stable tier and amd64-only GitLab", func(t *testing.T) {
		m, err := repo.GetByID(ctx, "gitlab-allinone-v1")
		require.NoError(t, err)

		gitlab, ok := m.Tools["source_repository"]
		require.True(t, ok)
		assert.Equal(t, "1.27", gitlab.MinK8sVersion)
		assert.Equal(t, []string{"amd64"}, gitlab.ArchSupport)
		assert.Equal(t, "stable", gitlab.Tier)
		assert.False(t, gitlab.SupportsArch("arm64"), "GitLab CE does not ship arm64 images")

		argocd, ok := m.Tools["cd_tool"]
		require.True(t, ok)
		assert.Equal(t, []string{"amd64", "arm64"}, argocd.ArchSupport)
		assert.True(t, argocd.SupportsArch("arm64"))
	})

	t.Run("external-only matrix is verified and imposes no arch limit", func(t *testing.T) {
		m, err := repo.GetByID(ctx, "github-argocd-v1")
		require.NoError(t, err)
		assert.Equal(t, "verified", m.Status)

		// 레지스트리가 GHCR 로 바뀌면서 이 조합에는 amd64 전용 도구가 없다.
		// Harbor 의 amd64 제약을 남겨두면 arm64 클러스터에서 호환성 검사가
		// 설치할 수도 없는 도구를 이유로 잘못 막는다. verified 행렬에서는
		// 아키텍처 불일치가 fail 이므로 이 제약이 남으면 배포가 통째로 막힌다.
		ghcr, ok := m.Tools["container_registry"]
		require.True(t, ok)
		assert.Equal(t, "GHCR", ghcr.Name)
		assert.Equal(t, "external", ghcr.HelmVersion, "GHCR 은 클러스터에 설치되지 않는다")
		assert.Equal(t, "stable", ghcr.Tier, "verified 행렬의 도구는 stable 티어를 물려받는다")
		assert.Equal(t, []string{"amd64", "arm64"}, ghcr.ArchSupport)
		assert.True(t, ghcr.SupportsArch("arm64"))

		github, ok := m.Tools["source_repository"]
		require.True(t, ok)
		assert.Equal(t, "stable", github.Tier)
		assert.True(t, github.SupportsArch("arm64"))
	})
}

// 세 Golden Path 매트릭스가 같은 기준선을 공유하는지 본다.
//
// 값을 리터럴로 다시 적지 않고 domain 상수를 참조한다. 예전에는 여기에 숫자를
// 박아 뒀는데, 그래서 "설치 경로 / 매트릭스 / 이 테스트" 셋이 각자 출처가 되어
// 조용히 갈라졌다 — 매트릭스가 Argo CD 6.8.0 을 말하는 동안 설치는 7.7.16 을
// 썼고 이 테스트는 6.8.0 을 지키고 있었다.
//
// 이 테스트가 지키는 것은 "숫자가 무엇인가" 가 아니라 "세 매트릭스가 같은 값을
// 쓰는가" 다. 숫자 자체는 TestChartVersionsMatchCompatibilityMatrix 가 실제 차트
// 스펙과 맞춰 지킨다.
func TestMemoryCompatibilityRepository_BaselineVersionsAreShared(t *testing.T) {
	repo := NewMemoryCompatibilityRepository()
	ctx := context.Background()

	type pin struct {
		category    string
		toolName    string
		helmVersion string
		appVersion  string
	}

	// Baseline shared by all three Golden Path matrices for tools
	// installed inside the cluster. GitHub / GitHub Actions are
	// external SaaS and covered separately below.
	shared := []pin{
		{"storage_backend", "MinIO", domain.MinIOChartVersion, domain.MinIOAppVersion},
		{"cd_tool", "Argo CD", domain.ArgoCDChartVersion, domain.ArgoCDAppVersion},
		{"monitoring_collection", "Prometheus", domain.PrometheusChartVersion, domain.PrometheusAppVersion},
		{"monitoring_visualization", "Grafana", domain.GrafanaChartVersion, domain.GrafanaAppVersion},
	}

	gitlabPins := append([]pin{
		{"source_repository", "GitLab CE", domain.GitLabChartVersion, domain.GitLabAppVersion},
		{"ci_platform", "GitLab CI", domain.GitLabChartVersion, domain.GitLabAppVersion},
		{"container_registry", "GitLab Registry", domain.GitLabChartVersion, domain.GitLabAppVersion},
	}, shared...)

	githubPins := append([]pin{
		{"source_repository", "GitHub", "external", "external"},
		{"ci_platform", "GitHub Actions", "external", "external"},
		// GitHub 호스티드 러너는 클러스터 내부 Harbor 에 닿을 수 없어
		// 이 조합의 레지스트리는 GHCR 이다.
		{"container_registry", "GHCR", "external", "external"},
	}, shared...)

	cases := []struct {
		matrixID string
		pins     []pin
	}{
		{"gitlab-allinone-v1", gitlabPins},
		{"gitlab-argocd-v1", gitlabPins},
		{"github-argocd-v1", githubPins},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.matrixID, func(t *testing.T) {
			m, err := repo.GetByID(ctx, tc.matrixID)
			require.NoError(t, err)

			for _, p := range tc.pins {
				tv, ok := m.Tools[p.category]
				require.True(t, ok, "%s: missing category %q", tc.matrixID, p.category)
				assert.Equal(t, p.toolName, tv.Name, "%s.%s tool name drift", tc.matrixID, p.category)
				assert.Equal(t, p.helmVersion, tv.HelmVersion, "%s.%s helm version drift", tc.matrixID, p.category)
				assert.Equal(t, p.appVersion, tv.AppVersion, "%s.%s app version drift", tc.matrixID, p.category)
			}
		})
	}
}

// --- F8-Phase5 (CRUD) ---------------------------------------------------

func sampleFixtureMatrix(id string) *domain.CompatibilityMatrix {
	return &domain.CompatibilityMatrix{
		ID:     id,
		Name:   "Fixture " + id,
		Status: "untested",
		Kubernetes: domain.KubernetesCompat{
			Min: "1.27", Max: "1.35", Recommended: "1.35",
		},
		Tools: map[string]domain.ToolVersion{
			"db": {
				Name:        "Postgres",
				HelmVersion: "12.0.0",
				AppVersion:  "16.0",
				Tier:        "stable",
				ArchSupport: []string{"amd64", "arm64"},
			},
		},
	}
}

func TestMemoryCompatibilityRepository_Create_RoundTrip(t *testing.T) {
	r := NewMemoryCompatibilityRepository()
	ctx := context.Background()

	require.NoError(t, r.Create(ctx, sampleFixtureMatrix("fixture-v1")))
	got, err := r.GetByID(ctx, "fixture-v1")
	require.NoError(t, err)
	assert.Equal(t, "Fixture fixture-v1", got.Name)
	assert.Equal(t, "untested", got.Status)
}

func TestMemoryCompatibilityRepository_Create_DuplicateRejected(t *testing.T) {
	r := NewMemoryCompatibilityRepository()
	ctx := context.Background()
	require.NoError(t, r.Create(ctx, sampleFixtureMatrix("dup")))
	err := r.Create(ctx, sampleFixtureMatrix("dup"))
	assert.True(t, errors.Is(err, port.ErrCompatibilityMatrixExists))
}

func TestMemoryCompatibilityRepository_Update_Success(t *testing.T) {
	r := NewMemoryCompatibilityRepository()
	ctx := context.Background()
	m := sampleFixtureMatrix("upd")
	require.NoError(t, r.Create(ctx, m))
	m.Name = "renamed"
	m.Status = "verified"
	require.NoError(t, r.Update(ctx, m))
	got, _ := r.GetByID(ctx, "upd")
	assert.Equal(t, "renamed", got.Name)
	assert.Equal(t, "verified", got.Status)
}

func TestMemoryCompatibilityRepository_Update_NotFound(t *testing.T) {
	r := NewMemoryCompatibilityRepository()
	err := r.Update(context.Background(), sampleFixtureMatrix("missing"))
	assert.True(t, errors.Is(err, port.ErrCompatibilityMatrixNotFound))
}

func TestMemoryCompatibilityRepository_Delete_Idempotent(t *testing.T) {
	r := NewMemoryCompatibilityRepository()
	ctx := context.Background()
	require.NoError(t, r.Create(ctx, sampleFixtureMatrix("del")))
	require.NoError(t, r.Delete(ctx, "del"))
	_, err := r.GetByID(ctx, "del")
	require.Error(t, err)
	// Idempotent: second delete + unknown id → no error.
	require.NoError(t, r.Delete(ctx, "del"))
	require.NoError(t, r.Delete(ctx, "never-existed"))
}

func TestMemoryCompatibilityRepository_Create_IDRequired(t *testing.T) {
	r := NewMemoryCompatibilityRepository()
	err := r.Create(context.Background(), &domain.CompatibilityMatrix{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id is required")
}
