package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStackEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	memStackRepo := stackrepo.NewMemoryStackRepository()
	memTemplateRepo := stackrepo.NewMemoryTemplateRepository()
	createStackUC := usecase.NewCreateStack(memStackRepo, memTemplateRepo)
	listStacksUC := usecase.NewListStacks(memStackRepo)
	h := stackhandler.NewStackHandler(createStackUC, listStacksUC, memStackRepo)

	v1 := e.Group("/api/v1")
	stacks := v1.Group("/stacks")
	h.RegisterRoutes(stacks)

	return e
}

func TestStackHandler_CreateStack_201(t *testing.T) {
	e := newStackEcho()

	body := `{"name":"my-stack","cluster_id":"cls-1","golden_path_id":"gitlab-allinone-v1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", "org-test")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	id, ok := resp["id"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, id)
}

func TestStackHandler_ListStacks_200(t *testing.T) {
	e := newStackEcho()

	// Create a stack first
	body := `{"name":"stack-list-test","cluster_id":"cls-1","golden_path_id":"github-argocd-v1"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Org-ID", "org-list")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	// List stacks
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	listReq.Header.Set("X-Org-ID", "org-list")
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)

	assert.Equal(t, http.StatusOK, listRec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &resp))
	items, ok := resp["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 1)
	assert.EqualValues(t, 1, resp["total"])
}
