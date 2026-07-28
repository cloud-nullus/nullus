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

// 설치 마법사가 StorageClass 를 선택하려면 대상 클러스터의 목록을 조회할 수 있어야 한다.
// 기본 SC 판별과 reclaimPolicy 노출이 핵심 — 전자는 기본 선택값을,
// 후자는 스택 삭제 후 볼륨 잔존 경고를 결정한다.
func TestClusterHandler_ListStorageClasses_ReturnsItems(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "nullus-dev-key-32bytes-padding!!")

	prevLister := storageClassListerFn
	storageClassListerFn = func(_ []byte) ([]clusterStorageClassResponseItem, error) {
		return []clusterStorageClassResponseItem{
			{
				Name:              "standard",
				Provisioner:       "rancher.io/local-path",
				IsDefault:         true,
				ReclaimPolicy:     "Delete",
				VolumeBindingMode: "WaitForFirstConsumer",
			},
			{
				Name:              "nfs-client",
				Provisioner:       "cluster.local/nfs-subdir",
				IsDefault:         false,
				ReclaimPolicy:     "Retain",
				VolumeBindingMode: "Immediate",
			},
		}, nil
	}
	t.Cleanup(func() { storageClassListerFn = prevLister })

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	clusterRepo := adminrepo.NewMemoryClusterRepository()
	clusterUC := usecase.NewClusterUseCase(clusterRepo)
	h := NewClusterHandler(clusterUC)

	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	registerBody := `{"name":"sc-cluster","type":"target","endpoint":"https://k8s.example.com","org_id":"org-1","kubeconfig":"apiVersion: v1\nkind: Config\n"}`
	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	e.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	var registerResp map[string]any
	require.NoError(t, json.Unmarshal(registerRec.Body.Bytes(), &registerResp))
	clusterID, ok := registerResp["id"].(string)
	require.True(t, ok)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters/"+clusterID+"/storage-classes", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)

	require.Equal(t, http.StatusOK, listRec.Code, listRec.Body.String())

	var listResp struct {
		Data []clusterStorageClassResponseItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 2)

	assert.Equal(t, "standard", listResp.Data[0].Name)
	assert.True(t, listResp.Data[0].IsDefault)
	assert.Equal(t, "Retain", listResp.Data[1].ReclaimPolicy)
}

// kubeconfig 가 등록되지 않은 클러스터는 조회할 수 없다.
func TestClusterHandler_ListStorageClasses_RequiresKubeconfig(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "nullus-dev-key-32bytes-padding!!")

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	clusterRepo := adminrepo.NewMemoryClusterRepository()
	clusterUC := usecase.NewClusterUseCase(clusterRepo)
	h := NewClusterHandler(clusterUC)

	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	registerBody := `{"name":"no-kubeconfig","type":"target","endpoint":"https://k8s.example.com","org_id":"org-1"}`
	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters", strings.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	e.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	var registerResp map[string]any
	require.NoError(t, json.Unmarshal(registerRec.Body.Bytes(), &registerResp))
	clusterID, _ := registerResp["id"].(string)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters/"+clusterID+"/storage-classes", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)

	assert.Equal(t, http.StatusBadRequest, listRec.Code)
}
