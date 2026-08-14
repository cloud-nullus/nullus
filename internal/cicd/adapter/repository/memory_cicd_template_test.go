package repository

import (
	"context"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/scaffold"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func TestMemoryCICDTemplateRepository_ListReturnsCanonicalTemplates(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	templates, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, templates, 2, "should have exactly 2 CI/CD pipeline templates")
}

func TestMemoryCICDTemplateRepository_GetByID_WebBackend(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "web-backend-v1")
	require.NoError(t, err)

	assert.Equal(t, "web-backend-v1", tmpl.ID)
	assert.Equal(t, "User Custom Pipeline", tmpl.Name)
	// 단계 목록의 출처는 스캐폴딩이다 — 손으로 적으면 실제 파이프라인과 어긋난다.
	assert.Equal(t, scaffold.PipelineStageNames(), tmpl.Stages)
}

func TestMemoryCICDTemplateRepository_DoesNotReturnRemovedWebFrontend(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	_, err := repo.GetByID(context.Background(), "web-frontend-v1")
	require.Error(t, err)
}

func TestMemoryCICDTemplateRepository_GetByID_BatchJob(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "batch-job-v1")
	require.NoError(t, err)

	assert.Equal(t, "batch-job-v1", tmpl.ID)
	assert.Equal(t, "Batch Job Pipeline", tmpl.Name)
	// 스캐폴딩은 app_type 과 무관하게 같은 파이프라인을 만든다.
	// CronJob 배포는 아직 구현돼 있지 않아 선언만 남기면 없는 기능을 약속한다.
	assert.Equal(t, scaffold.PipelineStageNames(), tmpl.Stages)
}

func TestMemoryCICDTemplateRepository_GetByID_NotFound(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	_, err := repo.GetByID(context.Background(), "nonexistent-template")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMemoryCICDTemplateRepository_ListContainsCanonicalTemplateIDs(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	templates, err := repo.List(context.Background())
	require.NoError(t, err)

	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		ids = append(ids, tmpl.ID)
	}

	assert.ElementsMatch(t, []string{
		"web-backend-v1",
		"batch-job-v1",
	}, ids)
}

func TestMemoryCICDTemplateRepository_GetByID_ReturnsExpectedAppType(t *testing.T) {
	repo := NewMemoryCICDTemplateRepository()

	tmpl, err := repo.GetByID(context.Background(), "web-backend-v1")
	require.NoError(t, err)

	assert.Equal(t, domain.AppTypeBackend, tmpl.AppType)
}
