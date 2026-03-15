package handler_test

import (
	"bytes"
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
	h := stackhandler.NewTemplateHandler(getTemplateUC, listTemplatesUC, templateRepo)

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

func TestTemplateHandler_CreateTemplate_201(t *testing.T) {
	e := newTemplateEcho()

	body := []byte(`{
		"id":"custom-template-v1",
		"name":"Custom Template",
		"description":"Custom description",
		"tools":[{"category":"cd_tool","name":"Argo CD","helm_version":"7.7.2","app_version":"2.13.2"}],
		"estimated_install_time":1800000000000,
		"recommended_use_case":"테스트",
		"min_resources":"2 vCPU / 4Gi RAM / 20Gi Storage"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/templates", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	checkReq := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/templates/custom-template-v1", nil)
	checkRec := httptest.NewRecorder()
	e.ServeHTTP(checkRec, checkReq)
	assert.Equal(t, http.StatusOK, checkRec.Code)
}

func TestTemplateHandler_UpdateTemplate_200(t *testing.T) {
	e := newTemplateEcho()

	body := []byte(`{
		"id":"gitlab-allinone-v1",
		"name":"GitLab All-in-One Updated",
		"description":"Updated description",
		"tools":[{"category":"source_repository","name":"GitLab CE","helm_version":"8.7.2","app_version":"17.7.2"}],
		"estimated_install_time":3600000000000,
		"recommended_use_case":"업데이트 테스트",
		"min_resources":"4 vCPU / 8Gi RAM / 50Gi Storage"
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/stacks/templates/gitlab-allinone-v1", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "GitLab All-in-One Updated", resp["name"])
}

func TestTemplateHandler_DeleteTemplate_204(t *testing.T) {
	e := newTemplateEcho()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stacks/templates/gitlab-allinone-v1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	checkReq := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/templates/gitlab-allinone-v1", nil)
	checkRec := httptest.NewRecorder()
	e.ServeHTTP(checkRec, checkReq)
	assert.Equal(t, http.StatusNotFound, checkRec.Code)
}
