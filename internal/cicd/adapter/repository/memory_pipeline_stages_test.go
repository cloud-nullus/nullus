package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func TestMemoryPipelineRepository_PersistsStages(t *testing.T) {
	repo := NewMemoryPipelineRepository()
	p := &domain.Pipeline{ID: "pip_1", Name: "api", OrgID: "org", Stages: []string{"Build", "ImageScan", "Deploy"}}
	require.NoError(t, repo.Create(context.Background(), p))

	got, err := repo.GetByID(context.Background(), "pip_1")
	require.NoError(t, err)
	assert.Equal(t, []string{"Build", "ImageScan", "Deploy"}, got.Stages)
}

// 복제가 슬라이스를 공유하면 호출부가 돌려받은 값을 고치는 순간 저장된 기록이
// 함께 바뀐다. 실제 DB 저장소에서는 일어나지 않는 일이라 테스트가 거짓 초록이 된다.
func TestMemoryPipelineRepository_StagesAreNotShared(t *testing.T) {
	repo := NewMemoryPipelineRepository()
	p := &domain.Pipeline{ID: "pip_1", Name: "api", OrgID: "org", Stages: []string{"Build", "Deploy"}}
	require.NoError(t, repo.Create(context.Background(), p))

	p.Stages[0] = "변조"
	got, err := repo.GetByID(context.Background(), "pip_1")
	require.NoError(t, err)
	got.Stages[1] = "변조"

	again, err := repo.GetByID(context.Background(), "pip_1")
	require.NoError(t, err)
	assert.Equal(t, []string{"Build", "Deploy"}, again.Stages)
}
