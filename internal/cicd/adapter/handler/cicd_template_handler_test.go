package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cicdhandler "github.com/cloud-nullus/draft/internal/cicd/adapter/handler"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// mockTemplateRepository 는 맵에 저장하는 템플릿 저장소다. 저장 규칙(중복 거부·없으면 not found)은
// 실제 저장소(memory·postgres)와 같아야 핸들러의 상태 코드 분기를 검증할 수 있다.
type mockTemplateRepository struct {
	listResp  []*domain.PipelineTemplate
	listErr   error
	store     map[string]*domain.PipelineTemplate
	createErr error
	updateErr error
	deleteErr error
}

func newStatefulTemplateRepository(seed ...*domain.PipelineTemplate) *mockTemplateRepository {
	store := make(map[string]*domain.PipelineTemplate, len(seed))
	for _, t := range seed {
		copied := *t
		store[t.ID] = &copied
	}
	return &mockTemplateRepository{store: store}
}

func (m *mockTemplateRepository) GetByID(_ context.Context, id string) (*domain.PipelineTemplate, error) {
	t, ok := m.store[id]
	if !ok {
		return nil, fmt.Errorf("pipeline template %q: %w", id, domain.ErrTemplateNotFound)
	}
	copied := *t
	return &copied, nil
}
func (m *mockTemplateRepository) Create(_ context.Context, tmpl *domain.PipelineTemplate) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, ok := m.store[tmpl.ID]; ok {
		return fmt.Errorf("pipeline template %q: %w", tmpl.ID, domain.ErrTemplateAlreadyExists)
	}
	copied := *tmpl
	m.store[tmpl.ID] = &copied
	return nil
}
func (m *mockTemplateRepository) Update(_ context.Context, tmpl *domain.PipelineTemplate) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.store[tmpl.ID]; !ok {
		return fmt.Errorf("pipeline template %q: %w", tmpl.ID, domain.ErrTemplateNotFound)
	}
	copied := *tmpl
	m.store[tmpl.ID] = &copied
	return nil
}
func (m *mockTemplateRepository) Delete(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.store[id]; !ok {
		return fmt.Errorf("pipeline template %q: %w", id, domain.ErrTemplateNotFound)
	}
	delete(m.store, id)
	return nil
}

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
