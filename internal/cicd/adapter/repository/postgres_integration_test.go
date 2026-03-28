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

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresRepositories_CICDModuleIntegration(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	t.Run("pipeline repository create get list update and delete", func(t *testing.T) {
		repo := NewPostgresPipelineRepository(pool)

		orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
		otherOrgID, otherClusterID := createTestOrgAndCluster(t, ctx, pool)
		baseTime := time.Now().UTC().Truncate(time.Microsecond)

		pipeline := &domain.Pipeline{
			ID:         "pipeline-" + uuid.NewString(),
			Name:       "Pipeline Integration",
			TemplateID: "web-backend-v1",
			OrgID:      orgID,
			ClusterID:  clusterID,
			Namespace:  "default",
			AppType:    domain.AppTypeBackend,
			GitRepoURL: "https://github.com/cloud-nullus/draft",
			Status:     domain.PipelineStatusActive,
			CreatedAt:  baseTime,
		}

		require.NoError(t, repo.Create(ctx, pipeline))

		secondPipeline := &domain.Pipeline{
			ID:         "pipeline-" + uuid.NewString(),
			Name:       "Pipeline Same Org",
			TemplateID: "web-frontend-v1",
			OrgID:      orgID,
			ClusterID:  clusterID,
			Namespace:  "frontend",
			AppType:    domain.AppTypeWeb,
			GitRepoURL: "https://github.com/cloud-nullus/nullus-web",
			Status:     domain.PipelineStatusActive,
			CreatedAt:  baseTime.Add(time.Second),
		}
		require.NoError(t, repo.Create(ctx, secondPipeline))

		otherOrgPipeline := &domain.Pipeline{
			ID:         "pipeline-" + uuid.NewString(),
			Name:       "Pipeline Other Org",
			TemplateID: "batch-job-v1",
			OrgID:      otherOrgID,
			ClusterID:  otherClusterID,
			Namespace:  "batch",
			AppType:    domain.AppTypeBatch,
			GitRepoURL: "https://github.com/cloud-nullus/nullus-batch",
			Status:     domain.PipelineStatusActive,
			CreatedAt:  baseTime.Add(2 * time.Second),
		}
		require.NoError(t, repo.Create(ctx, otherOrgPipeline))

		got, err := repo.GetByID(ctx, pipeline.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, pipeline.ID, got.ID)
		assert.Equal(t, pipeline.Name, got.Name)
		assert.Equal(t, pipeline.OrgID, got.OrgID)
		assert.Equal(t, pipeline.Status, got.Status)

		list, err := repo.List(ctx, orgID)
		require.NoError(t, err)
		require.Len(t, list, 2)
		assert.Equal(t, secondPipeline.ID, list[0].ID)
		assert.Equal(t, pipeline.ID, list[1].ID)

		got.Status = domain.PipelineStatusInactive
		require.NoError(t, repo.Update(ctx, got))

		updated, err := repo.GetByID(ctx, pipeline.ID)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, domain.PipelineStatusInactive, updated.Status)

		_, err = pool.Exec(ctx, `DELETE FROM pipelines WHERE id = $1`, pipeline.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, pipeline.ID)
		require.Error(t, err)
	})

	t.Run("template repository list and get by id from seeded migrations", func(t *testing.T) {
		repo := NewPostgresCICDTemplateRepository(pool)

		templates, err := repo.List(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, templates)

		ids := make([]string, 0, len(templates))
		for _, tmpl := range templates {
			ids = append(ids, tmpl.ID)
		}
		assert.Contains(t, ids, "web-backend-v1")
		assert.Contains(t, ids, "web-frontend-v1")

		tmpl, err := repo.GetByID(ctx, "web-backend-v1")
		require.NoError(t, err)
		require.NotNil(t, tmpl)
		assert.Equal(t, "Web Backend Pipeline", tmpl.Name)
		assert.Equal(t, domain.AppTypeBackend, tmpl.AppType)
		assert.Contains(t, tmpl.Stages, "Build")
		assert.Contains(t, tmpl.Stages, "Deploy")
	})

	t.Run("deployment repository create and list by pipeline id in desc order", func(t *testing.T) {
		pipelineRepo := NewPostgresPipelineRepository(pool)
		repo := NewPostgresDeploymentRepository(pool)
		orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)

		pipelineID := "pipeline-" + uuid.NewString()
		require.NoError(t, pipelineRepo.Create(ctx, &domain.Pipeline{
			ID:         pipelineID,
			Name:       "Deployment Parent Pipeline",
			TemplateID: "web-backend-v1",
			OrgID:      orgID,
			ClusterID:  clusterID,
			Namespace:  "deploy-ns",
			AppType:    domain.AppTypeBackend,
			GitRepoURL: "https://github.com/cloud-nullus/draft",
			Status:     domain.PipelineStatusActive,
			CreatedAt:  time.Now().UTC().Truncate(time.Microsecond),
		}))

		startedAtOld := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
		startedAtNew := time.Now().UTC().Add(-1 * time.Minute).Truncate(time.Microsecond)

		oldDeployment := &domain.Deployment{
			ID:         "deployment-" + uuid.NewString(),
			PipelineID: pipelineID,
			Version:    "v1.0.0",
			Status:     domain.DeploymentStatusSuccess,
			StartedAt:  startedAtOld,
			DeployedBy: "integration-test",
		}
		newDeployment := &domain.Deployment{
			ID:         "deployment-" + uuid.NewString(),
			PipelineID: pipelineID,
			Version:    "v1.1.0",
			Status:     domain.DeploymentStatusRunning,
			StartedAt:  startedAtNew,
			DeployedBy: "integration-test",
		}

		require.NoError(t, repo.Create(ctx, oldDeployment))
		require.NoError(t, repo.Create(ctx, newDeployment))

		deployments, err := repo.ListByPipelineID(ctx, pipelineID)
		require.NoError(t, err)
		require.Len(t, deployments, 2)
		assert.Equal(t, newDeployment.ID, deployments[0].ID)
		assert.Equal(t, oldDeployment.ID, deployments[1].ID)
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

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
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

	if err := ensurePreMigrationTables(ctx, pool); err != nil {
		return err
	}

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
