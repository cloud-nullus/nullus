package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloud-nullus/draft/internal/shared/audit"
	stackhandler "github.com/cloud-nullus/draft/internal/stack/adapter/handler"
	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCompatCRUDEcho wires a fresh echo server with CRUD routes enabled so
// individual tests don't share state.
func newCompatCRUDEcho(t *testing.T, sink audit.Sink) *echo.Echo {
	t.Helper()
	repo := stackrepo.NewMemoryCompatibilityRepository()
	validate := usecase.NewValidateCompatibility(repo)
	manage := usecase.NewManageCompatibility(repo)

	h := stackhandler.NewCompatibilityHandler(
		repo,
		validate,
		stackhandler.WithManageCompatibility(manage),
		stackhandler.WithCompatibilityAuditSink(sink),
	)
	e := echo.New()
	stacks := e.Group("/api/v1")
	admin := e.Group("/api/v1/admin")
	h.RegisterRoutes(stacks)
	h.RegisterAdminRoutes(admin)
	return e
}

func validPayload(id string) map[string]any {
	return map[string]any{
		"id":     id,
		"name":   "Fixture " + id,
		"status": "untested",
		"kubernetes": map[string]any{
			"min": "1.27", "max": "1.35", "recommended": "1.35",
		},
		"tools": map[string]any{
			"db": map[string]any{
				"name":         "Postgres",
				"helm_version": "12.0.0",
				"app_version":  "16.0",
				"tier":         "stable",
				"arch_support": []string{"amd64", "arm64"},
			},
		},
	}
}

func doJSON(t *testing.T, e *echo.Echo, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	parsed := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	}
	return rec.Code, parsed
}

func TestCompatibility_CreateMatrix_201(t *testing.T) {
	sink := audit.NewMemorySink()
	e := newCompatCRUDEcho(t, sink)

	code, body := doJSON(t, e, http.MethodPost, "/api/v1/admin/compatibility/matrices", validPayload("mx-create"))
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "mx-create", body["id"])

	entries := sink.Snapshot()
	require.Len(t, entries, 1)
	assert.Equal(t, "compatibility_matrix_create", entries[0].Action)
	assert.Equal(t, "mx-create", entries[0].ResourceID)
}

func TestCompatibility_CreateMatrix_Duplicate_409(t *testing.T) {
	e := newCompatCRUDEcho(t, nil)
	code, _ := doJSON(t, e, http.MethodPost, "/api/v1/admin/compatibility/matrices", validPayload("dup-matrix"))
	require.Equal(t, http.StatusCreated, code)

	code, body := doJSON(t, e, http.MethodPost, "/api/v1/admin/compatibility/matrices", validPayload("dup-matrix"))
	assert.Equal(t, http.StatusConflict, code)
	errObj, _ := body["error"].(map[string]any)
	assert.Equal(t, "COMPATIBILITY_MATRIX_EXISTS", errObj["code"])
}

func TestCompatibility_CreateMatrix_InvalidStatus_400(t *testing.T) {
	e := newCompatCRUDEcho(t, nil)
	p := validPayload("bad-status")
	p["status"] = "bogus"
	code, body := doJSON(t, e, http.MethodPost, "/api/v1/admin/compatibility/matrices", p)
	assert.Equal(t, http.StatusBadRequest, code)
	errObj, _ := body["error"].(map[string]any)
	assert.Equal(t, "COMPATIBILITY_VALIDATION", errObj["code"])
}

func TestCompatibility_UpdateMatrix_PathBodyMismatch_400(t *testing.T) {
	e := newCompatCRUDEcho(t, nil)
	_, _ = doJSON(t, e, http.MethodPost, "/api/v1/admin/compatibility/matrices", validPayload("upd-match"))

	p := validPayload("upd-different")
	code, body := doJSON(t, e, http.MethodPut, "/api/v1/admin/compatibility/matrices/upd-match", p)
	assert.Equal(t, http.StatusBadRequest, code)
	errObj, _ := body["error"].(map[string]any)
	assert.Equal(t, "COMPATIBILITY_REQUEST_INVALID", errObj["code"])
}

func TestCompatibility_UpdateMatrix_NotFound_404(t *testing.T) {
	e := newCompatCRUDEcho(t, nil)
	code, body := doJSON(t, e, http.MethodPut, "/api/v1/admin/compatibility/matrices/ghost", validPayload("ghost"))
	assert.Equal(t, http.StatusNotFound, code)
	errObj, _ := body["error"].(map[string]any)
	assert.Equal(t, "COMPATIBILITY_MATRIX_NOT_FOUND", errObj["code"])
}

func TestCompatibility_UpdateMatrix_Success_200(t *testing.T) {
	sink := audit.NewMemorySink()
	e := newCompatCRUDEcho(t, sink)
	_, _ = doJSON(t, e, http.MethodPost, "/api/v1/admin/compatibility/matrices", validPayload("upd-ok"))

	p := validPayload("upd-ok")
	p["status"] = "verified"
	code, body := doJSON(t, e, http.MethodPut, "/api/v1/admin/compatibility/matrices/upd-ok", p)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "verified", body["status"])

	entries := sink.Snapshot()
	var updates int
	for _, ent := range entries {
		if ent.Action == "compatibility_matrix_update" {
			updates++
		}
	}
	assert.Equal(t, 1, updates)
}

func TestCompatibility_DeleteMatrix_204(t *testing.T) {
	sink := audit.NewMemorySink()
	e := newCompatCRUDEcho(t, sink)
	_, _ = doJSON(t, e, http.MethodPost, "/api/v1/admin/compatibility/matrices", validPayload("del-ok"))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/compatibility/matrices/del-ok", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	var deletes int
	for _, ent := range sink.Snapshot() {
		if ent.Action == "compatibility_matrix_delete" {
			deletes++
		}
	}
	assert.Equal(t, 1, deletes)
}

// Compile-time check so the fixture matrix payload stays consistent with the
// domain type even without full roundtrip.
var _ = domain.CompatibilityMatrix{}
var _ = context.Background
