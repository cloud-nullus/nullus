package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
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
func (m *mockCreatePipelineRepo) List(_ context.Context, _ string) ([]*domain.Pipeline, error) {
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
	assert.Equal(t, "emergency_direct", out.Pipeline.ExecutionMode)
	require.Len(t, pipelineRepo.created, 1)
	assert.Equal(t, out.Pipeline.ID, pipelineRepo.created[0].ID)
}

func TestCreatePipeline_DefaultExecutionModeForStackIntegrated(t *testing.T) {
	pipelineRepo := &mockCreatePipelineRepo{}
	templateRepo := newMockCreateTemplateRepo(&domain.PipelineTemplate{ID: "tmpl-1", Name: "backend"})
	uc := NewCreatePipeline(pipelineRepo, templateRepo)

	out, err := uc.Execute(context.Background(), CreatePipelineInput{
		Name:       "orders",
		TemplateID: "tmpl-1",
		OrgID:      "org-1",
		ClusterID:  "cluster-1",
		StackID:    "stack-1",
		Namespace:  "apps",
		AppType:    domain.AppTypeBackend,
		GitRepoURL: "https://github.com/acme/orders",
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "stack_integrated", out.Pipeline.ExecutionMode)
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

// 스택에 묶인 파이프라인의 앱은 스택의 CD 도구가 스택 클러스터에 배포한다.
// 다른 클러스터를 받아 저장하면 모니터링·삭제가 빈 클러스터를 뒤지고, 사용자는
// 앱이 거기 있는 줄 안다. 조용히 바꾸지 않고 거절한다.
func TestCreatePipeline_RejectsClusterOtherThanStackCluster(t *testing.T) {
	pipelineRepo := &mockCreatePipelineRepo{}
	templateRepo := newMockCreateTemplateRepo()
	reader := &stubStackReader{summary: &port.StackSummary{
		ID: "stack-1", OrgID: "org-1", ClusterID: "c-stack", State: "completed",
	}}
	uc := NewCreatePipeline(pipelineRepo, templateRepo, reader)

	out, err := uc.Execute(context.Background(), CreatePipelineInput{
		Name:          "orders",
		OrgID:         "org-1",
		ClusterID:     "c-other",
		StackID:       "stack-1",
		ExecutionMode: domain.ExecutionModeEmergencyDirect,
		Namespace:     "apps",
		AppType:       domain.AppTypeBackend,
	})

	require.ErrorIs(t, err, ErrStackClusterMismatch)
	assert.Contains(t, err.Error(), "c-stack")
	assert.Contains(t, err.Error(), "c-other")
	assert.Nil(t, out)
	assert.Empty(t, pipelineRepo.created)
}

// 클러스터를 비우고 스택만 주면 스택 클러스터로 채운다 — 스택이 정하는 값을
// 호출자가 다시 알아내 적어 보낼 이유가 없다.
func TestCreatePipeline_FillsClusterFromStackWhenEmpty(t *testing.T) {
	pipelineRepo := &mockCreatePipelineRepo{}
	templateRepo := newMockCreateTemplateRepo()
	reader := &stubStackReader{summary: &port.StackSummary{
		ID: "stack-1", OrgID: "org-1", ClusterID: "c-stack", State: "completed",
	}}
	uc := NewCreatePipeline(pipelineRepo, templateRepo, reader)

	out, err := uc.Execute(context.Background(), CreatePipelineInput{
		Name:          "orders",
		OrgID:         "org-1",
		StackID:       "stack-1",
		ExecutionMode: domain.ExecutionModeEmergencyDirect,
		Namespace:     "apps",
		AppType:       domain.AppTypeBackend,
	})

	require.NoError(t, err)
	assert.Equal(t, "c-stack", out.Pipeline.ClusterID)
	require.Len(t, pipelineRepo.created, 1)
	assert.Equal(t, "c-stack", pipelineRepo.created[0].ClusterID)
}

func TestCreatePipeline_AcceptsStackCluster(t *testing.T) {
	pipelineRepo := &mockCreatePipelineRepo{}
	templateRepo := newMockCreateTemplateRepo()
	reader := &stubStackReader{summary: &port.StackSummary{
		ID: "stack-1", OrgID: "org-1", ClusterID: "c-stack", State: "completed",
	}}
	uc := NewCreatePipeline(pipelineRepo, templateRepo, reader)

	out, err := uc.Execute(context.Background(), CreatePipelineInput{
		Name:          "orders",
		OrgID:         "org-1",
		ClusterID:     "c-stack",
		StackID:       "stack-1",
		ExecutionMode: domain.ExecutionModeEmergencyDirect,
		Namespace:     "apps",
		AppType:       domain.AppTypeBackend,
	})

	require.NoError(t, err)
	assert.Equal(t, "c-stack", out.Pipeline.ClusterID)
}

// 스택 없는 파이프라인은 클러스터를 채워 줄 곳이 없다 — 종전대로 필수다.
func TestCreatePipeline_ClusterRequiredWithoutStack(t *testing.T) {
	uc := NewCreatePipeline(&mockCreatePipelineRepo{}, newMockCreateTemplateRepo())

	_, err := uc.Execute(context.Background(), CreatePipelineInput{
		Name: "orders", OrgID: "org-1", Namespace: "apps", AppType: domain.AppTypeBackend,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster_id is required")
}
