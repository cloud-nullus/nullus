package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

type mockCreatePipelineRepo struct {
	created   []*domain.Pipeline
	createErr error
}

func (m *mockCreatePipelineRepo) Create(_ context.Context, pipeline *domain.Pipeline) error {
	if m.createErr != nil {
		return m.createErr
	}
	copied := *pipeline
	m.created = append(m.created, &copied)
	return nil
}

func (m *mockCreatePipelineRepo) GetByID(_ context.Context, _ string) (*domain.Pipeline, error) {
	return nil, nil
}
func (m *mockCreatePipelineRepo) List(_ context.Context, _ string, _ ...string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (m *mockCreatePipelineRepo) ListByStackID(_ context.Context, _ string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (m *mockCreatePipelineRepo) Update(_ context.Context, _ *domain.Pipeline) error { return nil }
func (m *mockCreatePipelineRepo) Delete(_ context.Context, _ string) error           { return nil }

type mockCreateTemplateRepo struct {
	templates map[string]*domain.PipelineTemplate
	getErr    map[string]error
}

func newMockCreateTemplateRepo(seed ...*domain.PipelineTemplate) *mockCreateTemplateRepo {
	templates := make(map[string]*domain.PipelineTemplate, len(seed))
	for _, t := range seed {
		copied := *t
		templates[t.ID] = &copied
	}
	return &mockCreateTemplateRepo{templates: templates, getErr: map[string]error{}}
}

func (m *mockCreateTemplateRepo) GetByID(_ context.Context, id string) (*domain.PipelineTemplate, error) {
	if err, ok := m.getErr[id]; ok {
		return nil, err
	}
	template, ok := m.templates[id]
	if !ok {
		return nil, errors.New("template not found")
	}
	copied := *template
	return &copied, nil
}

func (m *mockCreateTemplateRepo) List(_ context.Context) ([]*domain.PipelineTemplate, error) {
	return nil, nil
}
func (m *mockCreateTemplateRepo) Create(_ context.Context, _ *domain.PipelineTemplate) error {
	return nil
}
func (m *mockCreateTemplateRepo) Update(_ context.Context, _ *domain.PipelineTemplate) error {
	return nil
}
func (m *mockCreateTemplateRepo) Delete(_ context.Context, _ string) error { return nil }

func TestCreatePipeline_Success(t *testing.T) {
	pipelineRepo := &mockCreatePipelineRepo{}
	templateRepo := newMockCreateTemplateRepo(&domain.PipelineTemplate{ID: "tmpl-1", Name: "backend"})
	uc := NewCreatePipeline(pipelineRepo, templateRepo)

	out, err := uc.Execute(context.Background(), CreatePipelineInput{
		Name:       "orders",
		TemplateID: "tmpl-1",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
		Namespace:  "apps",
		AppType:    domain.AppTypeBackend,
		GitRepoURL: "https://github.com/acme/orders",
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Pipeline)
	assert.Equal(t, "orders", out.Pipeline.Name)
	assert.Equal(t, domain.PipelineStatusActive, out.Pipeline.Status)
	require.Len(t, pipelineRepo.created, 1)
	assert.Equal(t, out.Pipeline.ID, pipelineRepo.created[0].ID)
}

func TestCreatePipeline_TemplateNotFound(t *testing.T) {
	pipelineRepo := &mockCreatePipelineRepo{}
	templateRepo := newMockCreateTemplateRepo()
	templateRepo.getErr["missing-template"] = errors.New("not found")
	uc := NewCreatePipeline(pipelineRepo, templateRepo)

	out, err := uc.Execute(context.Background(), CreatePipelineInput{
		Name:       "orders",
		TemplateID: "missing-template",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "template not found")
	assert.Empty(t, pipelineRepo.created)
}
