package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type PostgresUpgradeRepository struct{ pool *pgxpool.Pool }

func NewPostgresUpgradeRepository(pool *pgxpool.Pool) *PostgresUpgradeRepository {
	return &PostgresUpgradeRepository{pool: pool}
}

const upgradeBundleColumns = `id, tool, release_name, chart_name, repo_url,
 source_chart_version, source_app_version, target_chart_version, target_app_version,
 risk_level, expected_disruption, release_notes, requires_backup,
 supported_architectures, status, fixes_cves`

func (r *PostgresUpgradeRepository) ListVerifiedBundles(ctx context.Context, tool, source string) ([]domain.UpgradeBundle, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+upgradeBundleColumns+`
 FROM upgrade_bundles WHERE tool=$1 AND source_chart_version=$2 AND status='verified'
 ORDER BY target_chart_version`, tool, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.UpgradeBundle{}
	for rows.Next() {
		b, err := scanUpgradeBundle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

func (r *PostgresUpgradeRepository) GetVerifiedBundle(ctx context.Context, id string) (*domain.UpgradeBundle, error) {
	b, err := scanUpgradeBundle(r.pool.QueryRow(ctx, `SELECT `+upgradeBundleColumns+`
 FROM upgrade_bundles WHERE id=$1 AND status='verified'`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", domain.ErrUpgradeBundleNotFound, id)
	}
	return b, err
}

type rowScanner interface{ Scan(...any) error }

func scanUpgradeBundle(row rowScanner) (*domain.UpgradeBundle, error) {
	var b domain.UpgradeBundle
	var architectures, fixes []byte
	err := row.Scan(&b.ID, &b.Tool, &b.ReleaseName, &b.ChartName, &b.RepoURL,
		&b.SourceChartVersion, &b.SourceAppVersion, &b.TargetChartVersion, &b.TargetAppVersion,
		&b.RiskLevel, &b.ExpectedDisruption, &b.ReleaseNotes, &b.RequiresBackup,
		&architectures, &b.Status, &fixes)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(architectures, &b.SupportedArchitectures); err != nil {
		return nil, fmt.Errorf("decode upgrade bundle architectures: %w", err)
	}
	if err := json.Unmarshal(fixes, &b.FixesCVEs); err != nil {
		return nil, fmt.Errorf("decode upgrade bundle fixes_cves: %w", err)
	}
	return &b, nil
}

const upgradeRunColumns = `id, stack_id, bundle_id, tool, status, current_step,
 requested_by, reason, idempotency_key, source_chart_version,
 target_chart_version, previous_revision, result_revision, checks, error,
 started_at, updated_at, finished_at`

// CreateRun 은 진행 중 실행의 부분 유니크 인덱스와 Idempotency-Key 유니크 제약으로 동시
// 요청을 가린다. 어느 제약에 걸렸는지를 도메인 오류로 바꿔 돌려준다.
func (r *PostgresUpgradeRepository) CreateRun(ctx context.Context, run *domain.UpgradeRun) error {
	checks, err := json.Marshal(run.Checks)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO stack_upgrade_runs (id, stack_id, bundle_id,
 tool, status, current_step, requested_by, reason, idempotency_key,
 source_chart_version, target_chart_version, previous_revision, result_revision,
 checks, error, started_at, updated_at, finished_at)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		run.ID, run.StackID, run.BundleID, run.Tool, run.Status, run.CurrentStep,
		run.RequestedBy, run.Reason, run.IdempotencyKey, run.SourceChartVersion,
		run.TargetChartVersion, run.PreviousRevision, run.ResultRevision, checks,
		run.Error, run.StartedAt, run.UpdatedAt, run.FinishedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_stack_upgrade_runs_active":
			return domain.ErrUpgradeInProgress
		case "uq_stack_upgrade_runs_idempotency":
			return port.ErrUpgradeIdempotencyConflict
		}
	}
	return err
}

const activeUpgradeStatuses = `('upgrading', 'rolling_back')`

func (r *PostgresUpgradeRepository) UpdateRun(ctx context.Context, run *domain.UpgradeRun) error {
	checks, err := json.Marshal(run.Checks)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `UPDATE stack_upgrade_runs SET status=$2, current_step=$3,
 result_revision=$4, checks=$5, error=$6, updated_at=$7, finished_at=$8
 WHERE id=$1 AND status IN `+activeUpgradeStatuses, run.ID,
		run.Status, run.CurrentStep, run.ResultRevision, checks, run.Error, run.UpdatedAt, run.FinishedAt)
	if err != nil {
		return err
	}
	return r.closedOrMissing(ctx, run.ID, tag.RowsAffected())
}

func (r *PostgresUpgradeRepository) Heartbeat(ctx context.Context, runID string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE stack_upgrade_runs SET updated_at=$2
 WHERE id=$1 AND status IN `+activeUpgradeStatuses, runID, at)
	if err != nil {
		return err
	}
	return r.closedOrMissing(ctx, runID, tag.RowsAffected())
}

func (r *PostgresUpgradeRepository) CloseStaleRun(ctx context.Context, run *domain.UpgradeRun, staleBefore time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE stack_upgrade_runs SET status=$2, current_step=$3,
 error=$4, updated_at=$5, finished_at=$6
 WHERE id=$1 AND status IN `+activeUpgradeStatuses+` AND updated_at < $7`,
		run.ID, run.Status, run.CurrentStep, run.Error, run.UpdatedAt, run.FinishedAt, staleBefore)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// closedOrMissing 은 조건부 UPDATE 가 한 행도 바꾸지 못한 이유를 가른다.
func (r *PostgresUpgradeRepository) closedOrMissing(ctx context.Context, runID string, affected int64) error {
	if affected > 0 {
		return nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stack_upgrade_runs WHERE id=$1)`, runID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return domain.ErrUpgradeRunClosed
	}
	return domain.ErrUpgradeRunNotFound
}

func (r *PostgresUpgradeRepository) GetRun(ctx context.Context, stackID, runID string) (*domain.UpgradeRun, error) {
	run, err := scanUpgradeRun(r.pool.QueryRow(ctx, `SELECT `+upgradeRunColumns+`
 FROM stack_upgrade_runs WHERE stack_id=$1 AND id=$2`, stackID, runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUpgradeRunNotFound
	}
	return run, err
}

func (r *PostgresUpgradeRepository) FindRunByIdempotencyKey(ctx context.Context, stackID, key string) (*domain.UpgradeRun, error) {
	return r.optionalRun(ctx, `SELECT `+upgradeRunColumns+`
 FROM stack_upgrade_runs WHERE stack_id=$1 AND idempotency_key=$2`, stackID, key)
}

func (r *PostgresUpgradeRepository) ActiveRun(ctx context.Context, stackID string) (*domain.UpgradeRun, error) {
	return r.optionalRun(ctx, `SELECT `+upgradeRunColumns+`
 FROM stack_upgrade_runs WHERE stack_id=$1 AND status IN ('upgrading', 'rolling_back')`, stackID)
}

func (r *PostgresUpgradeRepository) LatestRun(ctx context.Context, stackID, tool string) (*domain.UpgradeRun, error) {
	return r.optionalRun(ctx, `SELECT `+upgradeRunColumns+`
 FROM stack_upgrade_runs WHERE stack_id=$1 AND tool=$2 AND status <> 'blocked'
 ORDER BY started_at DESC LIMIT 1`, stackID, tool)
}

func (r *PostgresUpgradeRepository) optionalRun(ctx context.Context, query string, args ...any) (*domain.UpgradeRun, error) {
	run, err := scanUpgradeRun(r.pool.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

func scanUpgradeRun(row rowScanner) (*domain.UpgradeRun, error) {
	var run domain.UpgradeRun
	var checks []byte
	err := row.Scan(&run.ID, &run.StackID, &run.BundleID, &run.Tool, &run.Status,
		&run.CurrentStep, &run.RequestedBy, &run.Reason, &run.IdempotencyKey,
		&run.SourceChartVersion, &run.TargetChartVersion, &run.PreviousRevision,
		&run.ResultRevision, &checks, &run.Error, &run.StartedAt, &run.UpdatedAt, &run.FinishedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(checks, &run.Checks); err != nil {
		return nil, fmt.Errorf("decode upgrade checks: %w", err)
	}
	return &run, nil
}

var _ port.UpgradeRepository = (*PostgresUpgradeRepository)(nil)
