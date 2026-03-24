package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDeployPipelineRepo struct {
	pipelines map[string]*domain.Pipeline
	getErr    map[string]error
}

func newMockDeployPipelineRepo(seed ...*domain.Pipeline) *mockDeployPipelineRepo {
	pipelines := make(map[string]*domain.Pipeline, len(seed))
	for _, p := range seed {
		copied := *p
		pipelines[p.ID] = &copied
	}
	return &mockDeployPipelineRepo{pipelines: pipelines, getErr: map[string]error{}}
}

func (m *mockDeployPipelineRepo) Create(_ context.Context, _ *domain.Pipeline) error { return nil }
func (m *mockDeployPipelineRepo) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	if err, ok := m.getErr[id]; ok {
		return nil, err
	}
	p, ok := m.pipelines[id]
	if !ok {
		return nil, errors.New("pipeline not found")
	}
	copied := *p
	return &copied, nil
}
func (m *mockDeployPipelineRepo) List(_ context.Context, _ string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (m *mockDeployPipelineRepo) Update(_ context.Context, _ *domain.Pipeline) error { return nil }

type mockDeployDeploymentRepo struct {
	created   []*domain.Deployment
	createErr error
}

func (m *mockDeployDeploymentRepo) Create(_ context.Context, d *domain.Deployment) error {
	if m.createErr != nil {
		return m.createErr
	}
	copied := *d
	m.created = append(m.created, &copied)
	return nil
}
func (m *mockDeployDeploymentRepo) GetByID(_ context.Context, _ string) (*domain.Deployment, error) {
	return nil, nil
}
func (m *mockDeployDeploymentRepo) ListByPipelineID(_ context.Context, _ string) ([]*domain.Deployment, error) {
	return nil, nil
}
func (m *mockDeployDeploymentRepo) Update(_ context.Context, _ *domain.Deployment) error {
	return nil
}

func TestDeployPipeline_Success(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", OrgID: "org-1"},
	)
	deploymentRepo := &mockDeployDeploymentRepo{}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "v1.2.0",
		DeployedBy: "devops@acme.io",
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Deployment)
	assert.Equal(t, "pip-1", out.Deployment.PipelineID)
	assert.Equal(t, "v1.2.0", out.Deployment.Version)
	assert.Equal(t, domain.DeploymentStatusSuccess, out.Deployment.Status)
	assert.NotEmpty(t, out.Deployment.ID)
	require.Len(t, deploymentRepo.created, 1)
}

func TestDeployPipeline_PipelineNotFound(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo()
	deploymentRepo := &mockDeployDeploymentRepo{}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "missing",
		Version:    "v1.0.0",
		DeployedBy: "devops@acme.io",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "pipeline not found")
	assert.Empty(t, deploymentRepo.created)
}

func TestDeployPipeline_MissingPipelineID(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo()
	deploymentRepo := &mockDeployDeploymentRepo{}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "",
		Version:    "v1.0.0",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "pipeline_id is required")
}

func TestDeployPipeline_MissingVersion(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders"},
	)
	deploymentRepo := &mockDeployDeploymentRepo{}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "version is required")
}

func TestDeployPipeline_DeploymentRepoError(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps"},
	)
	deploymentRepo := &mockDeployDeploymentRepo{createErr: errors.New("db error")}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "v1.0.0",
		DeployedBy: "devops@acme.io",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "create deployment")
}
