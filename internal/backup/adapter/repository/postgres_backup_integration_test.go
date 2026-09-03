//go:build integration

// 실제 PostgreSQL 로 저장 어댑터를 검증한다.
//
// 단위 테스트로는 잡을 수 없는 것들이 여기 있다 — JSONB 왕복, TEXT[] 매핑,
// NULL 컬럼 처리, ON DELETE CASCADE. 이 계층에서 조용히 깨지면 백업 이력이
// 남지 않거나 매니페스트가 빈 채로 저장되고, 그 사실은 복구할 때에야 드러난다.
//
// 실행: go test -tags integration ./internal/backup/adapter/repository/
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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

// stack ID 는 UUID 가 아니다 — stacks.id 는 `stk_<hex>` 형태의 짧은 식별자다.
// 여기에 uuid 를 넣어 두는 바람에 stack_id 컬럼이 UUID 로 선언된 것을 아무도
// 잡지 못했고, 실제 스택으로 백업을 돌려 보고서야 드러났다(000076).
const realisticStackID = "stk_352e9f68db06"

func newRun(orgID string) *domain.BackupRun {
	run := domain.NewBackupRun(orgID, domain.ModeFull, domain.TriggerManual, domain.ModeComponents(domain.ModeFull))
	run.StackID = realisticStackID
	return run
}

func TestPostgresBackupRepository_실행_왕복(t *testing.T) {
	pool, cleanup := setupBackupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresBackupRepository(pool)
	ctx := context.Background()
	orgID := uuid.NewString()

	run := newRun(orgID)
	require.NoError(t, repo.CreateRun(ctx, run))
	require.NotEmpty(t, run.ID, "DB 가 ID 를 채워야 한다")
	assert.False(t, run.CreatedAt.IsZero())

	require.NoError(t, run.Start())
	run.SchemaVersion = 75
	run.BeginQuiesce()
	run.EndQuiesce()
	run.Manifest = map[string]any{
		"schema_version": 75,
		"volumes":        []any{map[string]any{"name": "data-gitlab", "size_bytes": 100}},
	}
	run.TotalBytes = 4096
	run.Complete(map[domain.Component]error{domain.ComponentPlatformDB: nil})
	require.NoError(t, repo.UpdateRun(ctx, run))

	got, err := repo.GetRun(ctx, run.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.StatusSucceeded, got.Status)
	assert.Equal(t, 75, got.SchemaVersion)
	assert.Equal(t, int64(4096), got.TotalBytes)
	assert.ElementsMatch(t, domain.ModeComponents(domain.ModeFull), got.Scope, "TEXT[] 왕복")
	require.NotNil(t, got.QuiesceStartedAt)
	require.NotNil(t, got.QuiesceEndedAt)
	assert.EqualValues(t, 75, got.Manifest["schema_version"], "JSONB 왕복")
}

// 실제 스택 ID 형식(stk_*)이 그대로 저장·조회되는지 본다.
// UUID 컬럼이면 여기서 "invalid input syntax for type uuid" 로 죽는다.
func TestPostgresBackupRepository_스택_ID_는_UUID_가_아니다(t *testing.T) {
	pool, cleanup := setupBackupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresBackupRepository(pool)
	ctx := context.Background()

	run := newRun(uuid.NewString())
	require.NoError(t, repo.CreateRun(ctx, run), "stk_* 형식이 저장돼야 한다")

	got, err := repo.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, realisticStackID, got.StackID, "형식이 보존돼야 한다")
}

func TestPostgresBackupRepository_stack_id_가_없어도_된다(t *testing.T) {
	// 플랫폼 전용 백업은 스택이 없다. NOT NULL 로 두면 그 경로가 막힌다.
	pool, cleanup := setupBackupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresBackupRepository(pool)
	ctx := context.Background()

	run := domain.NewBackupRun(uuid.NewString(), domain.ModePlatformOnly, domain.TriggerSchedule,
		domain.ModeComponents(domain.ModePlatformOnly))
	require.NoError(t, repo.CreateRun(ctx, run))

	got, err := repo.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Empty(t, got.StackID)
}

func TestPostgresBackupRepository_없는_실행(t *testing.T) {
	pool, cleanup := setupBackupPostgres(t)
	t.Cleanup(cleanup)

	_, err := NewPostgresBackupRepository(pool).GetRun(context.Background(), uuid.NewString())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BACKUP_NOT_FOUND")
}

func TestPostgresBackupRepository_산출물_왕복과_CASCADE(t *testing.T) {
	pool, cleanup := setupBackupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresBackupRepository(pool)
	ctx := context.Background()

	run := newRun(uuid.NewString())
	require.NoError(t, repo.CreateRun(ctx, run))

	for _, a := range []*domain.Artifact{
		{BackupRunID: run.ID, Component: domain.ComponentPlatformDB, Location: "s3://b/platform.dump",
			SizeBytes: 10, ChecksumSHA256: "aaa", EncryptionKeyID: "k1"},
		{BackupRunID: run.ID, Component: domain.ComponentVolume, ResourceName: "data-gitlab",
			Location: "s3://b/volumes/data-gitlab.tar", SizeBytes: 20, ChecksumSHA256: "bbb", EncryptionKeyID: "k1"},
	} {
		require.NoError(t, repo.AddArtifact(ctx, a))
		assert.NotEmpty(t, a.ID)
	}

	arts, err := repo.ListArtifacts(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, arts, 2)

	byComp := map[domain.Component]*domain.Artifact{}
	for _, a := range arts {
		byComp[a.Component] = a
	}
	assert.Equal(t, "data-gitlab", byComp[domain.ComponentVolume].ResourceName)
	assert.Empty(t, byComp[domain.ComponentPlatformDB].ResourceName, "볼륨이 아니면 빈 문자열")

	// 보존 정책은 산출물만 지운다 — 이력 행은 남아야 한다 (§4.5).
	require.NoError(t, repo.DeleteArtifacts(ctx, run.ID))
	arts, err = repo.ListArtifacts(ctx, run.ID)
	require.NoError(t, err)
	assert.Empty(t, arts)

	_, err = repo.GetRun(ctx, run.ID)
	require.NoError(t, err, "산출물을 지워도 '언제 백업이 끊겼나' 의 근거는 남는다")
}

func TestPostgresBackupRepository_실행을_지우면_산출물도_사라진다(t *testing.T) {
	pool, cleanup := setupBackupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresBackupRepository(pool)
	ctx := context.Background()

	run := newRun(uuid.NewString())
	require.NoError(t, repo.CreateRun(ctx, run))
	require.NoError(t, repo.AddArtifact(ctx, &domain.Artifact{
		BackupRunID: run.ID, Component: domain.ComponentPlatformDB, Location: "s3://b/x", ChecksumSHA256: "c",
	}))

	_, err := pool.Exec(ctx, `DELETE FROM backup_runs WHERE id = $1`, run.ID)
	require.NoError(t, err)

	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM backup_artifacts WHERE backup_run_id = $1`, run.ID).Scan(&n))
	assert.Zero(t, n, "ON DELETE CASCADE 가 동작해야 고아 행이 남지 않는다")
}

func TestPostgresBackupRepository_목록과_요약(t *testing.T) {
	pool, cleanup := setupBackupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresBackupRepository(pool)
	ctx := context.Background()
	orgID := uuid.NewString()

	for i := 0; i < 3; i++ {
		run := newRun(orgID)
		require.NoError(t, repo.CreateRun(ctx, run))
		run.TotalBytes = int64(10 * (i + 1))
		run.Complete(map[domain.Component]error{domain.ComponentPlatformDB: nil})
		require.NoError(t, repo.UpdateRun(ctx, run))
	}
	// 다른 조직 것은 섞이면 안 된다.
	other := newRun(uuid.NewString())
	require.NoError(t, repo.CreateRun(ctx, other))

	runs, err := repo.ListRuns(ctx, orgID, 0)
	require.NoError(t, err)
	assert.Len(t, runs, 3)

	summaries, err := repo.ListSummaries(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, summaries, 3)
	for _, s := range summaries {
		assert.Equal(t, domain.StatusSucceeded, s.Status)
		assert.NotZero(t, s.TotalBytes)
	}
}

func TestPostgresBackupRepository_복구_실행_왕복(t *testing.T) {
	pool, cleanup := setupBackupPostgres(t)
	t.Cleanup(cleanup)

	repo := NewPostgresBackupRepository(pool)
	ctx := context.Background()

	backupRun := newRun(uuid.NewString())
	require.NoError(t, repo.CreateRun(ctx, backupRun))

	rr := domain.NewRestoreRun(backupRun.ID, domain.ModeFull)
	require.NoError(t, repo.CreateRestore(ctx, rr))
	require.NotEmpty(t, rr.ID)

	require.NoError(t, rr.Start())
	rr.SchemaCheck = domain.SchemaCheckResult{Allowed: true, BackupVersion: 75, CurrentVersion: 75}
	rr.IntegrityReport = domain.IntegrityReport{
		CheckedPaths: 2,
		Dangling: []domain.DanglingReference{
			{TokenSourceID: "ts-1", Path: "kv/nullus/dev/o/cicd/github/api-token"},
		},
		CheckedAt: time.Now().UTC(),
	}
	rr.Succeed()
	require.NoError(t, repo.UpdateRestore(ctx, rr))

	got, err := repo.GetRestore(ctx, rr.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, got.Status)
	assert.True(t, got.SchemaCheck.Allowed)

	// dangling 목록이 살아남아야 사용자가 어떤 토큰을 재등록할지 알 수 있다.
	require.True(t, got.IntegrityReport.HasIssues())
	assert.Equal(t, "ts-1", got.IntegrityReport.Dangling[0].TokenSourceID)
	assert.Equal(t, 2, got.IntegrityReport.CheckedPaths)
}

// ── 컨테이너 준비 ────────────────────────────────────────────────────────

func setupBackupPostgres(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)

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
		t.Fatalf("postgres 컨테이너 기동: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cancel()
		t.Fatalf("접속 문자열: %v", err)
	}
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		cancel()
		t.Fatalf("pgx 풀 생성: %v", err)
	}
	if err := applyBackupMigrations(ctx, pool); err != nil {
		pool.Close()
		cancel()
		t.Fatalf("마이그레이션: %v", err)
	}

	return pool, func() {
		pool.Close()
		_ = container.Terminate(context.Background())
		cancel()
	}
}

// applyBackupMigrations 는 이 모듈의 마이그레이션(000075)만 적용한다.
//
// 전체 74개를 돌리지 않는 이유: 백업 테이블은 다른 테이블에 FK 가 없어
// 독립적이고, seed 를 포함한 전체 적용은 이 테스트에 필요 없는 시간을 쓴다.
// 그 독립성 자체도 검증 대상이다 — 다른 모듈 테이블에 얽혀 있었다면
// 여기서 바로 실패한다.
func applyBackupMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("호출 파일 경로를 알 수 없습니다")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
	dir := filepath.Join(repoRoot, "db", "migrations")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("마이그레이션 디렉터리: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && (strings.HasPrefix(e.Name(), "000075_") || strings.HasPrefix(e.Name(), "000076_")) && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("백업 마이그레이션(000075/000076)을 찾지 못했습니다")
	}
	sort.Strings(files)

	for _, name := range files {
		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("%s 읽기: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("%s 적용: %w", name, err)
		}
	}
	return nil
}
