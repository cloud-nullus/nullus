package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTemplateEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	templateRepo := stackrepo.NewMemoryTemplateRepository()
	getTemplateUC := usecase.NewGetTemplate(templateRepo)
	listTemplatesUC := usecase.NewListTemplates(templateRepo)
	h := stackhandler.NewTemplateHandler(getTemplateUC, listTemplatesUC)

	v1 := e.Group("/api/v1")
	stacks := v1.Group("/stacks")
	h.RegisterRoutes(stacks)

	return e
}

func TestTemplateHandler_ListTemplates_ReturnsThree(t *testing.T) {
	e := newTemplateEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/templates", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var items []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &items))
	assert.Len(t, items, 3)
}

func TestTemplateHandler_GetTemplate_200(t *testing.T) {
	e := newTemplateEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/templates/gitlab-allinone-v1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "gitlab-allinone-v1", resp["id"])
}

func TestTemplateHandler_GetTemplate_NotFound(t *testing.T) {
	e := newTemplateEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/templates/does-not-exist", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
