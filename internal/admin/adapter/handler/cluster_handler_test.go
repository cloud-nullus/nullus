package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminhandler "github.com/cloud-nullus/draft/internal/admin/adapter/handler"
	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClusterEcho() (*echo.Echo, *adminhandler.ClusterHandler) {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	clusterRepo := adminrepo.NewMemoryClusterRepository()
	clusterUC := usecase.NewClusterUseCase(clusterRepo)
	h := adminhandler.NewClusterHandler(clusterUC)

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
