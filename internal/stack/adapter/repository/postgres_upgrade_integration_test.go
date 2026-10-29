//go:build integration

package repository

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

func createUpgradeTestStack(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	stackID := "stk-up-" + uuid.NewString()[:8]
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, NewPostgresStackRepository(pool).Create(ctx, &domain.Stack{
		ID: stackID, Name: "Upgrade Stack", TemplateID: "gitlab-allinone-v1", OrgID: orgID, ClusterID: clusterID,
		Namespace: "up-ns", State: domain.StateCompleted, Config: sampleStackConfig("GitLab CE"), CreatedAt: now, UpdatedAt: now,
	}))
	return stackID
}

func insertUpgradeBundle(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, source, target, status string) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO upgrade_bundles (id, tool, release_name, chart_name, repo_url,
 source_chart_version, source_app_version, target_chart_version, target_app_version, risk_level,
 expected_disruption, requires_backup, supported_architectures, status)
 VALUES ($1,'argocd','argo-cd','argo-cd','https://argoproj.github.io/argo-helm',$2,'v2.13.3',$3,'v2.13.5','low',
 'API 서버 재시작 약 1분',TRUE,'["amd64","arm64"]',$4)`, id, source, target, status)
	require.NoError(t, err)
}

func newUpgradeRunFor(stackID, key string, at time.Time) *domain.UpgradeRun {
	run, _ := domain.NewUpgradeRun(domain.NewUpgradeRunInput{
		ID: uuid.NewString(), StackID: stackID,
		Bundle:             domain.UpgradeBundle{ID: "bnd-20", Tool: domain.UpgradeToolArgoCD, TargetChartVersion: "7.7.20"},
		SourceChartVersion: "7.7.16", PreviousRevision: 3, RequestedBy: "devops", Reason: "CVE",
		IdempotencyKey: key, Checks: []domain.UpgradeCheck{domain.PassedCheck("backup", "ok")}, Now: at,
	})
	return run
}

func TestPostgresUpgradeRepository_Integration(t *testing.T) {
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresUpgradeRepository(pool)

	insertUpgradeBundle(t, ctx, pool, "bnd-18", "7.7.16", "7.7.18", domain.UpgradeBundleVerified)
	insertUpgradeBundle(t, ctx, pool, "bnd-20", "7.7.16", "7.7.20", domain.UpgradeBundleVerified)
	insertUpgradeBundle(t, ctx, pool, "bnd-draft", "7.7.16", "7.8.0", domain.UpgradeBundleDraft)
	insertUpgradeBundle(t, ctx, pool, "bnd-other", "7.7.17", "7.7.20", domain.UpgradeBundleVerified)

	t.Run("검증된 번들만 출발 버전으로 고른다", func(t *testing.T) {
		bundles, err := repo.ListVerifiedBundles(ctx, domain.UpgradeToolArgoCD, "7.7.16")
		require.NoError(t, err)
		ids := []string{}
		for _, b := range bundles {
			ids = append(ids, b.ID)
		}
		assert.ElementsMatch(t, []string{"bnd-18", "bnd-20"}, ids)
		assert.Equal(t, []string{"amd64", "arm64"}, bundles[0].SupportedArchitectures)
		assert.Empty(t, bundles[0].FixesCVEs, "고치는 CVE 를 적지 않은 번들")

		b, err := repo.GetVerifiedBundle(ctx, "bnd-20")
		require.NoError(t, err)
		assert.Equal(t, "v2.13.5", b.TargetAppVersion)
		assert.True(t, b.RequiresBackup)

		// 리허설하며 적은 "고치는 CVE" 를 함께 읽는다.
		_, err = pool.Exec(ctx, `UPDATE upgrade_bundles SET fixes_cves='["CVE-2025-1","CVE-2025-2"]' WHERE id='bnd-20'`)
		require.NoError(t, err)
		b, err = repo.GetVerifiedBundle(ctx, "bnd-20")
		require.NoError(t, err)
		assert.Equal(t, []string{"CVE-2025-1", "CVE-2025-2"}, b.FixesCVEs)

		_, err = repo.GetVerifiedBundle(ctx, "bnd-draft")
		assert.ErrorIs(t, err, domain.ErrUpgradeBundleNotFound, "draft 는 실행할 수 없다")
	})

	t.Run("실행 저장 · 갱신 · 조회", func(t *testing.T) {
		stackID := createUpgradeTestStack(t, ctx, pool)
		t0 := time.Now().UTC().Truncate(time.Microsecond)
		run := newUpgradeRunFor(stackID, "k1", t0)
		require.NoError(t, repo.CreateRun(ctx, run))

		active, err := repo.ActiveRun(ctx, stackID)
		require.NoError(t, err)
		require.NotNil(t, active)
		assert.Equal(t, run.ID, active.ID)

		run.EnterHealthCheck(4, t0.Add(time.Minute))
		run.Succeed(t0.Add(2 * time.Minute))
		require.NoError(t, repo.UpdateRun(ctx, run))

		got, err := repo.GetRun(ctx, stackID, run.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.UpgradeStatusSucceeded, got.Status)
		assert.Equal(t, 4, got.ResultRevision)
		assert.True(t, t0.Add(2*time.Minute).Equal(got.UpdatedAt), "%s", got.UpdatedAt)
		require.NotNil(t, got.FinishedAt)
		require.Len(t, got.Checks, 1)
		assert.Equal(t, "k1", got.IdempotencyKey)

		active, err = repo.ActiveRun(ctx, stackID)
		require.NoError(t, err)
		assert.Nil(t, active)

		byKey, err := repo.FindRunByIdempotencyKey(ctx, stackID, "k1")
		require.NoError(t, err)
		assert.Equal(t, run.ID, byKey.ID)
		none, err := repo.FindRunByIdempotencyKey(ctx, stackID, "nope")
		require.NoError(t, err)
		assert.Nil(t, none)

		_, err = repo.GetRun(ctx, stackID, uuid.NewString())
		assert.ErrorIs(t, err, domain.ErrUpgradeRunNotFound)
	})

	t.Run("마지막 실행은 도구별 최근 시작 순", func(t *testing.T) {
		stackID := createUpgradeTestStack(t, ctx, pool)
		t0 := time.Now().UTC().Truncate(time.Microsecond)
		older := newUpgradeRunFor(stackID, "a", t0)
		older.Succeed(t0)
		require.NoError(t, repo.CreateRun(ctx, older))
		newer := newUpgradeRunFor(stackID, "b", t0.Add(time.Hour))
		require.NoError(t, repo.CreateRun(ctx, newer))

		last, err := repo.LatestRun(ctx, stackID, domain.UpgradeToolArgoCD)
		require.NoError(t, err)
		assert.Equal(t, newer.ID, last.ID)
		// 막힌 시도는 더 나중이어도 마지막 실행이 아니다.
		blocked, err := domain.NewUpgradeRun(domain.NewUpgradeRunInput{ID: uuid.NewString(), StackID: stackID,
			Bundle: domain.UpgradeBundle{ID: "bnd-20", Tool: domain.UpgradeToolArgoCD, TargetChartVersion: "7.7.20"},
			Reason: "x", IdempotencyKey: "c", Checks: []domain.UpgradeCheck{domain.BlockedCheck("backup", "없음", "")},
			Now: t0.Add(2 * time.Hour)})
		require.NoError(t, err)
		require.NoError(t, repo.CreateRun(ctx, blocked))
		last, err = repo.LatestRun(ctx, stackID, domain.UpgradeToolArgoCD)
		require.NoError(t, err)
		assert.Equal(t, newer.ID, last.ID)

		none, err := repo.LatestRun(ctx, stackID, "grafana")
		require.NoError(t, err)
		assert.Nil(t, none)
	})

	// 같은 스택의 두 업그레이드가 동시에 와도 하나만 진행 중이 된다(수용 기준 4).
	t.Run("동시에 들어온 진행 중 실행은 하나만 저장된다", func(t *testing.T) {
		stackID := createUpgradeTestStack(t, ctx, pool)
		now := time.Now().UTC()
		var ok, inProgress atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				err := repo.CreateRun(ctx, newUpgradeRunFor(stackID, uuid.NewString(), now))
				switch {
				case err == nil:
					ok.Add(1)
				case assert.ErrorIs(t, err, domain.ErrUpgradeInProgress):
					inProgress.Add(1)
				}
			}(i)
		}
		wg.Wait()
		assert.Equal(t, int32(1), ok.Load())
		assert.Equal(t, int32(5), inProgress.Load())

		// 막힌 실행(끝난 상태)은 진행 중 실행과 함께 남을 수 있다.
		blocked, err := domain.NewUpgradeRun(domain.NewUpgradeRunInput{ID: uuid.NewString(), StackID: stackID,
			Bundle: domain.UpgradeBundle{ID: "bnd-20", Tool: domain.UpgradeToolArgoCD, TargetChartVersion: "7.7.20"},
			Reason: "x", IdempotencyKey: "blocked", Checks: []domain.UpgradeCheck{domain.BlockedCheck("backup", "없음", "")}, Now: now})
		require.NoError(t, err)
		require.NoError(t, repo.CreateRun(ctx, blocked))
	})

	t.Run("같은 키는 충돌로 알린다", func(t *testing.T) {
		stackID := createUpgradeTestStack(t, ctx, pool)
		now := time.Now().UTC()
		first := newUpgradeRunFor(stackID, "same", now)
		first.Succeed(now)
		require.NoError(t, repo.CreateRun(ctx, first))
		err := repo.CreateRun(ctx, newUpgradeRunFor(stackID, "same", now))
		assert.ErrorIs(t, err, port.ErrUpgradeIdempotencyConflict)
	})

	// 진행 중일 때만 쓴다 — 중단으로 닫힌 실행을 뒤늦은 백그라운드 쓰기가 되살리지 않는다.
	t.Run("끝난 실행은 갱신되지 않는다", func(t *testing.T) {
		stackID := createUpgradeTestStack(t, ctx, pool)
		t0 := time.Now().UTC().Truncate(time.Microsecond)
		run := newUpgradeRunFor(stackID, "k", t0)
		require.NoError(t, repo.CreateRun(ctx, run))
		require.NoError(t, repo.Heartbeat(ctx, run.ID, t0.Add(time.Minute)))

		closed := *run
		closed.Interrupt(t0.Add(time.Hour))
		ok, err := repo.CloseStaleRun(ctx, &closed, t0.Add(30*time.Second))
		require.NoError(t, err)
		assert.False(t, ok, "박동이 기준보다 나중이면 닫지 않는다")

		ok, err = repo.CloseStaleRun(ctx, &closed, t0.Add(2*time.Minute))
		require.NoError(t, err)
		assert.True(t, ok)
		got, err := repo.GetRun(ctx, stackID, run.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.UpgradeStatusManual, got.Status)

		run.Succeed(t0.Add(2 * time.Hour))
		assert.ErrorIs(t, repo.UpdateRun(ctx, run), domain.ErrUpgradeRunClosed)
		assert.ErrorIs(t, repo.Heartbeat(ctx, run.ID, t0.Add(3*time.Hour)), domain.ErrUpgradeRunClosed)
		got, err = repo.GetRun(ctx, stackID, run.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.UpgradeStatusManual, got.Status, "닫힌 실행은 그대로다")

		ghost := newUpgradeRunFor(stackID, "ghost", t0)
		assert.ErrorIs(t, repo.UpdateRun(ctx, ghost), domain.ErrUpgradeRunNotFound)
	})

	t.Run("변경 없이 실패한 실행도 저장된다", func(t *testing.T) {
		stackID := createUpgradeTestStack(t, ctx, pool)
		t0 := time.Now().UTC()
		run := newUpgradeRunFor(stackID, "f", t0)
		require.NoError(t, repo.CreateRun(ctx, run))
		run.Fail(assert.AnError, t0)
		require.NoError(t, repo.UpdateRun(ctx, run))
	})
}

func TestPostgresUpgradeRepository_Bundles_Integration(t *testing.T) {
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresUpgradeRepository(pool)
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

	bundle, err := domain.NewUpgradeBundle(domain.NewUpgradeBundleInput{
		Tool: "argocd", ReleaseName: "argo-cd", ChartName: "argo-cd", RepoURL: "https://argoproj.github.io/argo-helm",
		SourceChartVersion: "7.7.16", SourceAppVersion: "v2.13.3", TargetChartVersion: "7.7.22", TargetAppVersion: "v2.13.4",
		RiskLevel: "low", ExpectedDisruption: "약 1~2분 재시작", ReleaseNotes: "https://github.com/argoproj/argo-cd/releases/tag/v2.13.4",
		RequiresBackup: true, SupportedArchitectures: []string{"arm64", "amd64"}, FixesCVEs: []string{"CVE-2025-12345"},
		Status: domain.UpgradeBundleVerified, RehearsalConfirmed: true, Now: now,
	})
	require.NoError(t, err)
	require.NoError(t, repo.CreateBundle(ctx, bundle))

	t.Run("same id or same path is a conflict", func(t *testing.T) {
		assert.ErrorIs(t, repo.CreateBundle(ctx, bundle), domain.ErrUpgradeBundleExists)
		samePath := *bundle
		samePath.ID = "another-id"
		assert.ErrorIs(t, repo.CreateBundle(ctx, &samePath), domain.ErrUpgradeBundleExists)
	})

	t.Run("get and list carry every column", func(t *testing.T) {
		got, err := repo.GetBundle(ctx, bundle.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"amd64", "arm64"}, got.SupportedArchitectures)
		assert.Equal(t, []string{"CVE-2025-12345"}, got.FixesCVEs)
		assert.Equal(t, now, got.CreatedAt.UTC())
		assert.Equal(t, domain.UpgradeBundleVerified, got.Status)

		items, err := repo.ListBundles(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, items)
		verified, err := repo.GetVerifiedBundle(ctx, bundle.ID)
		require.NoError(t, err, "등록한 번들은 후보 조회에도 잡힌다")
		assert.Equal(t, "7.7.22", verified.TargetChartVersion)
	})

	t.Run("status update hides revoked bundles from candidates", func(t *testing.T) {
		require.NoError(t, repo.UpdateBundleStatus(ctx, bundle.ID, domain.UpgradeBundleRevoked))
		_, err := repo.GetVerifiedBundle(ctx, bundle.ID)
		assert.ErrorIs(t, err, domain.ErrUpgradeBundleNotFound)
		got, err := repo.GetBundle(ctx, bundle.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.UpgradeBundleRevoked, got.Status)
		assert.ErrorIs(t, repo.UpdateBundleStatus(ctx, "missing", domain.UpgradeBundleRevoked), domain.ErrUpgradeBundleUnknown)
		_, err = repo.GetBundle(ctx, "missing")
		assert.ErrorIs(t, err, domain.ErrUpgradeBundleUnknown)
	})

	t.Run("list runs newest first with blocked attempts", func(t *testing.T) {
		stackID := createUpgradeTestStack(t, ctx, pool)
		insertUpgradeBundle(t, ctx, pool, "bnd-20-"+stackID, "7.7.16", "7.7.20", domain.UpgradeBundleVerified)
		first := newUpgradeRunFor(stackID, "k1", now)
		first.BundleID = "bnd-20-" + stackID
		require.NoError(t, repo.CreateRun(ctx, first))
		first.Succeed(now.Add(time.Minute))
		require.NoError(t, repo.UpdateRun(ctx, first))
		blocked, _ := domain.NewUpgradeRun(domain.NewUpgradeRunInput{
			ID: uuid.NewString(), StackID: stackID,
			Bundle:             domain.UpgradeBundle{ID: "bnd-20-" + stackID, Tool: domain.UpgradeToolArgoCD, TargetChartVersion: "7.7.20"},
			SourceChartVersion: "7.7.16", PreviousRevision: 3, RequestedBy: "devops", Reason: "retry",
			IdempotencyKey: "k2", Checks: []domain.UpgradeCheck{domain.BlockedCheck("backup", "none", "backup first")}, Now: now.Add(2 * time.Minute),
		})
		require.NoError(t, repo.CreateRun(ctx, blocked))

		runs, err := repo.ListRuns(ctx, stackID, 10)
		require.NoError(t, err)
		require.Len(t, runs, 2)
		assert.Equal(t, blocked.ID, runs[0].ID)
		assert.Equal(t, domain.UpgradeStatusBlocked, runs[0].Status)
		assert.Equal(t, first.ID, runs[1].ID)
		assert.Equal(t, domain.UpgradeStatusSucceeded, runs[1].Status)

		one, err := repo.ListRuns(ctx, stackID, 1)
		require.NoError(t, err)
		require.Len(t, one, 1)
		empty, err := repo.ListRuns(ctx, "stk-none", 10)
		require.NoError(t, err)
		assert.Empty(t, empty)
		assert.NotNil(t, empty)
	})
}
