package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/port"
)

// PostgresStackAlertRepository 는 stack_alerts 다(000097). 열린 경보는 (stack_id, fingerprint) 마다 한 행이다.
type PostgresStackAlertRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresStackAlertRepository(pool *pgxpool.Pool) *PostgresStackAlertRepository {
	return &PostgresStackAlertRepository{pool: pool}
}

var _ port.StackAlertRepository = (*PostgresStackAlertRepository)(nil)

const stackAlertColumns = `id::text, org_id, stack_id, stack_name, fingerprint, alertname, severity, summary,
	description, runbook_url, labels, started_at, last_seen_at, resolved_at, last_notified_at`

func (r *PostgresStackAlertRepository) Upsert(ctx context.Context, a *domain.StackAlert) (bool, error) {
	labels, err := json.Marshal(a.Labels)
	if err != nil {
		return false, fmt.Errorf("경보 라벨 직렬화: %w", err)
	}
	// xmax = 0 은 이 문장이 새로 넣은 행이라는 뜻이다(ON CONFLICT 로 갱신한 행은 xmax 가 있다).
	const q = `
		INSERT INTO stack_alerts (org_id, stack_id, stack_name, fingerprint, alertname, severity, summary,
			description, runbook_url, labels, started_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (stack_id, fingerprint) WHERE resolved_at IS NULL
		DO UPDATE SET last_seen_at = EXCLUDED.last_seen_at, stack_name = EXCLUDED.stack_name,
			summary = EXCLUDED.summary, description = EXCLUDED.description,
			runbook_url = EXCLUDED.runbook_url, labels = EXCLUDED.labels
		RETURNING id::text, (xmax = 0)`
	var created bool
	err = r.pool.QueryRow(ctx, q, a.OrgID, a.StackID, a.StackName, a.Fingerprint, a.AlertName, a.Severity,
		a.Summary, a.Description, a.RunbookURL, labels, a.StartedAt, a.LastSeenAt).Scan(&a.ID, &created)
	return created, err
}

func (r *PostgresStackAlertRepository) ClaimNotification(ctx context.Context, id string, now time.Time, repeat time.Duration) (bool, bool, error) {
	// 행을 잠그고 본 뒤 바꾼다 — 동시에 부른 다른 레플리카는 잠금이 풀린 뒤 바뀐 값을 보고 빠진다.
	const q = `
		WITH prev AS (
			SELECT id, last_notified_at FROM stack_alerts
			WHERE id = $1 AND resolved_at IS NULL
			FOR UPDATE
		)
		UPDATE stack_alerts s SET last_notified_at = $2::timestamptz
		FROM prev
		WHERE s.id = prev.id
		  AND (prev.last_notified_at IS NULL OR prev.last_notified_at <= $2::timestamptz - make_interval(secs => $3::float8))
		RETURNING prev.last_notified_at IS NULL`
	var first bool
	err := r.pool.QueryRow(ctx, q, id, now, repeat.Seconds()).Scan(&first)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return true, first, nil
}

func (r *PostgresStackAlertRepository) ListOpen(ctx context.Context, stackID string) ([]*domain.StackAlert, error) {
	q := `SELECT ` + stackAlertColumns + ` FROM stack_alerts WHERE stack_id = $1 AND resolved_at IS NULL ORDER BY started_at`
	return r.query(ctx, q, stackID)
}

func (r *PostgresStackAlertRepository) Resolve(ctx context.Context, id string, at, staleBefore time.Time) (bool, error) {
	// 다른 레플리카(또는 잠금이 풀린 틈의 다른 수집)가 방금 본 경보는 해제하지 않는다.
	tag, err := r.pool.Exec(ctx,
		`UPDATE stack_alerts SET resolved_at = $2 WHERE id = $1 AND resolved_at IS NULL AND last_seen_at < $3`,
		id, at, staleBefore)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresStackAlertRepository) CloseForMissingStacks(ctx context.Context, keep []string, at, seenBefore time.Time) (int, error) {
	if keep == nil {
		keep = []string{}
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE stack_alerts SET resolved_at = $2
		 WHERE resolved_at IS NULL AND NOT (stack_id = ANY($1)) AND last_seen_at < $3`, keep, at, seenBefore)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (r *PostgresStackAlertRepository) List(ctx context.Context, orgID string, openOnly bool, limit int) ([]*domain.StackAlert, error) {
	q := `SELECT ` + stackAlertColumns + ` FROM stack_alerts WHERE org_id = $1`
	if openOnly {
		q += ` AND resolved_at IS NULL`
	}
	// 진행 중인 경보를 먼저 준다 — 오래 이어진 활성 경보가 최근 해제된 것들에 밀려 목록 밖으로 나가지 않게.
	q += ` ORDER BY (resolved_at IS NULL) DESC, started_at DESC, id LIMIT $2`
	return r.query(ctx, q, orgID, limit)
}

func (r *PostgresStackAlertRepository) query(ctx context.Context, q string, args ...any) ([]*domain.StackAlert, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.StackAlert
	for rows.Next() {
		var a domain.StackAlert
		var labels []byte
		if err := rows.Scan(&a.ID, &a.OrgID, &a.StackID, &a.StackName, &a.Fingerprint, &a.AlertName, &a.Severity,
			&a.Summary, &a.Description, &a.RunbookURL, &labels, &a.StartedAt, &a.LastSeenAt, &a.ResolvedAt,
			&a.LastNotifiedAt); err != nil {
			return nil, err
		}
		a.Labels = map[string]string{}
		_ = json.Unmarshal(labels, &a.Labels)
		out = append(out, &a)
	}
	return out, rows.Err()
}

// stackAlertCycleLockKey 는 수집 잠금의 Postgres advisory lock 키다(임의의 고정값 — 다른 잠금과 겹치지 않게).
const stackAlertCycleLockKey int64 = 0x4e554c4c5553_87 // "NULLUS" + #87

// PostgresCycleLock 은 port.CycleLock 이다 — 세션 advisory lock 을 연결 하나에 잡고 수집이 끝나면 푼다.
type PostgresCycleLock struct {
	pool *pgxpool.Pool
}

func NewPostgresCycleLock(pool *pgxpool.Pool) *PostgresCycleLock {
	return &PostgresCycleLock{pool: pool}
}

var _ port.CycleLock = (*PostgresCycleLock)(nil)

func (l *PostgresCycleLock) TryAcquire(ctx context.Context) (func(), bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	var ok bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, stackAlertCycleLockKey).Scan(&ok); err != nil {
		conn.Release()
		return nil, false, err
	}
	if !ok {
		conn.Release()
		return nil, false, nil
	}
	release := func() {
		// 수집 ctx 가 끝났어도 풀어야 한다 — 못 풀면 연결이 닫힐 때까지 아무도 돌지 못한다.
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, stackAlertCycleLockKey); err != nil {
			// 풀지 못한 연결은 풀에 돌려보내지 않는다 — 닫으면 세션 잠금도 함께 풀린다.
			_ = conn.Conn().Close(unlockCtx)
		}
		conn.Release()
	}
	return release, true, nil
}
