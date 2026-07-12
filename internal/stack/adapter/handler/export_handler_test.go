package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	stackuc "github.com/cloud-nullus/draft/internal/stack/usecase"
)

func TestExportHandler_ExportDeleteAndImport_RoundTrip(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	stacks := v1.Group("/stacks")

	repo := stackrepo.NewMemoryStackRepository()
	templateRepo := stackrepo.NewMemoryTemplateRepository()
	createUC := stackuc.NewCreateStack(repo, templateRepo)
	addToolsUC := stackuc.NewAddToolsUseCase(repo)
	deleteUC := stackuc.NewDeleteStack(repo, nil, nil)
	exportUC := stackuc.NewExportConfig(repo)
	importUC := stackuc.NewImportConfig(createUC, addToolsUC)
	stackHandler := NewStackHandler(createUC, stackuc.NewListStacks(repo), deleteUC, addToolsUC, repo, nil)
	exportHandler := NewExportHandler(exportUC, importUC)

	stackHandler.RegisterRoutes(stacks)
	exportHandler.RegisterRoutes(v1)

	created, err := createUC.Execute(t.Context(), stackuc.CreateStackInput{
		Name:      "devsecops-core",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus",
		Config: domain.StackConfig{
			Resources: domain.ResourcesConfig{DevCount: 9, ConcurrentRunners: 2, CommitsPerWeek: 28, BuildFrequency: "daily"},
		},
	})
	require.NoError(t, err)
	_, err = addToolsUC.Execute(t.Context(), stackuc.AddToolsInput{
		StackID: created.Stack.ID,
		Tools:   []domain.ToolConfig{{Category: "pipeline", Tool: "argo-cd", Version: "2.11.0"}},
	})
	require.NoError(t, err)

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/stacks/"+created.Stack.ID+"/export?format=json", nil)
	exportReq.Header.Set("X-Org-ID", "org-1")
	exportRec := httptest.NewRecorder()
	e.ServeHTTP(exportRec, exportReq)
	require.Equal(t, http.StatusOK, exportRec.Code)
	require.Contains(t, exportRec.Body.String(), `"kind": "StackExport"`)
	require.Contains(t, exportRec.Body.String(), `"apiVersion": "stack.nullus.dev/v1alpha1"`)
	require.Contains(t, exportRec.Header().Get("Content-Disposition"), created.Stack.ID)
	require.Contains(t, exportRec.Header().Get("Content-Disposition"), ".json")
	require.Contains(t, exportRec.Body.String(), `"spec"`)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/stacks/"+created.Stack.ID, nil)
	deleteReq.Header.Set("X-Org-ID", "org-1")
	deleteRec := httptest.NewRecorder()
	e.ServeHTTP(deleteRec, deleteReq)
	require.Equal(t, http.StatusNoContent, deleteRec.Code)

	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/stacks/import", bytes.NewReader(exportRec.Body.Bytes()))
	importReq.Header.Set("X-Org-ID", "org-1")
	importReq.Header.Set("Content-Type", "application/json")
	importRec := httptest.NewRecorder()
	e.ServeHTTP(importRec, importReq)
	require.Equal(t, http.StatusCreated, importRec.Code)

	var importResp map[string]any
	require.NoError(t, json.Unmarshal(importRec.Body.Bytes(), &importResp))
	importedID, ok := importResp["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, importedID)
	require.NotEqual(t, created.Stack.ID, importedID)

	restored, err := repo.GetByID(t.Context(), importedID)
	require.NoError(t, err)
	require.Len(t, restored.Tools, 1)
	require.Equal(t, "argo-cd", restored.Tools[0].Tool)
	restoredCfg, ok := restored.Config.(domain.StackConfig)
	require.True(t, ok)
	require.Equal(t, 9, restoredCfg.Resources.DevCount)
	require.Equal(t, 2, restoredCfg.Resources.ConcurrentRunners)
	_, err = repo.GetByID(t.Context(), created.Stack.ID)
	require.Error(t, err)
}
