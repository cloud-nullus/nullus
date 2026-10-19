//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func webSampleSource() *domain.TemplateSource {
	return &domain.TemplateSource{
		RepoURL:      "https://github.com/eGovFramework/egovframe-web-sample",
		Commit:       "158daebd2bfdab951e90ad526c3d2a6b6b42513b",
		Label:        "v5.0.0",
		BuildProfile: domain.BuildProfileMavenWAR,
	}
}

// 원본은 템플릿과 함께 저장되고, 수정으로 바뀌고, 지우면 nil 로 되읽힌다.
func TestPostgresCICDTemplateRepository_SourceRoundTrip(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresCICDTemplateRepository(pool)

	tmpl := &domain.PipelineTemplate{
		ID:      "egov-" + uuid.NewString(),
		Name:    "eGov Web",
		AppType: domain.AppTypeBackend,
		Stages:  []string{"Build", "Deploy"},
		Source:  webSampleSource(),
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	got, err := repo.GetByID(ctx, tmpl.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Source)
	assert.Equal(t, *webSampleSource(), *got.Source)

	tmpl.Source.Label = "v5.0.1"
	require.NoError(t, repo.Update(ctx, tmpl))
	got, err = repo.GetByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "v5.0.1", got.Source.Label)

	tmpl.Source = nil
	require.NoError(t, repo.Update(ctx, tmpl))
	got, err = repo.GetByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Source, "원본이 없는 템플릿은 nil 로 읽혀야 파이프라인 생성이 가져오기를 시도하지 않는다")
}

// 원본이 없는 기존 시드는 nil 로 읽힌다 — 빈 원본을 가진 템플릿으로 보이면 안 된다.
func TestPostgresCICDTemplateRepository_SeedWithoutSourceReadsNil(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	repo := NewPostgresCICDTemplateRepository(pool)

	got, err := repo.GetByID(context.Background(), "nullus-sample-backend-v1")
	require.NoError(t, err)
	assert.Nil(t, got.Source)
}

// 전자정부 표준프레임워크 샘플 두 개가 시드된다(nullus-plan#91).
func TestPostgresCICDTemplateRepository_SeedsEgovFrameTemplates(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	repo := NewPostgresCICDTemplateRepository(pool)
	ctx := context.Background()

	cases := map[string]domain.TemplateSource{
		"egovframe-boot-sample-v1": {
			RepoURL:      "https://github.com/eGovFramework/egovframe-boot-sample-java-config",
			Commit:       "c44e4bc8ea1e1b1e83e0aaab1037fbf04c761feb",
			Label:        "v5.0.0",
			BuildProfile: domain.BuildProfileSpringBoot,
		},
		"egovframe-web-sample-v1": *webSampleSource(),
	}
	for id, want := range cases {
		got, err := repo.GetByID(ctx, id)
		require.NoError(t, err, id)
		require.NoError(t, got.Validate(), "시드는 도메인 검증을 통과해야 한다: %s", id)
		require.NotNil(t, got.Source, id)
		assert.Equal(t, want, *got.Source, id)
		assert.Equal(t, domain.AppTypeBackend, got.AppType, id)
		assert.Equal(t, []string{"Build", "Deploy"}, got.Stages, "선택 단계는 파이프라인이 정한다: %s", id)
	}
}

// 파이프라인은 처음 가져온 원본을 기록한다. 원본이 없는 파이프라인은 nil 로 읽힌다.
func TestPostgresPipelineRepository_SourceRoundTrip(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresPipelineRepository(pool)
	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)

	withSource := &domain.Pipeline{
		ID: "pipeline-" + uuid.NewString(), Name: "egov-web", TemplateID: "egovframe-web-sample-v1",
		OrgID: orgID, ClusterID: clusterID, Namespace: "egov", AppType: domain.AppTypeBackend,
		GitRepoURL: "http://gitlab/nullus/egov-web.git", Status: domain.PipelineStatusActive,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond), Source: webSampleSource(),
	}
	require.NoError(t, repo.Create(ctx, withSource))
	got, err := repo.GetByID(ctx, withSource.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Source)
	assert.Equal(t, *webSampleSource(), *got.Source)

	list, err := repo.List(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].Source, "목록도 같은 열을 읽는다")

	// 수정이 원본을 지우지 않는다.
	got.Name = "egov-web-renamed"
	require.NoError(t, repo.Update(ctx, got))
	got, err = repo.GetByID(ctx, withSource.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Source)

	plain := &domain.Pipeline{
		ID: "pipeline-" + uuid.NewString(), Name: "plain", TemplateID: "web-backend-v1",
		OrgID: orgID, ClusterID: clusterID, Namespace: "default", AppType: domain.AppTypeBackend,
		Status: domain.PipelineStatusActive, CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	require.NoError(t, repo.Create(ctx, plain))
	got, err = repo.GetByID(ctx, plain.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Source)
}
