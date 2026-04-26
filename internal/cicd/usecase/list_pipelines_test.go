package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

type mockListPipelinesRepo struct {
	listResp []*domain.Pipeline
	listErr  error
	listOrg  string
}

func (m *mockListPipelinesRepo) Create(_ context.Context, _ *domain.Pipeline) error { return nil }
func (m *mockListPipelinesRepo) GetByID(_ context.Context, _ string) (*domain.Pipeline, error) {
	return nil, nil
}
func (m *mockListPipelinesRepo) List(_ context.Context, orgID string, _ ...string) ([]*domain.Pipeline, error) {
	m.listOrg = orgID
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*domain.Pipeline, 0, len(m.listResp))
	for _, pipeline := range m.listResp {
		copied := *pipeline
		result = append(result, &copied)
	}
	return result, nil
}
func (m *mockListPipelinesRepo) ListByStackID(_ context.Context, _ string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (m *mockListPipelinesRepo) Update(_ context.Context, _ *domain.Pipeline) error { return nil }
func (m *mockListPipelinesRepo) Delete(_ context.Context, _ string) error           { return nil }

func TestListPipelines_Success(t *testing.T) {
	repo := &mockListPipelinesRepo{listResp: []*domain.Pipeline{
		{ID: "pip-1", Name: "orders", OrgID: "org-1"},
		{ID: "pip-2", Name: "payments", OrgID: "org-1"},
	}}
	uc := NewListPipelines(repo)

	out, err := uc.Execute(context.Background(), ListPipelinesInput{OrgID: "org-1"})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "org-1", repo.listOrg)
	require.Len(t, out.Pipelines, 2)
	assert.Equal(t, "orders", out.Pipelines[0].Name)
	assert.Equal(t, "payments", out.Pipelines[1].Name)
}

func TestListPipelines_Empty(t *testing.T) {
	repo := &mockListPipelinesRepo{listResp: []*domain.Pipeline{}}
	uc := NewListPipelines(repo)

	out, err := uc.Execute(context.Background(), ListPipelinesInput{OrgID: "org-empty"})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "org-empty", repo.listOrg)
	assert.Empty(t, out.Pipelines)
}
