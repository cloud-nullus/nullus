package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/adapter/kube"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/usecase"
)

type mockPipelineRepository struct {
	pipelines map[string]*domain.Pipeline
	createErr error
	listErr   error
	getErr    map[string]error
	created   int
	deleteErr error
	deleted   []string
}

func newMockPipelineRepository(seed ...*domain.Pipeline) *mockPipelineRepository {
	pipelines := make(map[string]*domain.Pipeline, len(seed))
	for _, p := range seed {
		copied := *p
		pipelines[p.ID] = &copied
	}

	return &mockPipelineRepository{
		pipelines: pipelines,
		getErr:    map[string]error{},
	}
}

func (m *mockPipelineRepository) Create(_ context.Context, pipeline *domain.Pipeline) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created++
	copied := *pipeline
	m.pipelines[pipeline.ID] = &copied
	return nil
}

func (m *mockPipelineRepository) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	if err, ok := m.getErr[id]; ok {
		return nil, err
	}
	pipeline, ok := m.pipelines[id]
	if !ok {
		return nil, errors.New("pipeline not found")
	}
	copied := *pipeline
	return &copied, nil
}

func (m *mockPipelineRepository) List(_ context.Context, orgID string, stackID ...string) ([]*domain.Pipeline, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	filterStack := len(stackID) > 0 && stackID[0] != ""
	result := make([]*domain.Pipeline, 0, len(m.pipelines))
	for _, p := range m.pipelines {
		if p.OrgID == orgID {
			if filterStack && p.StackID != stackID[0] {
				continue
			}
			copied := *p
			result = append(result, &copied)
		}
	}
	return result, nil
}

func (m *mockPipelineRepository) ListByStackID(_ context.Context, stackID string) ([]*domain.Pipeline, error) {
	result := make([]*domain.Pipeline, 0)
	for _, p := range m.pipelines {
		if p.StackID == stackID {
			copied := *p
			result = append(result, &copied)
		}
	}
	return result, nil
}

func (m *mockPipelineRepository) Update(_ context.Context, _ *domain.Pipeline) error { return nil }

func (m *mockPipelineRepository) Delete(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.pipelines[id]; !ok {
		return errors.New("pipeline not found")
	}
	delete(m.pipelines, id)
	m.deleted = append(m.deleted, id)
	return nil
}

type mockPipelineTemplateRepository struct {
	templates map[string]*domain.PipelineTemplate
	getErr    map[string]error
	listErr   error
}

func newMockPipelineTemplateRepository(seed ...*domain.PipelineTemplate) *mockPipelineTemplateRepository {
	templates := make(map[string]*domain.PipelineTemplate, len(seed))
	for _, t := range seed {
		copied := *t
		templates[t.ID] = &copied
	}

	return &mockPipelineTemplateRepository{
		templates: templates,
		getErr:    map[string]error{},
	}
}

func (m *mockPipelineTemplateRepository) GetByID(_ context.Context, id string) (*domain.PipelineTemplate, error) {
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

func (m *mockPipelineTemplateRepository) List(_ context.Context) ([]*domain.PipelineTemplate, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*domain.PipelineTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		copied := *t
		result = append(result, &copied)
	}
	return result, nil
}
func (m *mockPipelineTemplateRepository) Create(_ context.Context, _ *domain.PipelineTemplate) error {
	return nil
}
func (m *mockPipelineTemplateRepository) Update(_ context.Context, _ *domain.PipelineTemplate) error {
	return nil
}
func (m *mockPipelineTemplateRepository) Delete(_ context.Context, _ string) error { return nil }

type mockDeploymentRepository struct {
	deployments []*domain.Deployment
	createErr   error
}

func (m *mockDeploymentRepository) Create(_ context.Context, deployment *domain.Deployment) error {
	if m.createErr != nil {
		return m.createErr
	}
	copied := *deployment
	m.deployments = append(m.deployments, &copied)
	return nil
}

func (m *mockDeploymentRepository) GetByID(_ context.Context, id string) (*domain.Deployment, error) {
	for _, d := range m.deployments {
		if d.ID == id {
			copied := *d
			return &copied, nil
		}
	}
	return nil, errors.New("deployment not found")
}

func (m *mockDeploymentRepository) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.Deployment, error) {
	result := make([]*domain.Deployment, 0)
	for _, d := range m.deployments {
		if d.PipelineID == pipelineID {
			copied := *d
			result = append(result, &copied)
		}
	}
	return result, nil
}

func (m *mockDeploymentRepository) Update(_ context.Context, _ *domain.Deployment) error { return nil }

type noopKubeconfigProvider struct{}

func (n *noopKubeconfigProvider) GetKubeconfig(_ context.Context, _ string) ([]byte, error) {
	return []byte("fake-kubeconfig"), nil
}

type noopManifestApplier struct{}

func (n *noopManifestApplier) Apply(_ context.Context, _ []byte, _ []string) error { return nil }
func (n *noopManifestApplier) ApplyWithTracking(_ context.Context, _ []byte, _ []string, _ string, _ ...int) error {
	return nil
}

func newPipelineEcho(t *testing.T, pipelineRepo *mockPipelineRepository, templateRepo *mockPipelineTemplateRepository, deploymentRepo *mockDeploymentRepository) *echo.Echo {
	t.Helper()

	e := echo.New()
	createPipelineUC := usecase.NewCreatePipeline(pipelineRepo, templateRepo)
	listPipelinesUC := usecase.NewListPipelines(pipelineRepo)
	deployPipelineUC := usecase.NewDeployPipeline(pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, &noopManifestApplier{})
	h := cicdhandler.NewPipelineHandler(createPipelineUC, listPipelinesUC, deployPipelineUC, pipelineRepo, deploymentRepo, &noopKubeconfigProvider{}, kube.NewStepTracker(), nil)

	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1)

	return e
}

func TestPipelineHandler_Create_Success(t *testing.T) {
	pipelineRepo := newMockPipelineRepository()
	templateRepo := newMockPipelineTemplateRepository(&domain.PipelineTemplate{ID: "tmpl-1", Name: "backend"})
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	body := `{"name":"orders","template_id":"tmpl-1","cluster_id":"cluster-1","namespace":"apps","app_type":"backend","git_repo_url":"https://github.com/acme/orders"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", "org-1")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, 1, pipelineRepo.created)

	var resp struct {
		Pipeline domain.Pipeline `json:"pipeline"`
		Warning  string          `json:"warning,omitempty"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "orders", resp.Pipeline.Name)
	assert.Equal(t, "org-1", resp.Pipeline.OrgID)
	assert.Equal(t, domain.PipelineStatusActive, resp.Pipeline.Status)
	assert.NotEmpty(t, resp.Pipeline.ID)
	assert.Empty(t, resp.Warning)
}

func TestPipelineHandler_Create_InvalidBody(t *testing.T) {
	pipelineRepo := newMockPipelineRepository()
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, pipelineRepo.created)

	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "PIPELINE_CONFIG_INVALID", resp["error"]["code"])
}

func TestPipelineHandler_List_Success(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(
		&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "org-1"},
		&domain.Pipeline{ID: "pip-2", Name: "payments", OrgID: "org-1"},
		&domain.Pipeline{ID: "pip-3", Name: "billing", OrgID: "org-2"},
	)
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pipelines", nil)
	req.Header.Set("X-Org-ID", "org-1")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Items []domain.Pipeline `json:"items"`
		Total int               `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	require.Len(t, resp.Items, 2)
}

func TestPipelineHandler_Deploy_Success(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(&domain.Pipeline{ID: "pip-1", Name: "orders", Namespace: "apps", OrgID: "org-1"})
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	body := `{"version":"v1.0.0","deployed_by":"devops@acme.io"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/pip-1/deploy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Len(t, deploymentRepo.deployments, 1)
	assert.Equal(t, "pip-1", deploymentRepo.deployments[0].PipelineID)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["deploymentId"])
}

func TestPipelineHandler_Deploy_NotFound(t *testing.T) {
	pipelineRepo := newMockPipelineRepository()
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	body := `{"version":"v1.0.0","deployed_by":"devops@acme.io"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/missing/deploy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Empty(t, deploymentRepo.deployments)

	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "PIPELINE_DEPLOY_FAILED", resp["error"]["code"])
	assert.Contains(t, resp["error"]["message"], "pipeline not found")
}

func TestPipelineHandler_Delete_Success(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "org-1"})
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pipelines/pip-1", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Len(t, pipelineRepo.deleted, 1)
	assert.Equal(t, "pip-1", pipelineRepo.deleted[0])
}

func TestPipelineHandler_Delete_NotFound(t *testing.T) {
	pipelineRepo := newMockPipelineRepository()
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pipelines/missing", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "PIPELINE_NOT_FOUND", resp["error"]["code"])
}

func TestPipelineHandler_List_NoOrgHeader(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(
		&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "11111111-1111-1111-1111-111111111111"},
	)
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pipelines", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []domain.Pipeline `json:"items"`
		Total int               `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
}

func TestPipelineHandler_ListDeployments_Success(t *testing.T) {
	pipelineRepo := newMockPipelineRepository(
		&domain.Pipeline{ID: "pip-1", Name: "orders", OrgID: "org-1"},
	)
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{
		deployments: []*domain.Deployment{
			{ID: "dep-1", PipelineID: "pip-1", Version: "v1.0.0"},
		},
	}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/deployments", nil)
	req.Header.Set("X-Org-ID", "org-1")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Items []domain.Deployment `json:"items"`
		Total int                 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "dep-1", resp.Items[0].ID)
}

func TestPipelineHandler_ListAppTemplates_Success(t *testing.T) {
	pipelineRepo := newMockPipelineRepository()
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/app-templates", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 3)
	assert.Equal(t, "go-web-api", resp[0]["id"])
}

func TestPipelineHandler_DeployApp_Success(t *testing.T) {
	pipelineRepo := newMockPipelineRepository()
	templateRepo := newMockPipelineTemplateRepository()
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	body := `{"templateId":"go-web-api","appName":"my-api","clusterId":"c1","namespace":"default","gitUrl":"https://github.com/acme/api","replicas":2,"port":8080,"resources":{"cpuLimit":"500m","memLimit":"512Mi","cpuRequest":"100m","memRequest":"128Mi"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deploy-app", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Regexp(t, `^dep_app_my-api_\d{14}$`, resp["deploymentId"])
	assert.Equal(t, "go-web-api", resp["templateId"])
	assert.NotNil(t, resp["manifests"])
}

func TestPipelineHandler_Create_TemplateNotFound(t *testing.T) {
	pipelineRepo := newMockPipelineRepository()
	templateRepo := newMockPipelineTemplateRepository() // no templates
	deploymentRepo := &mockDeploymentRepository{}
	e := newPipelineEcho(t, pipelineRepo, templateRepo, deploymentRepo)

	body := `{"name":"orders","template_id":"missing","cluster_id":"c1","namespace":"apps","app_type":"backend","git_repo_url":"https://github.com/acme/orders"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", "org-1")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, pipelineRepo.created)
}
