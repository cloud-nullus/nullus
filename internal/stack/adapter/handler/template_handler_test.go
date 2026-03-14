package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
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
	h.RegisterRoutes(v1)

	return e
}

func TestTemplateHandler_ListTemplates_ReturnsThree(t *testing.T) {
	e := newTemplateEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	items, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 3)
}

func TestTemplateHandler_GetTemplate_200(t *testing.T) {
	e := newTemplateEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/gitlab-allinone-v1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gitlab-allinone-v1", data["id"])
}

func TestTemplateHandler_GetTemplate_NotFound(t *testing.T) {
	e := newTemplateEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates/does-not-exist", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
