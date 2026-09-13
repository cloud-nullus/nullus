package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresUpgradeBackupVerifier requires a recent, completed stack backup.
// A stale or partial backup is deliberately not considered rollback-ready.
type PostgresUpgradeBackupVerifier struct {
	pool   *pgxpool.Pool
	maxAge time.Duration
}

func NewPostgresUpgradeBackupVerifier(pool *pgxpool.Pool, maxAge time.Duration) *PostgresUpgradeBackupVerifier {
	return &PostgresUpgradeBackupVerifier{pool: pool, maxAge: maxAge}
}

func (v *PostgresUpgradeBackupVerifier) Ready(ctx context.Context, stackID string) (bool, string, error) {
	var finished time.Time
	err := v.pool.QueryRow(ctx, `SELECT finished_at FROM backup_runs b
 WHERE b.stack_id=$1 AND b.status='succeeded' AND b.finished_at IS NOT NULL
	 AND EXISTS (
		 SELECT 1 FROM backup_artifacts a
		 WHERE a.backup_run_id=b.id AND a.component='ns_resources'
		   AND a.location <> '' AND a.checksum_sha256 <> ''
	 )
	 ORDER BY finished_at DESC LIMIT 1`, stackID).Scan(&finished)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "복구 가능한 namespace 리소스 백업이 없다", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("check latest backup: %w", err)
	}
	age := time.Since(finished)
	if age > v.maxAge {
		return false, fmt.Sprintf("마지막 성공 백업이 %s 전이다", age.Round(time.Minute)), nil
	}
	return true, fmt.Sprintf("마지막 성공 백업: %s", finished.UTC().Format(time.RFC3339)), nil
}
