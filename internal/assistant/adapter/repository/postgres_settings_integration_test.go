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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
)

var testKey = []byte("assistant-test-key-32bytes-pad!!")

// 키는 암호화해 저장하고 읽을 때 되살린다. 조직마다 하나다.
func TestPostgresSettingsRepository(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	repo := NewPostgresSettingsRepository(pool, testKey)

	got, err := repo.Get(ctx, "org-a")
	require.NoError(t, err)
	assert.Nil(t, got, "등록 전에는 없다")

	at := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.Save(ctx, domain.Settings{OrgID: "org-a", Enabled: true, Provider: domain.ProviderUpstage,
		Model: "solar-pro4", APIKey: "up_secret_value_1234", UpdatedAt: at, UpdatedBy: "admin"}))

	var stored string
	require.NoError(t, pool.QueryRow(ctx, `SELECT api_key_encrypted FROM assistant_settings WHERE org_id = 'org-a'`).Scan(&stored))
	assert.NotContains(t, stored, "up_secret_value_1234", "원문을 저장하지 않는다")

	got, err = repo.Get(ctx, "org-a")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "up_secret_value_1234", got.APIKey)
	assert.True(t, got.Enabled)
	assert.Equal(t, "solar-pro4", got.Model)
	assert.True(t, got.UpdatedAt.Equal(at))
	assert.Equal(t, "admin", got.UpdatedBy)

	// 다시 저장하면 덮어쓴다 — 키를 지우면 빈 값이다.
	require.NoError(t, repo.Save(ctx, domain.Settings{OrgID: "org-a", Provider: domain.ProviderAnthropic, UpdatedAt: at}))
	got, err = repo.Get(ctx, "org-a")
	require.NoError(t, err)
	assert.Equal(t, domain.ProviderAnthropic, got.Provider)
	assert.Empty(t, got.APIKey)

	other, err := repo.Get(ctx, "org-b")
	require.NoError(t, err)
	assert.Nil(t, other, "다른 조직의 설정을 보지 않는다")

	require.NoError(t, repo.Delete(ctx, "org-a"))
	got, err = repo.Get(ctx, "org-a")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// ENCRYPTION_KEY 가 없으면 키를 저장하지 않는다(평문으로 남기지 않는다). 키 없는 설정은 저장된다.
func TestPostgresSettingsRepository_RefusesKeyWithoutEncryptionKey(t *testing.T) {
	pool := setupPostgres(t)
	repo := NewPostgresSettingsRepository(pool, nil)
	err := repo.Save(context.Background(), domain.Settings{OrgID: "org-a", Provider: domain.ProviderUpstage, APIKey: "up_x"})
	assert.ErrorIs(t, err, ErrEncryptionKeyMissing)
	assert.NoError(t, repo.Save(context.Background(), domain.Settings{OrgID: "org-a",
		Provider: domain.ProviderOpenAICompatible, BaseURL: "http://vllm:8000/v1", Model: "m"}))
}

func setupPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("nullus"), postgres.WithUsername("nullus"), postgres.WithPassword("nullus_dev"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, runMigrations(ctx, pool))
	return pool
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "db", "migrations"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var ups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	for _, name := range ups {
		sql, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}
