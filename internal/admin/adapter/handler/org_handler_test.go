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

func newOrgEcho() (*echo.Echo, *adminhandler.OrgHandler) {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = middleware.AppErrorHandler

	orgRepo := adminrepo.NewMemoryOrgRepository()
	orgUC := usecase.NewOrgUseCase(orgRepo)
	h := adminhandler.NewOrgHandler(orgUC)

	v1 := e.Group("/api/v1")
	h.RegisterRoutes(v1)

	return e, h
}

func TestOrgHandler_CreateOrg_201(t *testing.T) {
	e, _ := newOrgEcho()

	body := `{"name":"Acme","slug":"acme","domain":"acme.io"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orgs", strings.NewReader(body))
	req.Header.Set(echo.MIMEApplicationJSON, "application/json")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "acme", data["slug"])
	assert.Equal(t, "Acme", data["name"])
}

func TestOrgHandler_GetOrg_200(t *testing.T) {
	e, _ := newOrgEcho()

	// Create first
	createBody := `{"name":"Beta Corp","slug":"beta-corp","domain":"beta.io"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/orgs", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	data := createResp["data"].(map[string]any)
	id := data["id"].(string)

	// Get by ID
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/orgs/"+id, nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)

	assert.Equal(t, http.StatusOK, getRec.Code)

	var getResp map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	gotData := getResp["data"].(map[string]any)
	assert.Equal(t, id, gotData["id"])
}

func TestOrgHandler_GetOrg_404(t *testing.T) {
	e, _ := newOrgEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orgs/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOrgHandler_UpdateOrg_200(t *testing.T) {
	e, _ := newOrgEcho()

	// Create first
	createBody := `{"name":"Original","slug":"original","domain":"original.io"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/orgs", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	id := createResp["data"].(map[string]any)["id"].(string)

	// Update
	updateBody := `{"name":"Updated Name","domain":"updated.io"}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/orgs/"+id, strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	e.ServeHTTP(updateRec, updateReq)

	assert.Equal(t, http.StatusOK, updateRec.Code)

	var updateResp map[string]any
	require.NoError(t, json.Unmarshal(updateRec.Body.Bytes(), &updateResp))
	updatedData := updateResp["data"].(map[string]any)
	assert.Equal(t, "Updated Name", updatedData["name"])
	assert.Equal(t, "updated.io", updatedData["domain"])
}
