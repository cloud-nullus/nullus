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

func newOrgEcho() (*echo.Echo, *adminhandler.OrgHandler) {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	orgRepo := adminrepo.NewMemoryOrgRepository()
	orgUC := usecase.NewOrgUseCase(orgRepo)
	h := adminhandler.NewOrgHandler(orgUC, nil)

	v1 := e.Group("/api/v1")
	admin := v1.Group("/admin")
	h.RegisterRoutes(admin)

	return e, h
}

func TestOrgHandler_CreateOrg_201(t *testing.T) {
	e, _ := newOrgEcho()

	body := `{"name":"Acme","slug":"acme","domain":"acme.io"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orgs", strings.NewReader(body))
	req.Header.Set(echo.MIMEApplicationJSON, "application/json")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "acme", resp["slug"])
	assert.Equal(t, "Acme", resp["name"])
}

func TestOrgHandler_GetOrg_200(t *testing.T) {
	e, _ := newOrgEcho()

	// Create first
	createBody := `{"name":"Beta Corp","slug":"beta-corp","domain":"beta.io"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orgs", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	id := createResp["id"].(string)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organization", nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)

	assert.Equal(t, http.StatusOK, getRec.Code)

	var getResp map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	assert.Equal(t, id, getResp["id"])
}

func TestOrgHandler_GetOrg_404WhenNoOrganizationExists(t *testing.T) {
	e, _ := newOrgEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organization", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrgHandler_GetOrg_ReturnsFirstOrganization(t *testing.T) {
	e, _ := newOrgEcho()

	createBodyA := `{"name":"First","slug":"first","domain":"first.io"}`
	createReqA := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orgs", strings.NewReader(createBodyA))
	createReqA.Header.Set("Content-Type", "application/json")
	createRecA := httptest.NewRecorder()
	e.ServeHTTP(createRecA, createReqA)
	require.Equal(t, http.StatusCreated, createRecA.Code)

	var createRespA map[string]any
	require.NoError(t, json.Unmarshal(createRecA.Body.Bytes(), &createRespA))
	firstID := createRespA["id"].(string)

	createBodyB := `{"name":"Second","slug":"second","domain":"second.io"}`
	createReqB := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orgs", strings.NewReader(createBodyB))
	createReqB.Header.Set("Content-Type", "application/json")
	createRecB := httptest.NewRecorder()
	e.ServeHTTP(createRecB, createReqB)
	require.Equal(t, http.StatusCreated, createRecB.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organization", nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var getResp map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	assert.NotEmpty(t, getResp["id"])
	assert.NotEmpty(t, firstID)
}

func TestOrgHandler_UpdateOrg_200(t *testing.T) {
	e, _ := newOrgEcho()

	// Create first
	createBody := `{"name":"Original","slug":"original","domain":"original.io"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orgs", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))

	// Update
	updateBody := `{"name":"Updated Name","domain":"updated.io"}`
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/organization", strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	e.ServeHTTP(updateRec, updateReq)

	assert.Equal(t, http.StatusOK, updateRec.Code)

	var updateResp map[string]any
	require.NoError(t, json.Unmarshal(updateRec.Body.Bytes(), &updateResp))
	assert.Equal(t, "Updated Name", updateResp["name"])
	assert.Equal(t, "updated.io", updateResp["domain"])
}

func TestOrgHandler_UpdateOrg_UpdatesFirstOrganization(t *testing.T) {
	e, _ := newOrgEcho()

	createBodyA := `{"name":"First","slug":"first-updatable","domain":"first.io"}`
	createReqA := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orgs", strings.NewReader(createBodyA))
	createReqA.Header.Set("Content-Type", "application/json")
	createRecA := httptest.NewRecorder()
	e.ServeHTTP(createRecA, createReqA)
	require.Equal(t, http.StatusCreated, createRecA.Code)

	var createRespA map[string]any
	require.NoError(t, json.Unmarshal(createRecA.Body.Bytes(), &createRespA))
	firstID := createRespA["id"].(string)

	createBodyB := `{"name":"Second","slug":"second-updatable","domain":"second.io"}`
	createReqB := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orgs", strings.NewReader(createBodyB))
	createReqB.Header.Set("Content-Type", "application/json")
	createRecB := httptest.NewRecorder()
	e.ServeHTTP(createRecB, createReqB)
	require.Equal(t, http.StatusCreated, createRecB.Code)

	updateBody := `{"name":"Updated First","domain":"updated-first.io"}`
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/organization", strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	e.ServeHTTP(updateRec, updateReq)
	require.Equal(t, http.StatusOK, updateRec.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organization", nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var getResp map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	assert.NotEmpty(t, getResp["id"])
	assert.NotEmpty(t, firstID)
	assert.Equal(t, "Updated First", getResp["name"])
	assert.Equal(t, "updated-first.io", getResp["domain"])
}
