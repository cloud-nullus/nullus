package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

func TestExportImport_RoundTripPreservesResourcesAndTools(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryStackRepository()
	templateRepo := repository.NewMemoryTemplateRepository()
	createUC := NewCreateStack(repo, templateRepo)
	addToolsUC := NewAddToolsUseCase(repo)
	exportUC := NewExportConfig(repo)
	importUC := NewImportConfig(createUC, addToolsUC)
	deleteUC := NewDeleteStack(repo, nil, nil)

	created, err := createUC.Execute(ctx, CreateStackInput{
		Name:      "devsecops-core",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus",
		Config: domain.StackConfig{
			Resources: domain.ResourcesConfig{
				DevCount:          12,
				ConcurrentRunners: 3,
				CommitsPerWeek:    40,
				BuildFrequency:    "daily",
			},
			AppliedResourceOverrides: map[string]domain.ResourceVector{
				"artifacts.packageRegistry:gitlab": {
					CPURequest: 1.5, CPULimit: 2.5, MemoryRequestGi: 3, MemoryLimitGi: 4, StorageRequestGi: 10, StorageLimitGi: 20,
				},
			},
			RowUnits: map[string]domain.PlanningRowUnit{
				"artifacts.packageRegistry:gitlab": {Memory: "Gi", Storage: "Gi"},
			},
		},
	})
	require.NoError(t, err)

	_, err = addToolsUC.Execute(ctx, AddToolsInput{
		StackID: created.Stack.ID,
		Tools:   []domain.ToolConfig{{Category: "artifacts", Tool: "gitlab", Version: "18.5.1"}},
	})
	require.NoError(t, err)

	exportedJSON, err := exportUC.ExportAsJSON(ctx, created.Stack.ID)
	require.NoError(t, err)

	var exported map[string]any
	require.NoError(t, json.Unmarshal(exportedJSON, &exported))
	require.Equal(t, "StackExport", exported["kind"])
	require.Equal(t, "stack.nullus.dev/v1alpha1", exported["apiVersion"])
	require.Contains(t, exported, "spec")

	spec := exported["spec"].(map[string]any)
	require.Equal(t, "v1", spec["schema_version"])
	require.Equal(t, float64(12), spec["resources"].(map[string]any)["developers"])
	require.Equal(t, float64(3), spec["resources"].(map[string]any)["concurrent_runners"])
	require.Equal(t, float64(40), spec["resources"].(map[string]any)["weekly_commits"])
	require.Equal(t, "daily", spec["resources"].(map[string]any)["build_frequency"])
	require.Equal(t, float64(1.5), spec["config"].(map[string]any)["applied_resource_overrides"].(map[string]any)["artifacts.packageRegistry:gitlab"].(map[string]any)["cpuRequest"])

	require.NoError(t, deleteUC.Execute(ctx, created.Stack.ID))
	_, err = repo.GetByID(ctx, created.Stack.ID)
	require.Error(t, err)

	imported, err := importUC.Execute(ctx, ImportConfigInput{OrgID: "org-1", Payload: exportedJSON})
	require.NoError(t, err)
	require.NotNil(t, imported.Stack)
	require.NotEqual(t, created.Stack.ID, imported.Stack.ID)

	restored, err := repo.GetByID(ctx, imported.Stack.ID)
	require.NoError(t, err)
	require.Len(t, restored.Tools, 1)
	require.Equal(t, "gitlab", restored.Tools[0].Tool)

	restoredCfg, ok := restored.Config.(domain.StackConfig)
	require.True(t, ok)
	require.Equal(t, 12, restoredCfg.Resources.DevCount)
	require.Equal(t, 3, restoredCfg.Resources.ConcurrentRunners)
	require.Equal(t, 40, restoredCfg.Resources.CommitsPerWeek)
	require.Equal(t, "daily", restoredCfg.Resources.BuildFrequency)
	require.Equal(t, 1.5, restoredCfg.AppliedResourceOverrides["artifacts.packageRegistry:gitlab"].CPURequest)
	require.Equal(t, "Gi", restoredCfg.RowUnits["artifacts.packageRegistry:gitlab"].Memory)
}

func TestImportPreviewAndApply_UpdatesExistingStack(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryStackRepository()
	templateRepo := repository.NewMemoryTemplateRepository()
	createUC := NewCreateStack(repo, templateRepo)
	addToolsUC := NewAddToolsUseCase(repo)
	exportUC := NewExportConfig(repo)
	importUC := NewImportConfig(createUC, addToolsUC)

	created, err := createUC.Execute(ctx, CreateStackInput{
		Name:      "nullus-devsecops-stack",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus",
		Config: domain.StackConfig{
			Resources: domain.ResourcesConfig{DevCount: 10, ConcurrentRunners: 5, CommitsPerWeek: 50, BuildFrequency: "medium"},
		},
	})
	require.NoError(t, err)

	created.Stack.Config = domain.StackConfig{
		Resources: domain.ResourcesConfig{DevCount: 20, ConcurrentRunners: 8, CommitsPerWeek: 90, BuildFrequency: "daily"},
		AppliedResourceOverrides: map[string]domain.ResourceVector{
			"pipeline.cdTool:argocd": {CPURequest: 0.8, CPULimit: 1.2},
		},
	}
	created.Stack.Tools = []domain.ToolConfig{{Category: "pipeline.cdTool", Tool: "argocd", Version: "v2.8.3"}}
	require.NoError(t, repo.Update(ctx, created.Stack))
	require.NoError(t, repo.UpdateTools(ctx, created.Stack))

	exportPayload, err := exportUC.ExportAsJSON(ctx, created.Stack.ID)
	require.NoError(t, err)
	var exported map[string]any
	require.NoError(t, json.Unmarshal(exportPayload, &exported))
	specMap := exported["spec"].(map[string]any)
	configMap := specMap["config"].(map[string]any)
	configMap["applied_resource_overrides"].(map[string]any)["pipeline.cdTool:argocd"].(map[string]any)["cpuRequest"] = 1.4
	exportPayload, err = json.Marshal(exported)
	require.NoError(t, err)

	preview, err := importUC.Preview(ctx, ImportConfigInput{OrgID: "org-1", Payload: exportPayload})
	require.NoError(t, err)
	require.Equal(t, "update", preview.Mode)
	require.Equal(t, created.Stack.ID, preview.ExistingStackID)
	require.NotNil(t, preview.Changes)

	_, err = importUC.Execute(ctx, ImportConfigInput{OrgID: "org-1", Payload: exportPayload})
	require.ErrorIs(t, err, ErrImportConfirmationRequired)

	out, err := importUC.Execute(ctx, ImportConfigInput{OrgID: "org-1", Payload: exportPayload, ReplaceExisting: true})
	require.NoError(t, err)
	require.Equal(t, created.Stack.ID, out.Stack.ID)
	restored, err := repo.GetByID(ctx, created.Stack.ID)
	require.NoError(t, err)
	restoredCfg, ok := restored.Config.(domain.StackConfig)
	require.True(t, ok)
	require.Equal(t, 20, restoredCfg.Resources.DevCount)
	require.Equal(t, 1.4, restoredCfg.AppliedResourceOverrides["pipeline.cdTool:argocd"].CPURequest)
}

func TestImportExecute_CreatesNewStack(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryStackRepository()
	templateRepo := repository.NewMemoryTemplateRepository()
	createUC := NewCreateStack(repo, templateRepo)
	addToolsUC := NewAddToolsUseCase(repo)
	exportUC := NewExportConfig(repo)
	importUC := NewImportConfig(createUC, addToolsUC)

	created, err := createUC.Execute(ctx, CreateStackInput{
		Name:      "source-stack",
		OrgID:     "org-1",
		ClusterID: "cluster-1",
		Namespace: "nullus",
		Config: domain.StackConfig{
			Resources: domain.ResourcesConfig{DevCount: 5, ConcurrentRunners: 2, CommitsPerWeek: 20, BuildFrequency: "daily"},
		},
	})
	require.NoError(t, err)
	_, err = addToolsUC.Execute(ctx, AddToolsInput{StackID: created.Stack.ID, Tools: []domain.ToolConfig{{Category: "pipeline.cdTool", Tool: "argocd", Version: "v2.8.3"}}})
	require.NoError(t, err)
	payload, err := exportUC.ExportAsJSON(ctx, created.Stack.ID)
	require.NoError(t, err)

	var exported map[string]any
	require.NoError(t, json.Unmarshal(payload, &exported))
	spec := exported["spec"].(map[string]any)
	spec["name"] = "import-created-stack"
	mutated, err := json.Marshal(exported)
	require.NoError(t, err)

	out, err := importUC.Execute(ctx, ImportConfigInput{OrgID: "org-1", Payload: mutated})
	require.NoError(t, err)
	require.NotEqual(t, created.Stack.ID, out.Stack.ID)
	restored, err := repo.GetByID(ctx, out.Stack.ID)
	require.NoError(t, err)
	require.Equal(t, "import-created-stack", restored.Name)
}
