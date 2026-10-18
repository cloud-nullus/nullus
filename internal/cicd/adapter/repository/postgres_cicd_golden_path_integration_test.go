//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 골든패스는 프로세스 메모리에 박혀 있었다. 만들고 고치고 지우는 API 가 성공을 돌려주면서도
// 재시작하면 세 개의 기본값으로 되돌아갔다. DB 로 옮긴 뒤 왕복이 남는지 본다.
func TestPostgresCICDGoldenPathRepository_CreateUpdateDelete(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresCICDGoldenPathRepository(pool)

	gp := &domain.CICDGoldenPath{
		ID:                   "team-gitops-" + uuid.NewString(),
		Name:                 "Team GitOps",
		Description:          "팀 표준 GitOps 구성",
		EstimatedInstallTime: 75,
		RecommendedUseCase:   "GitOps 중심 조직",
		MinResources:         "8 vCPU / 16Gi RAM / 100Gi Storage",
		Tools: []domain.CICDTool{
			{Category: "source_repository", Name: "GitLab CE", HelmVersion: "8.7.2", AppVersion: "17.7.2"},
			{Category: "cd_tool", Name: "Argo CD", HelmVersion: "7.7.2", AppVersion: "2.13.2"},
		},
	}
	require.NoError(t, repo.Create(ctx, gp))

	// 만든 그대로 되읽힌다 — 도구 목록과 권장 용도까지.
	got, err := repo.GetByID(ctx, gp.ID)
	require.NoError(t, err)
	assert.Equal(t, gp.Name, got.Name)
	assert.Equal(t, gp.Description, got.Description)
	assert.Equal(t, gp.EstimatedInstallTime, got.EstimatedInstallTime)
	assert.Equal(t, gp.RecommendedUseCase, got.RecommendedUseCase)
	assert.Equal(t, gp.MinResources, got.MinResources)
	assert.Equal(t, gp.Tools, got.Tools)
	assert.False(t, got.CreatedAt.IsZero(), "created_at 이 채워져야 목록에서 순서를 가릴 수 있다")

	// 도구를 하나 줄여 저장하면 줄어든 채로 남는다 — 덮어쓰기지 합치기가 아니다.
	gp.Name = "Team GitOps v2"
	gp.Tools = gp.Tools[:1]
	require.NoError(t, repo.Update(ctx, gp))

	got, err = repo.GetByID(ctx, gp.ID)
	require.NoError(t, err)
	assert.Equal(t, "Team GitOps v2", got.Name)
	assert.Len(t, got.Tools, 1)

	require.NoError(t, repo.Delete(ctx, gp.ID))

	_, err = repo.GetByID(ctx, gp.ID)
	assert.ErrorIs(t, err, domain.ErrGoldenPathNotFound)
}

// 마이그레이션이 종전 하드코딩 세 건을 그대로 넣어 둔다. 넣지 않으면 DB 로 옮기는 순간
// 화면이 빈 목록이 된다.
func TestPostgresCICDGoldenPathRepository_SeedMatchesFormerHardcodedSet(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresCICDGoldenPathRepository(pool)

	paths, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, paths, 3)

	// 목록 순서가 고정이다. 맵을 돌던 종전 구현은 요청마다 카드 순서가 바뀌었다.
	ids := []string{paths[0].ID, paths[1].ID, paths[2].ID}
	assert.Equal(t, []string{"gitlab-allinone-v1", "gitlab-argocd-v1", "github-argocd-v1"}, ids)

	byID := map[string]*domain.CICDGoldenPath{}
	for _, p := range paths {
		byID[p.ID] = p
	}

	allInOne := byID["gitlab-allinone-v1"]
	assert.Equal(t, "GitLab All-in-One", allInOne.Name)
	assert.Equal(t, 90, allInOne.EstimatedInstallTime)
	assert.Len(t, allInOne.Tools, 8)

	// GitHub 구성은 외부 서비스를 쓴다 — helm 버전이 "external" 로 들어가야
	// 설치 화면이 클러스터에 깔 대상으로 오해하지 않는다.
	github := byID["github-argocd-v1"]
	assert.Equal(t, 45, github.EstimatedInstallTime)
	var registry *domain.CICDTool
	for i := range github.Tools {
		if github.Tools[i].Category == "container_registry" {
			registry = &github.Tools[i]
		}
	}
	require.NotNil(t, registry)
	assert.Equal(t, "GHCR", registry.Name)
	assert.Equal(t, "external", registry.HelmVersion)
}

// 같은 ID 를 두 번 만들면 조용히 덮어쓰지 않는다. 종전 메모리 구현은 덮어썼다.
func TestPostgresCICDGoldenPathRepository_CreateRejectsDuplicate(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresCICDGoldenPathRepository(pool)

	gp := &domain.CICDGoldenPath{ID: "gitlab-allinone-v1", Name: "덮어쓰기 시도"}
	err := repo.Create(ctx, gp)
	assert.ErrorIs(t, err, domain.ErrGoldenPathAlreadyExists)

	// 원래 행은 그대로다.
	got, err := repo.GetByID(ctx, "gitlab-allinone-v1")
	require.NoError(t, err)
	assert.Equal(t, "GitLab All-in-One", got.Name)
}

// 없는 ID 를 고치거나 지우면 성공으로 응답하지 않는다.
func TestPostgresCICDGoldenPathRepository_MissingRowIsNotFound(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresCICDGoldenPathRepository(pool)

	missing := &domain.CICDGoldenPath{ID: "no-such-path", Name: "없음"}
	assert.ErrorIs(t, repo.Update(ctx, missing), domain.ErrGoldenPathNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, "no-such-path"), domain.ErrGoldenPathNotFound)
}
