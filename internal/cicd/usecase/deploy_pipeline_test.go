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

type mockKubeconfigProvider struct {
	kubeconfig []byte
	err        error
}

func (m *mockKubeconfigProvider) GetKubeconfig(_ context.Context, _ string) ([]byte, error) {
	return m.kubeconfig, m.err
}

type mockManifestApplier struct {
	appliedManifests [][]string
	err              error
}

func (m *mockManifestApplier) Apply(_ context.Context, _ []byte, manifests []string) error {
	m.appliedManifests = append(m.appliedManifests, manifests)
	return m.err
}

func TestDeployPipeline_Success(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", ClusterID: "c1", AppType: domain.AppTypeBackend, OrgID: "org-1"},
	)
	deploymentRepo := &mockDeployDeploymentRepo{}
	kubeconfigProvider := &mockKubeconfigProvider{kubeconfig: []byte("fake-kubeconfig")}
	applier := &mockManifestApplier{}

	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, kubeconfigProvider, applier)

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
	require.Len(t, applier.appliedManifests, 1)
}

func TestDeployPipeline_PipelineNotFound(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo()
	deploymentRepo := &mockDeployDeploymentRepo{}
	kubeconfigProvider := &mockKubeconfigProvider{}
	applier := &mockManifestApplier{}

	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, kubeconfigProvider, applier)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "missing",
		Version:    "v1.0.0",
		DeployedBy: "devops@acme.io",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "pipeline not found")
}

func TestDeployPipeline_MissingPipelineID(t *testing.T) {
	uc := NewDeployPipeline(
		newMockDeployPipelineRepo(), &mockDeployDeploymentRepo{},
		&mockKubeconfigProvider{}, &mockManifestApplier{},
	)

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
	uc := NewDeployPipeline(pipelineRepo, &mockDeployDeploymentRepo{}, &mockKubeconfigProvider{}, &mockManifestApplier{})

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
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", ClusterID: "c1"},
	)
	deploymentRepo := &mockDeployDeploymentRepo{createErr: errors.New("db error")}
	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, &mockKubeconfigProvider{}, &mockManifestApplier{})

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "v1.0.0",
		DeployedBy: "devops@acme.io",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "create deployment")
}

func TestDeployPipeline_ApplierError(t *testing.T) {
	pipelineRepo := newMockDeployPipelineRepo(
		&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", ClusterID: "c1", AppType: domain.AppTypeBackend},
	)
	deploymentRepo := &mockDeployDeploymentRepo{}
	kubeconfigProvider := &mockKubeconfigProvider{kubeconfig: []byte("fake")}
	applier := &mockManifestApplier{err: errors.New("apply failed")}

	uc := NewDeployPipeline(pipelineRepo, deploymentRepo, kubeconfigProvider, applier)

	out, err := uc.Execute(context.Background(), DeployPipelineInput{
		PipelineID: "pip-1",
		Version:    "v1.0.0",
		DeployedBy: "devops@acme.io",
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "apply to cluster")
	require.Len(t, deploymentRepo.created, 1)
	assert.Equal(t, domain.DeploymentStatusRunning, deploymentRepo.created[0].Status)
}
