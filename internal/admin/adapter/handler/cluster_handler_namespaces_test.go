package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adminrepo "github.com/cloud-nullus/draft/internal/admin/adapter/repository"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/middleware"
)

func TestClusterHandler_ListNamespaces_FiltersSystemNamespaces(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "nullus-dev-key-32bytes-padding!!")

	prevLister := namespaceListerFn
	namespaceListerFn = func(_ []byte) ([]string, error) {
		return []string{"default", "kube-system", "production", "kube-public", "kube-node-lease", "local-path-storage"}, nil
	}
	t.Cleanup(func() {
		namespaceListerFn = prevLister
	})

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	clusterRepo := adminrepo.NewMemoryClusterRepository()
	clusterUC := usecase.NewClusterUseCase(clusterRepo)
	h := NewClusterHandler(clusterUC)

	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	registerBody := `{"name":"cluster-with-kubeconfig","type":"target","endpoint":"https://k8s.example.com","org_id":"org-1","kubeconfig":"apiVersion: v1\nkind: Config\n"}`
	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	e.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	var registerResp map[string]any
	require.NoError(t, json.Unmarshal(registerRec.Body.Bytes(), &registerResp))
	clusterID, ok := registerResp["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, clusterID)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters/"+clusterID+"/namespaces", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)

	assert.Equal(t, http.StatusOK, listRec.Code)

	var listResp map[string]any
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	items, ok := listResp["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)

	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	second, ok := items[1].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "default", first["name"])
	assert.Equal(t, "production", second["name"])
}
