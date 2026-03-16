package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTemplateRepository struct {
	listResp []*domain.PipelineTemplate
	listErr  error
}

func (m *mockTemplateRepository) GetByID(_ context.Context, _ string) (*domain.PipelineTemplate, error) {
	return nil, nil
}
func (m *mockTemplateRepository) Create(_ context.Context, _ *domain.PipelineTemplate) error {
	return nil
}
func (m *mockTemplateRepository) Update(_ context.Context, _ *domain.PipelineTemplate) error {
	return nil
}
func (m *mockTemplateRepository) Delete(_ context.Context, _ string) error { return nil }

func (m *mockTemplateRepository) List(_ context.Context) ([]*domain.PipelineTemplate, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*domain.PipelineTemplate, 0, len(m.listResp))
	for _, template := range m.listResp {
		copied := *template
		result = append(result, &copied)
	}
	return result, nil
}

func newCICDTemplateEcho(repo *mockTemplateRepository) *echo.Echo {
	e := echo.New()
	h := cicdhandler.NewCICDTemplateHandler(repo)
	v1 := e.Group("/api/v1/cicd")
	h.RegisterRoutes(v1)
	return e
}

func TestCICDTemplateHandler_List_Success(t *testing.T) {
	repo := &mockTemplateRepository{listResp: []*domain.PipelineTemplate{
		{ID: "tmpl-1", Name: "Web Backend", AppType: domain.AppTypeBackend, Stages: []string{"Build", "Deploy"}},
		{ID: "tmpl-2", Name: "Web Frontend", AppType: domain.AppTypeWeb, Stages: []string{"Build", "StaticBuild", "Deploy"}},
	}}
	e := newCICDTemplateEcho(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cicd/templates", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp []domain.PipelineTemplate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, "tmpl-1", resp[0].ID)
	assert.Equal(t, "tmpl-2", resp[1].ID)
}

func TestCICDTemplateHandler_List_EmptyResult(t *testing.T) {
	repo := &mockTemplateRepository{listResp: []*domain.PipelineTemplate{}}
	e := newCICDTemplateEcho(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cicd/templates", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}
