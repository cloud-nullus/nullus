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

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/port"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresOrgRepository_Integration(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresOrgRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	orgID := uuid.NewString()
	orgSlug := uniqueSlug("org")
	org := &domain.Organization{
		ID:        orgID,
		Name:      "Integration Org",
		Slug:      orgSlug,
		Domain:    "integration-org.test",
		Status:    domain.OrgStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	t.Run("create org and get by id", func(t *testing.T) {
		require.NoError(t, repo.Create(ctx, org))

		got, err := repo.GetByID(ctx, org.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, org.ID, got.ID)
		assert.Equal(t, org.Name, got.Name)
		assert.Equal(t, org.Slug, got.Slug)
		assert.Equal(t, org.Domain, got.Domain)
		assert.Equal(t, org.Status, got.Status)
		assert.Empty(t, got.DefaultAdminID)
		assert.NotNil(t, got.ClusterAccessScope)
	})

	t.Run("update org name and domain", func(t *testing.T) {
		org.Name = "Integration Org Updated"
		org.Domain = "updated-org.test"
		org.UpdatedAt = time.Now().UTC()

		require.NoError(t, repo.Update(ctx, org))

		got, err := repo.GetByID(ctx, org.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "Integration Org Updated", got.Name)
		assert.Equal(t, "updated-org.test", got.Domain)
	})

	t.Run("list orgs", func(t *testing.T) {
		before, err := repo.List(ctx, 1000, 0)
		require.NoError(t, err)

		orgA := buildOrganization(uniqueSlug("list-a"))
		orgB := buildOrganization(uniqueSlug("list-b"))
		require.NoError(t, repo.Create(ctx, orgA))
		require.NoError(t, repo.Create(ctx, orgB))

		after, err := repo.List(ctx, 1000, 0)
		require.NoError(t, err)
		assert.Equal(t, len(before)+2, len(after))
	})

	t.Run("get by slug", func(t *testing.T) {
		got, err := repo.GetBySlug(ctx, orgSlug)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, org.ID, got.ID)
		assert.Equal(t, orgSlug, got.Slug)
	})
}

func TestPostgresClusterRepository_Integration(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)

	orgRepo := NewPostgresOrgRepository(pool)
	clusterRepo := NewPostgresClusterRepository(pool)
	ctx := context.Background()

	org := buildOrganization(uniqueSlug("cluster-org"))
	require.NoError(t, orgRepo.Create(ctx, org))

	now := time.Now().UTC()
	cluster := &domain.Cluster{
		ID:               uuid.NewString(),
		Name:             "cluster-integration",
		Type:             domain.ClusterTypePipeline,
		Endpoint:         "https://cluster.integration.test",
		ConnectionStatus: domain.ConnectionStatusConnected,
		OrgID:            org.ID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	t.Run("create cluster and get by id", func(t *testing.T) {
		require.NoError(t, clusterRepo.Create(ctx, cluster))

		got, err := clusterRepo.GetByID(ctx, cluster.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, cluster.ID, got.ID)
		assert.Equal(t, cluster.Name, got.Name)
		assert.Equal(t, cluster.Type, got.Type)
		assert.Equal(t, cluster.Endpoint, got.Endpoint)
		assert.Equal(t, cluster.ConnectionStatus, got.ConnectionStatus)
		assert.Equal(t, cluster.OrgID, got.OrgID)
	})

	t.Run("list clusters by org id", func(t *testing.T) {
		list, err := clusterRepo.List(ctx, org.ID)
		require.NoError(t, err)
		assert.True(t, containsClusterID(list, cluster.ID))
	})

	t.Run("update cluster connection status", func(t *testing.T) {
		cluster.ConnectionStatus = domain.ConnectionStatusUnreachable
		cluster.UpdatedAt = time.Now().UTC()
		require.NoError(t, clusterRepo.Update(ctx, cluster))

		got, err := clusterRepo.GetByID(ctx, cluster.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.ConnectionStatusUnreachable, got.ConnectionStatus)
	})

	t.Run("node architectures round trip", func(t *testing.T) {
		// Task 3: Pre-Deploy Gate needs NodeArchitectures persisted per-cluster.
		// Caller may pass unsorted/duplicate values — repository must normalize.
		cluster.NodeArchitectures = []string{"arm64", "amd64", "arm64"}
		cluster.UpdatedAt = time.Now().UTC()
		require.NoError(t, clusterRepo.Update(ctx, cluster))

		got, err := clusterRepo.GetByID(ctx, cluster.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, []string{"amd64", "arm64"}, got.NodeArchitectures)

		// Clearing to empty should persist as empty slice and read back as nil.
		cluster.NodeArchitectures = nil
		cluster.UpdatedAt = time.Now().UTC()
		require.NoError(t, clusterRepo.Update(ctx, cluster))

		got, err = clusterRepo.GetByID(ctx, cluster.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Nil(t, got.NodeArchitectures)
	})

	t.Run("store and retrieve kubeconfig", func(t *testing.T) {
		encrypted := []byte("encrypted-kubeconfig-" + uuid.NewString())
		require.NoError(t, clusterRepo.SaveKubeconfig(ctx, cluster.ID, encrypted))

		got, err := clusterRepo.GetKubeconfig(ctx, cluster.ID)
		require.NoError(t, err)
		assert.Equal(t, encrypted, got)
	})

	t.Run("delete cluster", func(t *testing.T) {
		require.NoError(t, clusterRepo.Delete(ctx, cluster.ID))

		got, err := clusterRepo.GetByID(ctx, cluster.ID)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestPostgresUserRepository_Integration(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)

	orgRepo := NewPostgresOrgRepository(pool)
	userRepo := NewPostgresUserRepository(pool)
	ctx := context.Background()

	org := buildOrganization(uniqueSlug("user-org"))
	require.NoError(t, orgRepo.Create(ctx, org))

	user := &domain.User{
		ID:        uuid.NewString(),
		Email:     uniqueEmail("member"),
		Name:      "Integration User",
		Role:      domain.RoleDevOps,
		OrgID:     org.ID,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	t.Run("create user and get by id", func(t *testing.T) {
		require.NoError(t, userRepo.Create(ctx, user))

		got, err := userRepo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, user.ID, got.ID)
		assert.Equal(t, user.Email, got.Email)
		assert.Equal(t, user.Name, got.Name)
		assert.Equal(t, user.Role, got.Role)
		assert.Equal(t, user.OrgID, got.OrgID)
		assert.Equal(t, user.IsActive, got.IsActive)
	})

	t.Run("search by email finds existing user", func(t *testing.T) {
		got, err := userRepo.SearchByEmail(ctx, user.Email)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, user.ID, got.ID)
	})

	t.Run("search by email returns nil for unknown", func(t *testing.T) {
		got, err := userRepo.SearchByEmail(ctx, uniqueEmail("missing"))
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("list users by org", func(t *testing.T) {
		secondUser := &domain.User{
			ID:        uuid.NewString(),
			Email:     uniqueEmail("member2"),
			Name:      "Integration User 2",
			Role:      domain.RoleDeveloper,
			OrgID:     org.ID,
			IsActive:  true,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		require.NoError(t, userRepo.Create(ctx, secondUser))

		users, err := userRepo.ListByOrg(ctx, org.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(users), 2)
		assert.True(t, containsUserID(users, user.ID))
		assert.True(t, containsUserID(users, secondUser.ID))
	})
}

func TestPostgresUserMutationRepository_Integration(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)

	orgRepo := NewPostgresOrgRepository(pool)
	userRepo := NewPostgresUserRepository(pool)
	ctx := context.Background()

	targetOrg := buildOrganization(uniqueSlug("member-org"))
	sourceOrg := buildOrganization(uniqueSlug("source-org"))
	require.NoError(t, orgRepo.Create(ctx, targetOrg))
	require.NoError(t, orgRepo.Create(ctx, sourceOrg))

	user := &domain.User{
		ID:        uuid.NewString(),
		Email:     uniqueEmail("mutation"),
		Name:      "Mutation User",
		Role:      domain.RoleDeveloper,
		OrgID:     sourceOrg.ID,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, userRepo.Create(ctx, user))

	t.Run("add member to org", func(t *testing.T) {
		require.NoError(t, userRepo.AddMember(ctx, targetOrg.ID, user.ID, domain.RoleDevOps))

		isMember, err := userRepo.IsMember(ctx, targetOrg.ID, user.ID)
		require.NoError(t, err)
		assert.True(t, isMember)
	})

	t.Run("list members by org id", func(t *testing.T) {
		members, err := userRepo.ListByOrg(ctx, targetOrg.ID)
		require.NoError(t, err)
		assert.True(t, containsUserID(members, user.ID))
	})

	t.Run("update member role", func(t *testing.T) {
		user.Role = domain.RoleAdmin
		user.OrgID = targetOrg.ID
		user.UpdatedAt = time.Now().UTC()
		require.NoError(t, userRepo.Update(ctx, user))

		got, err := userRepo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.RoleAdmin, got.Role)
		assert.Equal(t, targetOrg.ID, got.OrgID)
	})

	t.Run("remove member from org", func(t *testing.T) {
		require.NoError(t, userRepo.Delete(ctx, user.ID))

		isMember, err := userRepo.IsMember(ctx, targetOrg.ID, user.ID)
		require.NoError(t, err)
		assert.False(t, isMember)
	})
}

func TestPostgresKnownIssuesRepository_Integration(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresKnownIssuesRepository(pool)
	ctx := context.Background()

	t.Run("list known issues", func(t *testing.T) {
		items, err := repo.List(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(items), 0)
	})

	t.Run("create known issue and verify in list", func(t *testing.T) {
		before, err := repo.List(ctx)
		require.NoError(t, err)

		title := "Integration issue " + uuid.NewString()
		_, err = pool.Exec(ctx, `
			INSERT INTO known_issues (id, severity, title, description, workaround, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
			uuid.NewString(),
			"medium",
			title,
			"integration test known issue",
			"none",
			"open",
		)
		require.NoError(t, err)

		after, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(before)+1, len(after))
		assert.True(t, containsIssueTitle(after, title))
	})
}

func buildOrganization(slug string) *domain.Organization {
	now := time.Now().UTC()
	return &domain.Organization{
		ID:        uuid.NewString(),
		Name:      "Org " + slug,
		Slug:      slug,
		Domain:    slug + ".test",
		Status:    domain.OrgStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func uniqueSlug(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))[:12])
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%s@integration.test", prefix, strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))[:12])
}

func containsClusterID(items []*domain.Cluster, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func containsUserID(items []*domain.User, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func containsIssueTitle(items []port.KnownIssue, title string) bool {
	for _, item := range items {
		if item.Title == title {
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
	if err != nil {
		cancel()
		t.Fatalf("start postgres container: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cancel()
		t.Fatalf("get container connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		cancel()
		t.Fatalf("create pgx pool: %v", err)
	}

	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		cancel()
		t.Fatalf("run migrations: %v", err)
	}

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
