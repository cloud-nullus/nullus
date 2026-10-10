//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 화면의 "템플릿 생성·수정·삭제"가 DB 에 남는지 본다. 이 세 메서드가 TODO 로 성공만 돌려주던
// 동안에는 버튼이 성공으로 응답하고도 목록에 아무것도 바뀌지 않았다.
func TestPostgresCICDTemplateRepository_CreateUpdateDelete(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresCICDTemplateRepository(pool)

	tmpl := &domain.PipelineTemplate{
		ID:             "team-backend-" + uuid.NewString(),
		Name:           "Team Backend",
		Description:    "팀 표준 백엔드 파이프라인",
		AppType:        domain.AppTypeBackend,
		Stages:         []string{"Build", "SAST", "Deploy"},
		GitRepoURL:     "https://gitlab.example.com/team/backend",
		DockerfilePath: "build/Dockerfile",
		DockerContext:  "./",
		EnvVars:        map[string]string{"PORT": "8080"},
		CreatedBy:      "alice",
	}
	require.NoError(t, repo.Create(ctx, tmpl))

	// 만든 그대로 되읽힌다 — 빌드 설정과 만든 사람까지.
	got, err := repo.GetByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, tmpl.Name, got.Name)
	assert.Equal(t, tmpl.Description, got.Description)
	assert.Equal(t, domain.AppTypeBackend, got.AppType)
	assert.Equal(t, tmpl.Stages, got.Stages)
	assert.Equal(t, tmpl.GitRepoURL, got.GitRepoURL)
	assert.Equal(t, tmpl.DockerfilePath, got.DockerfilePath)
	assert.Equal(t, tmpl.DockerContext, got.DockerContext)
	assert.Equal(t, tmpl.EnvVars, got.EnvVars)
	assert.Equal(t, "alice", got.CreatedBy)

	// 목록에도 보인다.
	list, err := repo.List(ctx)
	require.NoError(t, err)
	ids := make([]string, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.ID)
	}
	assert.Contains(t, ids, tmpl.ID)

	// 같은 ID 로 다시 만들면 덮어쓰지 않고 거부한다.
	dup := *tmpl
	dup.Name = "Overwrite Attempt"
	err = repo.Create(ctx, &dup)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrTemplateAlreadyExists), "got %v", err)
	got, err = repo.GetByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Team Backend", got.Name)

	// 수정은 모든 열을 바꾼다. 빈 env_vars 는 빈 객체로 남는다(NULL 이 아니다).
	got.Name = "Team Backend v2"
	got.Description = ""
	got.AppType = domain.AppTypeWeb
	got.Stages = []string{"Build", "Deploy"}
	got.GitRepoURL = ""
	got.EnvVars = nil
	got.CreatedBy = "alice"
	require.NoError(t, repo.Update(ctx, got))

	updated, err := repo.GetByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Team Backend v2", updated.Name)
	assert.Equal(t, "", updated.Description)
	assert.Equal(t, domain.AppTypeWeb, updated.AppType)
	assert.Equal(t, []string{"Build", "Deploy"}, updated.Stages)
	assert.Equal(t, "", updated.GitRepoURL)
	assert.Equal(t, "build/Dockerfile", updated.DockerfilePath)
	assert.Empty(t, updated.EnvVars)
	assert.Equal(t, "alice", updated.CreatedBy)

	// 없는 템플릿의 수정·삭제는 not found 다.
	missing := &domain.PipelineTemplate{ID: "missing-" + uuid.NewString(), Name: "x", AppType: domain.AppTypeWeb, Stages: []string{"Build"}}
	err = repo.Update(ctx, missing)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrTemplateNotFound), "got %v", err)
	err = repo.Delete(ctx, missing.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrTemplateNotFound), "got %v", err)

	// 삭제하면 사라지고, 다시 지우면 not found 다.
	require.NoError(t, repo.Delete(ctx, tmpl.ID))
	_, err = repo.GetByID(ctx, tmpl.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrTemplateNotFound), "got %v", err)
	err = repo.Delete(ctx, tmpl.ID)
	assert.True(t, errors.Is(err, domain.ErrTemplateNotFound), "got %v", err)
}

// 시드 템플릿은 created_by 가 없다 — NULL 열이 빈 문자열로 읽혀야 목록이 깨지지 않는다.
func TestPostgresCICDTemplateRepository_SeededTemplateHasNoCreator(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	repo := NewPostgresCICDTemplateRepository(pool)

	got, err := repo.GetByID(context.Background(), "web-backend-v1")
	require.NoError(t, err)
	assert.Equal(t, "", got.CreatedBy)
}
