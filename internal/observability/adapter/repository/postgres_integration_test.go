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

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresRepositories_ObservabilityModuleIntegration(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	t.Run("alert rule repository CRUD", func(t *testing.T) {
		repo := NewPostgresAlertRuleRepository(pool)

		ruleID := "rule-" + uuid.NewString()
		rule := &domain.AlertRule{
			ID:                ruleID,
			Name:              "High CPU",
			MetricName:        "cpu_usage",
			Condition:         "cpu_usage >= critical_threshold",
			WarningThreshold:  70,
			CriticalThreshold: 80,
			Threshold:         80,
			Channel:           domain.AlertChannelSlack,
			Enabled:           true,
		}

		require.NoError(t, repo.Create(ctx, rule))

		got, err := repo.GetByID(ctx, ruleID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, rule.ID, got.ID)
		assert.Equal(t, rule.Name, got.Name)
		assert.Equal(t, rule.MetricName, got.MetricName)
		assert.Equal(t, rule.Condition, got.Condition)
		assert.Equal(t, rule.WarningThreshold, got.WarningThreshold)
		assert.Equal(t, rule.CriticalThreshold, got.CriticalThreshold)
		assert.Equal(t, rule.Threshold, got.Threshold)
		assert.Equal(t, rule.Channel, got.Channel)
		assert.Equal(t, rule.Enabled, got.Enabled)

		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.True(t, containsAlertRuleID(list, ruleID))

		got.Name = "High CPU Updated"
		got.WarningThreshold = 75
		got.CriticalThreshold = 90
		got.Channel = domain.AlertChannelEmail
		got.Enabled = false
		require.NoError(t, repo.Update(ctx, got))

		updated, err := repo.GetByID(ctx, ruleID)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, "High CPU Updated", updated.Name)
		assert.Equal(t, 75.0, updated.WarningThreshold)
		assert.Equal(t, 90.0, updated.CriticalThreshold)
		assert.Equal(t, domain.AlertChannelEmail, updated.Channel)
		assert.False(t, updated.Enabled)

		require.NoError(t, repo.Delete(ctx, ruleID))

		_, err = repo.GetByID(ctx, ruleID)
		require.ErrorIs(t, err, domain.ErrAlertRuleNotFound)
	})

	t.Run("alert repository create and list in fired_at desc order", func(t *testing.T) {
		ruleRepo := NewPostgresAlertRuleRepository(pool)
		alertRepo := NewPostgresAlertRepository(pool)

		ruleID := "rule-" + uuid.NewString()
		require.NoError(t, ruleRepo.Create(ctx, &domain.AlertRule{
			ID:                ruleID,
			Name:              "Error Rate",
			MetricName:        "error_rate",
			Condition:         "error_rate >= critical_threshold",
			WarningThreshold:  3,
			CriticalThreshold: 5,
			Threshold:         5,
			Channel:           domain.AlertChannelSlack,
			Enabled:           true,
		}))

		older := &domain.Alert{
			ID:       "alert-" + uuid.NewString(),
			RuleID:   ruleID,
			Severity: domain.AlertSeverityWarning,
			Message:  "warning level",
			FiredAt:  time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond),
		}
		newer := &domain.Alert{
			ID:       "alert-" + uuid.NewString(),
			RuleID:   ruleID,
			Severity: domain.AlertSeverityCritical,
			Message:  "critical level",
			FiredAt:  time.Now().UTC().Add(-1 * time.Minute).Truncate(time.Microsecond),
		}

		require.NoError(t, alertRepo.Create(ctx, older))
		require.NoError(t, alertRepo.Create(ctx, newer))

		alerts, err := alertRepo.List(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(alerts), 2)
		assert.Equal(t, newer.ID, alerts[0].ID)
		assert.Equal(t, older.ID, alerts[1].ID)
	})

	t.Run("dashboard repository returns defensive copies", func(t *testing.T) {
		repo := NewMemoryDashboardRepository()

		first, err := repo.GetDashboard(ctx)
		require.NoError(t, err)
		require.NotNil(t, first)
		require.NotEmpty(t, first.ToolHealthList)

		first.ClusterMetrics.CPUUsage = 0
		first.ToolHealthList[0].Name = "Mutated"

		second, err := repo.GetDashboard(ctx)
		require.NoError(t, err)
		require.NotNil(t, second)

		assert.NotEqual(t, 0.0, second.ClusterMetrics.CPUUsage)
		assert.NotEqual(t, "Mutated", second.ToolHealthList[0].Name)
	})
}

func containsAlertRuleID(items []*domain.AlertRule, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
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
