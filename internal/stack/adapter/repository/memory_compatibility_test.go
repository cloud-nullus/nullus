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
	assert.Len(t, matrices, 3)
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

	_, err := repo.Validate(context.Background(), map[string]string{
		"ci_platform": "Jenkins",
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

	t.Run("untested matrix carries beta tier and blocks Harbor on arm64", func(t *testing.T) {
		m, err := repo.GetByID(ctx, "github-argocd-v1")
		require.NoError(t, err)

		harbor, ok := m.Tools["container_registry"]
		require.True(t, ok)
		assert.Equal(t, "beta", harbor.Tier, "Harbor inherits beta tier from untested matrix")
		assert.Equal(t, []string{"amd64"}, harbor.ArchSupport)
		assert.False(t, harbor.SupportsArch("arm64"))

		github, ok := m.Tools["source_repository"]
		require.True(t, ok)
		assert.Equal(t, "beta", github.Tier)
		assert.True(t, github.SupportsArch("arm64"))
	})
}

// TestMemoryCompatibilityRepository_NarwhalBaselineVersions pins the
// Narwhal baseline v1 version numbers asserted by migration
// 000042_seed_narwhal_compat_refresh. If any of these values change,
// update the three layers simultaneously: this test, the SQL migration,
// and docs/20_아키텍처/Narwhal_호환성_Seed_Sources.md. A drift on a
// single layer produces divergent Pre-Deploy Gate verdicts between
// unit tests and real deployments.
func TestMemoryCompatibilityRepository_NarwhalBaselineVersions(t *testing.T) {
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
		{"storage_backend", "MinIO", "5.2.0", "RELEASE.2024-08-03T04-33-23Z"},
		{"cd_tool", "Argo CD", "6.8.0", "v2.8.3"},
		{"monitoring_collection", "Prometheus", "67.0.0", "v2.54.1"},
		{"monitoring_visualization", "Grafana", "8.5.0", "11.1.0"},
	}

	gitlabPins := append([]pin{
		{"source_repository", "GitLab CE", "9.5.1", "18.5.1"},
		{"ci_platform", "GitLab CI", "9.5.1", "18.5.1"},
		{"container_registry", "GitLab Registry", "9.5.1", "18.5.1"},
	}, shared...)

	githubPins := append([]pin{
		{"source_repository", "GitHub", "external", "external"},
		{"ci_platform", "GitHub Actions", "external", "external"},
		{"container_registry", "Harbor", "1.15.0", "2.11.0"},
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
