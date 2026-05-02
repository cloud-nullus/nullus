package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/cloud-nullus/draft/internal/stack/port"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

type slowDeleteKubeconfigProvider struct{}

func (slowDeleteKubeconfigProvider) GetKubeconfig(context.Context, string) ([]byte, error) {
	return []byte("kubeconfig"), nil
}

type slowDeleteInstaller struct{}

func (slowDeleteInstaller) Install(context.Context, port.HelmInstallRequest) (*port.HelmInstallResult, error) {
	return nil, nil
}

func (slowDeleteInstaller) Uninstall(context.Context, string, string) error {
	time.Sleep(150 * time.Millisecond)
	return nil
}

func (slowDeleteInstaller) Status(context.Context, string, string) (*port.HelmInstallResult, error) {
	return nil, nil
}

func newStackEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	memStackRepo := stackrepo.NewMemoryStackRepository()
	memTemplateRepo := stackrepo.NewMemoryTemplateRepository()
	memHistoryRepo := stackrepo.NewMemoryHistoryRepository()
	createStackUC := usecase.NewCreateStack(memStackRepo, memTemplateRepo)
	listStacksUC := usecase.NewListStacks(memStackRepo)
	deleteStackUC := usecase.NewDeleteStack(memStackRepo, nil, nil)
	addToolsUC := usecase.NewAddToolsUseCase(memStackRepo)
	manageHistoryUC := usecase.NewManageHistory(memHistoryRepo)
	h := stackhandler.NewStackHandler(createStackUC, listStacksUC, deleteStackUC, addToolsUC, memStackRepo, manageHistoryUC, nil)
	historyHandler := stackhandler.NewHistoryHandler(memHistoryRepo, memStackRepo, manageHistoryUC)

	v1 := e.Group("/api/v1")
	stacks := v1.Group("/stacks")
	h.RegisterRoutes(stacks)
	historyHandler.RegisterRoutes(stacks)

	return e
}

func newStackEchoWithSlowDelete() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	memStackRepo := stackrepo.NewMemoryStackRepository()
	memTemplateRepo := stackrepo.NewMemoryTemplateRepository()
	memHistoryRepo := stackrepo.NewMemoryHistoryRepository()
	createStackUC := usecase.NewCreateStack(memStackRepo, memTemplateRepo)
	listStacksUC := usecase.NewListStacks(memStackRepo)
	deleteStackUC := usecase.NewDeleteStack(memStackRepo, slowDeleteKubeconfigProvider{}, func([]byte) port.HelmInstaller {
		return slowDeleteInstaller{}
	})
	addToolsUC := usecase.NewAddToolsUseCase(memStackRepo)
	manageHistoryUC := usecase.NewManageHistory(memHistoryRepo)
	h := stackhandler.NewStackHandler(createStackUC, listStacksUC, deleteStackUC, addToolsUC, memStackRepo, manageHistoryUC, nil)
	historyHandler := stackhandler.NewHistoryHandler(memHistoryRepo, memStackRepo, manageHistoryUC)

	v1 := e.Group("/api/v1")
	stacks := v1.Group("/stacks")
	h.RegisterRoutes(stacks)
	historyHandler.RegisterRoutes(stacks)

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

	historyReq := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/"+id+"/history", nil)
	historyRec := httptest.NewRecorder()
	e.ServeHTTP(historyRec, historyReq)
	require.Equal(t, http.StatusOK, historyRec.Code)

	var versions []map[string]any
	require.NoError(t, json.Unmarshal(historyRec.Body.Bytes(), &versions))
	require.Len(t, versions, 1)
	assert.Equal(t, "stack created", versions[0]["ChangeReason"])
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

func TestStackHandler_DeleteStack_202(t *testing.T) {
	e := newStackEcho()

	body := `{"name":"stack-delete-test","cluster_id":"cls-1","golden_path_id":"github-argocd-v1"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Org-ID", "org-delete")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	stackID, ok := createResp["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, stackID)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/stacks/"+stackID, nil)
	deleteRec := httptest.NewRecorder()
	e.ServeHTTP(deleteRec, deleteReq)
	assert.Equal(t, http.StatusAccepted, deleteRec.Code)

	require.Eventually(t, func() bool {
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
		listReq.Header.Set("X-Org-ID", "org-delete")
		listRec := httptest.NewRecorder()
		e.ServeHTTP(listRec, listReq)
		if listRec.Code != http.StatusOK {
			return false
		}

		var listResp map[string]any
		if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
			return false
		}
		items, ok := listResp["items"].([]any)
		return ok && len(items) == 0
	}, 2*time.Second, 20*time.Millisecond)
}

func TestStackHandler_DeleteStack_404WhenNotFound(t *testing.T) {
	e := newStackEcho()

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/stacks/stk-not-found", nil)
	deleteRec := httptest.NewRecorder()
	e.ServeHTTP(deleteRec, deleteReq)

	assert.Equal(t, http.StatusNotFound, deleteRec.Code)
}

func TestStackHandler_CreateStack_409WhileSameKeyDeleting(t *testing.T) {
	e := newStackEchoWithSlowDelete()

	body := `{"name":"race-stack","cluster_id":"cls-1","namespace":"nullus","golden_path_id":"github-argocd-v1"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Org-ID", "org-race")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	stackID, _ := createResp["id"].(string)
	require.NotEmpty(t, stackID)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/stacks/"+stackID, nil)
	deleteRec := httptest.NewRecorder()
	e.ServeHTTP(deleteRec, deleteReq)
	require.Equal(t, http.StatusAccepted, deleteRec.Code)

	recreateReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(body))
	recreateReq.Header.Set("Content-Type", "application/json")
	recreateReq.Header.Set("X-Org-ID", "org-race")
	recreateRec := httptest.NewRecorder()
	e.ServeHTTP(recreateRec, recreateReq)

	assert.Equal(t, http.StatusConflict, recreateRec.Code)
	assert.Contains(t, recreateRec.Body.String(), "STACK_DELETE_IN_PROGRESS")
}

func TestStackHandler_SaveConfig_CreatesHistoryWithYAMLOverridesReason(t *testing.T) {
	e := newStackEcho()

	createBody := `{"name":"stack-config-test","cluster_id":"cls-1","golden_path_id":"github-argocd-v1"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Org-ID", "org-config")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	stackID, ok := createResp["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, stackID)

	cfgBody := `{"config":{"yaml_overrides":{"gitlab":"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: gitlab-override"}}}`
	cfgReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/"+stackID+"/config", strings.NewReader(cfgBody))
	cfgReq.Header.Set("Content-Type", "application/json")
	cfgReq.Header.Set("X-User-ID", "tester")
	cfgRec := httptest.NewRecorder()
	e.ServeHTTP(cfgRec, cfgReq)
	require.Equal(t, http.StatusOK, cfgRec.Code)

	historyReq := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/"+stackID+"/history", nil)
	historyRec := httptest.NewRecorder()
	e.ServeHTTP(historyRec, historyReq)
	require.Equal(t, http.StatusOK, historyRec.Code)

	var versions []map[string]any
	require.NoError(t, json.Unmarshal(historyRec.Body.Bytes(), &versions))
	require.Len(t, versions, 2)
	assert.Equal(t, "stack created", versions[0]["ChangeReason"])
	assert.Equal(t, "yaml_view_customization (1 overrides)", versions[1]["ChangeReason"])
	assert.Equal(t, "tester", versions[1]["ChangedBy"])
}
