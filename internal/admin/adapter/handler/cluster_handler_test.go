package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminhandler "github.com/cloud-nullus/draft/internal/admin/adapter/handler"
	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
)

func newClusterEcho() (*echo.Echo, *adminhandler.ClusterHandler) {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	clusterRepo := adminrepo.NewMemoryClusterRepository()
	clusterUC := usecase.NewClusterUseCase(clusterRepo)
	h := adminhandler.NewClusterHandler(clusterUC, nil)

	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	return e, h
}

func TestClusterHandler_RegisterCluster_201(t *testing.T) {
	e, _ := newClusterEcho()

	body := `{"name":"prod-cluster","type":"target","endpoint":"https://k8s.example.com","org_id":"org-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "prod-cluster", resp["name"])
	assert.Equal(t, "pending", resp["connection_status"])
}

func TestClusterHandler_RegisterCluster_WithKubeconfigAndInvalidKey_500(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "short-key")
	e, _ := newClusterEcho()

	body := `{"name":"prod-cluster","type":"target","endpoint":"https://k8s.example.com","org_id":"org-1","kubeconfig":"apiVersion: v1\nkind: Config"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)

	var listResp map[string]any
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	assert.EqualValues(t, 0, listResp["total"])
}

func TestClusterHandler_RegisterCluster_WithoutOrgIDAndNoOrg_400(t *testing.T) {
	e, _ := newClusterEcho()

	body := `{"name":"prod-cluster","type":"target","endpoint":"https://k8s.example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestClusterHandler_ListClusters_200(t *testing.T) {
	e, _ := newClusterEcho()

	// Register two clusters
	for _, name := range []string{"cluster-a", "cluster-b"} {
		body := `{"name":"` + name + `","type":"target","endpoint":"https://k8s.example.com","org_id":"org-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	items, ok := resp["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 2)
	assert.EqualValues(t, 2, resp["total"])
}

func TestClusterHandler_DeleteCluster_204(t *testing.T) {
	e, _ := newClusterEcho()

	// Create cluster
	body := `{"name":"to-delete","type":"pipeline","endpoint":"https://k8s.example.com","org_id":"org-1"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	id := createResp["id"].(string)

	// Delete
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/clusters/"+id, nil)
	delRec := httptest.NewRecorder()
	e.ServeHTTP(delRec, delReq)

	assert.Equal(t, http.StatusNoContent, delRec.Code)
}

func TestClusterHandler_GetCluster_IncludesKubeconfig_200(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	e, _ := newClusterEcho()

	kubeconfig := "apiVersion: v1\nkind: Config\nclusters: []"
	body := `{"name":"with-kubeconfig","type":"target","endpoint":"https://k8s.example.com","org_id":"org-1","kubeconfig":"` + strings.ReplaceAll(kubeconfig, "\n", "\\n") + `"}`

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	id := createResp["id"].(string)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters/"+id, nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var got map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &got))
	assert.Equal(t, kubeconfig, got["kubeconfig"])
}

func TestClusterHandler_UpdateCluster_SavesKubeconfig_200(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	e, _ := newClusterEcho()

	createBody := `{"name":"editable-cluster","type":"target","endpoint":"https://k8s.example.com","org_id":"org-1","kubeconfig":"apiVersion: v1\nkind: Config"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	id := createResp["id"].(string)

	updatedKubeconfig := "apiVersion: v1\nkind: Config\ncontexts: []"
	updateBody := `{"name":"edited-cluster","endpoint":"https://edited.k8s.example.com","kubeconfig":"` + strings.ReplaceAll(updatedKubeconfig, "\n", "\\n") + `"}`
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/clusters/"+id, strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	e.ServeHTTP(updateRec, updateReq)
	require.Equal(t, http.StatusOK, updateRec.Code)
	var updateResp map[string]any
	require.NoError(t, json.Unmarshal(updateRec.Body.Bytes(), &updateResp))
	assert.Equal(t, updatedKubeconfig, updateResp["kubeconfig"])

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters/"+id, nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var got map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &got))
	assert.Equal(t, "edited-cluster", got["name"])
	assert.Equal(t, "https://edited.k8s.example.com", got["endpoint"])
	assert.Equal(t, updatedKubeconfig, got["kubeconfig"])
}
