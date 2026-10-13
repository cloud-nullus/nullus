//go:build integration

package repository

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

func newAlert(stackID, fp string, started time.Time) *domain.StackAlert {
	return &domain.StackAlert{
		OrgID: "org-sa", StackID: stackID, StackName: "gitlab", Fingerprint: fp,
		AlertName: "NodeFilesystemAlmostOutOfSpace", Severity: "critical",
		Summary: "disk", Description: "2% left", RunbookURL: "https://runbooks/x",
		Labels: map[string]string{"instance": "10.0.0.1:9100"}, StartedAt: started, LastSeenAt: started,
	}
}

func TestPostgresStackAlertRepository_Integration(t *testing.T) {
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresStackAlertRepository(pool)
	t0 := time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)

	t.Run("데모 이력은 남지 않는다(000021 · 000097)", func(t *testing.T) {
		var alerts, history int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM alerts`).Scan(&alerts))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM notification_history`).Scan(&history))
		assert.Zero(t, alerts)
		assert.Zero(t, history)
	})

	t.Run("열린 경보는 하나 — 다시 보이면 갱신", func(t *testing.T) {
		a := newAlert("stk_1", "fp1", t0)
		created, err := repo.Upsert(ctx, a)
		require.NoError(t, err)
		assert.True(t, created)
		require.NotEmpty(t, a.ID)

		again := newAlert("stk_1", "fp1", t0)
		again.LastSeenAt = t0.Add(time.Minute)
		again.Summary = "disk (updated)"
		created, err = repo.Upsert(ctx, again)
		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, a.ID, again.ID)

		open, err := repo.ListOpen(ctx, "stk_1")
		require.NoError(t, err)
		require.Len(t, open, 1)
		assert.Equal(t, "disk (updated)", open[0].Summary)
		assert.Equal(t, "10.0.0.1:9100", open[0].Labels["instance"])
		assert.True(t, open[0].LastSeenAt.Equal(t0.Add(time.Minute)))
	})

	t.Run("알림 차례는 여러 레플리카 중 한 번만", func(t *testing.T) {
		a := newAlert("stk_1", "fp-claim", t0)
		_, err := repo.Upsert(ctx, a)
		require.NoError(t, err)

		var claimed, first atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, f, err := repo.ClaimNotification(ctx, a.ID, t0, 4*time.Hour)
				assert.NoError(t, err)
				if c {
					claimed.Add(1)
				}
				if f {
					first.Add(1)
				}
			}()
		}
		wg.Wait()
		assert.EqualValues(t, 1, claimed.Load())
		assert.EqualValues(t, 1, first.Load())

		c, _, err := repo.ClaimNotification(ctx, a.ID, t0.Add(3*time.Hour), 4*time.Hour)
		require.NoError(t, err)
		assert.False(t, c, "repeat 전에는 다시 알리지 않는다")
		c, f, err := repo.ClaimNotification(ctx, a.ID, t0.Add(4*time.Hour+time.Second), 4*time.Hour)
		require.NoError(t, err)
		assert.True(t, c)
		assert.False(t, f, "다시 알림이다")
	})

	t.Run("해제는 한 번만, 해제 뒤 같은 경보는 새 행", func(t *testing.T) {
		a := newAlert("stk_2", "fp2", t0)
		_, err := repo.Upsert(ctx, a)
		require.NoError(t, err)

		// 방금 본 경보는 해제하지 않는다(유예).
		ok, err := repo.Resolve(ctx, a.ID, t0.Add(time.Minute), t0)
		require.NoError(t, err)
		assert.False(t, ok, "staleBefore 이후에 본 경보는 해제하지 않는다")

		var resolved atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok, err := repo.Resolve(ctx, a.ID, t0.Add(time.Hour), t0.Add(time.Hour))
				assert.NoError(t, err)
				if ok {
					resolved.Add(1)
				}
			}()
		}
		wg.Wait()
		assert.EqualValues(t, 1, resolved.Load())

		next := newAlert("stk_2", "fp2", t0.Add(2*time.Hour))
		created, err := repo.Upsert(ctx, next)
		require.NoError(t, err)
		assert.True(t, created)
		assert.NotEqual(t, a.ID, next.ID)
	})

	t.Run("지워진 스택의 열린 경보를 닫는다", func(t *testing.T) {
		_, err := repo.Upsert(ctx, newAlert("stk_gone", "fp3", t0))
		require.NoError(t, err)
		// 이번 주기에 본 것(seenBefore 이후)은 닫지 않는다.
		n, err := repo.CloseForMissingStacks(ctx, []string{"stk_1", "stk_2"}, t0.Add(time.Hour), t0)
		require.NoError(t, err)
		assert.Zero(t, n)
		n, err = repo.CloseForMissingStacks(ctx, []string{"stk_1", "stk_2"}, t0.Add(time.Hour), t0.Add(time.Minute))
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		open, err := repo.ListOpen(ctx, "stk_gone")
		require.NoError(t, err)
		assert.Empty(t, open)

		// 목록이 비어도(스택이 하나도 없으면) 모두 닫는다.
		_, err = repo.Upsert(ctx, newAlert("stk_x", "fp4", t0))
		require.NoError(t, err)
		n, err = repo.CloseForMissingStacks(ctx, nil, t0.Add(time.Hour), t0.Add(time.Minute))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, n, 1)
	})

	t.Run("조직의 목록 — 진행 중 먼저, 그 안에서 최근 시작 순, 열린 것만도", func(t *testing.T) {
		all, err := repo.List(ctx, "org-sa", false, 100)
		require.NoError(t, err)
		require.NotEmpty(t, all)
		seenResolved := false
		for i, a := range all {
			if a.ResolvedAt != nil {
				seenResolved = true
			} else {
				assert.False(t, seenResolved, "진행 중 경보가 해제된 경보 뒤에 왔다")
			}
			if i > 0 && (all[i-1].ResolvedAt == nil) == (a.ResolvedAt == nil) {
				assert.False(t, a.StartedAt.After(all[i-1].StartedAt))
			}
		}
		open, err := repo.List(ctx, "org-sa", true, 100)
		require.NoError(t, err)
		for _, a := range open {
			assert.Nil(t, a.ResolvedAt)
		}
		assert.Less(t, len(open), len(all))
		other, err := repo.List(ctx, "org-other", false, 100)
		require.NoError(t, err)
		assert.Empty(t, other)
	})
}

// 수집 잠금은 한 번에 하나만 잡는다 — 레플리카 둘이 같은 주기를 돌지 않는다.
func TestPostgresCycleLock_Integration(t *testing.T) {
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	a, b := NewPostgresCycleLock(pool), NewPostgresCycleLock(pool)

	releaseA, ok, err := a.TryAcquire(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	_, ok, err = b.TryAcquire(ctx)
	require.NoError(t, err)
	assert.False(t, ok, "다른 레플리카가 잡고 있으면 건너뛴다")

	releaseA()
	releaseB, ok, err := b.TryAcquire(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "풀린 뒤에는 잡힌다")
	releaseB()
}
