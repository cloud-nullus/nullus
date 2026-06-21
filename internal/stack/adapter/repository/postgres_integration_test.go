//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresRepositories_StackModuleIntegration(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	t.Run("stack repository CRUD and filtering", func(t *testing.T) {
		repo := NewPostgresStackRepository(pool)

		orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
		otherOrgID, otherClusterID := createTestOrgAndCluster(t, ctx, pool)

		stackID := "stack-" + uuid.NewString()
		now := time.Now().UTC().Truncate(time.Microsecond)
		stack := &domain.Stack{
			ID:         stackID,
			Name:       "Integration Stack",
			TemplateID: "gitlab-allinone-v1",
			OrgID:      orgID,
			ClusterID:  clusterID,
			Namespace:  "nullus-int",
			State:      domain.StatePending,
			Config:     sampleStackConfig("GitLab CE"),
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		require.NoError(t, repo.Create(ctx, stack))

		got, err := repo.GetByID(ctx, stackID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, stack.ID, got.ID)
		assert.Equal(t, stack.Name, got.Name)
		assert.Equal(t, stack.TemplateID, got.TemplateID)
		assert.Equal(t, stack.OrgID, got.OrgID)
		assert.Equal(t, stack.ClusterID, got.ClusterID)
		assert.Equal(t, stack.Namespace, got.Namespace)
		assert.Equal(t, stack.State, got.State)

		cfg, ok := got.Config.(domain.StackConfig)
		require.True(t, ok)
		assert.Equal(t, "GitLab CE", cfg.Artifacts.SourceRepository.Name)

		otherStack := &domain.Stack{
			ID:         "stack-" + uuid.NewString(),
			Name:       "Other Org Stack",
			TemplateID: "gitlab-allinone-v1",
			OrgID:      otherOrgID,
			ClusterID:  otherClusterID,
			Namespace:  "nullus-other",
			State:      domain.StatePending,
			Config:     sampleStackConfig("GitHub"),
			CreatedAt:  time.Now().UTC().Truncate(time.Microsecond),
			UpdatedAt:  time.Now().UTC().Truncate(time.Microsecond),
		}
		require.NoError(t, repo.Create(ctx, otherStack))

		list, err := repo.List(ctx, orgID, false)
		require.NoError(t, err)
		assert.NotEmpty(t, list)
		assert.True(t, containsStackID(list, stackID))
		assert.False(t, containsStackID(list, otherStack.ID))

		got.State = domain.StateInstalling
		require.NoError(t, repo.Update(ctx, got))

		updated, err := repo.GetByID(ctx, stackID)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, domain.StateInstalling, updated.State)

		require.NoError(t, repo.Delete(ctx, stackID))

		listAfterDelete, err := repo.List(ctx, orgID, false)
		require.NoError(t, err)
		assert.False(t, containsStackID(listAfterDelete, stackID))

		deleted, err := repo.FindByID(ctx, stackID)
		require.NoError(t, err)
		assert.Nil(t, deleted)

		notFound, err := repo.FindByID(ctx, "stack-"+uuid.NewString())
		require.NoError(t, err)
		assert.Nil(t, notFound)
	})

	t.Run("template repository seeded templates and tools decoding", func(t *testing.T) {
		repo := NewPostgresTemplateRepository(pool)

		templates, err := repo.List(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, templates)

		ids := make([]string, 0, len(templates))
		for _, tmpl := range templates {
			ids = append(ids, tmpl.ID)
		}
		assert.Contains(t, ids, "gitlab-allinone-v1")

		tmpl, err := repo.GetByID(ctx, "gitlab-allinone-v1")
		require.NoError(t, err)
		require.NotNil(t, tmpl)
		require.NotEmpty(t, tmpl.Tools)

		tool := tmpl.Tools[0]
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.HelmVersion)
		assert.NotEmpty(t, tool.AppVersion)
	})

	t.Run("history repository save list and get versions", func(t *testing.T) {
		repo := NewPostgresHistoryRepository(pool)

		orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
		stackRepo := NewPostgresStackRepository(pool)
		stackID := "stack-" + uuid.NewString()
		now := time.Now().UTC().Truncate(time.Microsecond)
		require.NoError(t, stackRepo.Create(ctx, &domain.Stack{
			ID:         stackID,
			Name:       "History Stack",
			TemplateID: "gitlab-allinone-v1",
			OrgID:      orgID,
			ClusterID:  clusterID,
			Namespace:  "history-ns",
			State:      domain.StatePending,
			Config:     sampleStackConfig("GitLab CE"),
			CreatedAt:  now,
			UpdatedAt:  now,
		}))

		older := &domain.StackVersion{
			ID:           "ver-" + uuid.NewString(),
			StackID:      stackID,
			Version:      2,
			Config:       sampleStackConfig("GitLab CE"),
			ChangedBy:    "test-user",
			ChangeReason: "initial",
			CreatedAt:    now.Add(-2 * time.Minute),
		}
		newer := &domain.StackVersion{
			ID:           "ver-" + uuid.NewString(),
			StackID:      stackID,
			Version:      1,
			Config:       sampleStackConfig("GitHub"),
			ChangedBy:    "test-user",
			ChangeReason: "updated",
			CreatedAt:    now.Add(-1 * time.Minute),
		}

		require.NoError(t, repo.SaveVersion(ctx, older))
		require.NoError(t, repo.SaveVersion(ctx, newer))

		versions, err := repo.ListVersions(ctx, stackID)
		require.NoError(t, err)
		require.Len(t, versions, 2)
		assert.Equal(t, newer.ID, versions[0].ID)

		require.NoError(t, stackRepo.Delete(ctx, stackID))

		versionsAfterDelete, err := repo.ListVersions(ctx, stackID)
		require.NoError(t, err)
		require.Len(t, versionsAfterDelete, 0)

		got, err := repo.GetVersion(ctx, stackID, older.ID)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("compatibility repository seeded data", func(t *testing.T) {
		repo := NewPostgresCompatibilityRepository(pool)

		matrices, err := repo.GetAll(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, matrices)
		assert.True(t, containsCompatibilityID(matrices, "gitlab-allinone-v1"))

		matrix, err := repo.GetByID(ctx, "gitlab-allinone-v1")
		require.NoError(t, err)
		require.NotNil(t, matrix)
		cdTool, ok := matrix.Tools["cd_tool"]
		require.True(t, ok)
		assert.NotEmpty(t, cdTool.Name)
		assert.NotEmpty(t, cdTool.AppVersion)
	})

	t.Run("resource default repository list known keys and upsert update", func(t *testing.T) {
		repo := NewPostgresResourceDefaultRepository(pool)

		items, err := repo.List(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, items)

		gitlab, ok := findResourceByToolKey(items, "gitlab-ce")
		require.True(t, ok)
		assert.Equal(t, "GitLab CE", gitlab.DisplayName)

		argocd, ok := findResourceByToolKey(items, "argocd")
		require.True(t, ok)
		assert.Equal(t, "Argo CD", argocd.DisplayName)

		toolKey := "tool-" + uuid.NewString()
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM stack_resource_defaults WHERE tool_key = $1`, toolKey)
		})

		resource := &domain.ResourceDefault{
			ToolKey:          toolKey,
			DisplayName:      "Custom Tool",
			CPURequest:       0.5,
			CPULimit:         1.0,
			MemoryRequestGi:  1.0,
			MemoryLimitGi:    2.0,
			StorageRequestGi: 3.0,
			StorageLimitGi:   4.0,
			IsDefault:        false,
		}
		require.NoError(t, repo.Upsert(ctx, resource))

		resource.CPURequest = 1.5
		resource.DisplayName = "Custom Tool Updated"
		require.NoError(t, repo.Upsert(ctx, resource))

		updatedItems, err := repo.List(ctx)
		require.NoError(t, err)
		updated, ok := findResourceByToolKey(updatedItems, toolKey)
		require.True(t, ok)
		assert.Equal(t, "Custom Tool Updated", updated.DisplayName)
		assert.Equal(t, 1.5, updated.CPURequest)
	})

	t.Run("helm step metadata repository CRUD", func(t *testing.T) {
		repo := NewPostgresHelmStepMetadataRepository(pool)

		stepName := "installing_test_chart_" + uuid.NewString()
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM stack_helm_step_configs WHERE step_name = $1`, stepName)
		})

		now := time.Now().UTC().Truncate(time.Microsecond)
		item := &domain.HelmStepMetadata{
			StepName:    stepName,
			ReleaseName: "test-release",
			ChartName:   "test-chart",
			RepoURL:     "https://example.com/charts",
			Version:     "1.2.3",
			Namespace:   "test-ns",
			Phase:       "B",
			SortOrder:   99,
			Wait:        true,
			IsEnabled:   true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		require.NoError(t, repo.Create(ctx, item))

		got, err := repo.GetByStep(ctx, stepName)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, item.ChartName, got.ChartName)
		assert.Equal(t, item.RepoURL, got.RepoURL)

		item.Version = "2.0.0"
		item.Wait = false
		require.NoError(t, repo.Update(ctx, item))

		updated, err := repo.GetByStep(ctx, stepName)
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", updated.Version)
		assert.False(t, updated.Wait)

		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.True(t, containsHelmStepMetadata(list, stepName))

		require.NoError(t, repo.Delete(ctx, stepName))
		deleted, err := repo.GetByStep(ctx, stepName)
		require.Error(t, err)
		assert.Nil(t, deleted)
	})
}

func setupPostgres(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

	container, err := postgres.Run(ctx,
		"postgres:18",
		postgres.WithDatabase("nullus"),
		postgres.WithUsername("nullus"),
		postgres.WithPassword("nullus_dev"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	require.NoError(t, runMigrations(ctx, pool))

	cleanup := func() {
		pool.Close()
		_ = container.Terminate(context.Background())
		cancel()
	}

	return pool, cleanup
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("determine caller file")
	}

	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))))
	migrationsDir := filepath.Join(repoRoot, "db", "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migration directory: %w", err)
	}

	upFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			upFiles = append(upFiles, name)
		}
	}
	sort.Strings(upFiles)

	for _, filename := range upFiles {
		path := filepath.Join(migrationsDir, filename)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}
	}

	return nil
}

func ensurePreMigrationTables(ctx context.Context, pool *pgxpool.Pool) error {
	const q = `
		CREATE TABLE IF NOT EXISTS golden_path_templates (
			id VARCHAR(100) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			tools JSONB NOT NULL DEFAULT '[]',
			estimated_install_time BIGINT NOT NULL DEFAULT 0,
			recommended_use_case TEXT,
			min_resources TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`

	if _, err := pool.Exec(ctx, q); err != nil {
		return fmt.Errorf("ensure golden_path_templates table: %w", err)
	}

	return nil
}

func createTestOrgAndCluster(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (string, string) {
	t.Helper()

	orgID := uuid.NewString()
	slug := "org-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug, domain, status) VALUES ($1, $2, $3, $4, $5)`,
		orgID,
		"Integration Org",
		slug,
		slug+".integration.test",
		"active",
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	var clusterID string
	err = pool.QueryRow(ctx,
		`INSERT INTO clusters (name, type, endpoint, connection_status, org_id) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		"cluster-"+slug,
		"pipeline",
		"https://k8s.integration.test",
		"connected",
		orgID,
	).Scan(&clusterID)
	require.NoError(t, err)

	return orgID, clusterID
}

func sampleStackConfig(sourceRepoName string) domain.StackConfig {
	return domain.StackConfig{
		Artifacts: domain.ArtifactsConfig{
			PackageRegistry:   domain.ToolSelection{Name: "GitLab Registry", Version: "17.7.2", Enabled: true},
			SourceRepository:  domain.ToolSelection{Name: sourceRepoName, Version: "17.7.2", Enabled: true},
			ContainerRegistry: domain.ToolSelection{Name: "Harbor", Version: "2.11.0", Enabled: true},
			StorageBackend:    domain.ToolSelection{Name: "MinIO", Version: "2024.11.7", Enabled: true},
		},
		Pipeline: domain.PipelineConfig{
			CIPlatform: domain.ToolSelection{Name: "GitLab CI", Version: "17.7.2", Enabled: true},
			CDTool:     domain.ToolSelection{Name: "Argo CD", Version: "2.13.2", Enabled: true},
		},
		Monitoring: domain.MonitoringConfig{
			Collection:    domain.ToolSelection{Name: "Prometheus", Version: "3.1.0", Enabled: true},
			Visualization: domain.ToolSelection{Name: "Grafana", Version: "11.4.0", Enabled: true},
		},
		Logging: domain.LoggingConfig{
			Collection: domain.ToolSelection{Name: "OpenSearch", Version: "2.17.0", Enabled: true},
			Search:     domain.ToolSelection{Name: "OpenSearch", Version: "2.17.0", Enabled: true},
		},
		Resources: domain.ResourcesConfig{
			DevCount:          3,
			ConcurrentRunners: 2,
			CommitsPerWeek:    20,
			BuildFrequency:    "daily",
		},
	}
}

func containsStackID(stacks []*domain.Stack, id string) bool {
	for _, stack := range stacks {
		if stack.ID == id {
			return true
		}
	}
	return false
}

func containsCompatibilityID(matrices []*domain.CompatibilityMatrix, id string) bool {
	for _, matrix := range matrices {
		if matrix.ID == id {
			return true
		}
	}
	return false
}

func findResourceByToolKey(resources []*domain.ResourceDefault, toolKey string) (*domain.ResourceDefault, bool) {
	for _, resource := range resources {
		if resource.ToolKey == toolKey {
			return resource, true
		}
	}
	return nil, false
}

func containsHelmStepMetadata(items []*domain.HelmStepMetadata, stepName string) bool {
	for _, item := range items {
		if item != nil && item.StepName == stepName {
			return true
		}
	}
	return false
}
