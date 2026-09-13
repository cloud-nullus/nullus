package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type PostgresUpgradeRepository struct{ pool *pgxpool.Pool }

func NewPostgresUpgradeRepository(pool *pgxpool.Pool) *PostgresUpgradeRepository {
	return &PostgresUpgradeRepository{pool: pool}
}

const upgradeBundleColumns = `id, tool, release_name, chart_name, repo_url,
 source_chart_version, source_app_version, target_chart_version, target_app_version,
 risk_level, expected_disruption, release_notes, requires_backup,
 supported_architectures, status`

func (r *PostgresUpgradeRepository) ListVerifiedBundles(ctx context.Context, tool, source string) ([]domain.UpgradeBundle, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+upgradeBundleColumns+`
 FROM upgrade_bundles WHERE tool=$1 AND source_chart_version=$2 AND status='verified'
 ORDER BY target_chart_version`, tool, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.UpgradeBundle
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
		return nil, fmt.Errorf("verified upgrade bundle %q not found", id)
	}
	return b, err
}

type rowScanner interface{ Scan(...any) error }

func scanUpgradeBundle(row rowScanner) (*domain.UpgradeBundle, error) {
	var b domain.UpgradeBundle
	var architectures []byte
	err := row.Scan(&b.ID, &b.Tool, &b.ReleaseName, &b.ChartName, &b.RepoURL,
		&b.SourceChartVersion, &b.SourceAppVersion, &b.TargetChartVersion, &b.TargetAppVersion,
		&b.RiskLevel, &b.ExpectedDisruption, &b.ReleaseNotes, &b.RequiresBackup,
		&architectures, &b.Status)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(architectures, &b.SupportedArchitectures); err != nil {
		return nil, fmt.Errorf("decode upgrade bundle architectures: %w", err)
	}
	return &b, nil
}

const upgradeRunColumns = `id, stack_id, bundle_id, tool, status, current_step,
 requested_by, reason, COALESCE(idempotency_key, ''), source_chart_version,
 target_chart_version, previous_revision, result_revision, checks, error,
 started_at, finished_at`

func (r *PostgresUpgradeRepository) CreateRun(ctx context.Context, run *domain.UpgradeRun) error {
	checks, err := json.Marshal(run.Checks)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO stack_upgrade_runs (id, stack_id, bundle_id,
 tool, status, current_step, requested_by, reason, idempotency_key,
 source_chart_version, target_chart_version, previous_revision, result_revision,
 checks, error, started_at, finished_at)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,$11,$12,$13,$14,$15,$16,$17)`,
		run.ID, run.StackID, run.BundleID, run.Tool, run.Status, run.CurrentStep,
		run.RequestedBy, run.Reason, run.IdempotencyKey, run.SourceChartVersion,
		run.TargetChartVersion, run.PreviousRevision, run.ResultRevision, checks,
		run.Error, run.StartedAt, run.FinishedAt)
	return err
}

func (r *PostgresUpgradeRepository) UpdateRun(ctx context.Context, run *domain.UpgradeRun) error {
	checks, err := json.Marshal(run.Checks)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE stack_upgrade_runs SET status=$2,current_step=$3,
 result_revision=$4,checks=$5,error=$6,finished_at=$7 WHERE id=$1`, run.ID,
		run.Status, run.CurrentStep, run.ResultRevision, checks, run.Error, run.FinishedAt)
	return err
}

func (r *PostgresUpgradeRepository) GetRun(ctx context.Context, stackID, runID string) (*domain.UpgradeRun, error) {
	run, err := scanUpgradeRun(r.pool.QueryRow(ctx, `SELECT `+upgradeRunColumns+`
 FROM stack_upgrade_runs WHERE stack_id=$1 AND id=$2`, stackID, runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("upgrade run not found")
	}
	return run, err
}

func (r *PostgresUpgradeRepository) FindRunByIdempotencyKey(ctx context.Context, stackID, key string) (*domain.UpgradeRun, error) {
	if key == "" {
		return nil, nil
	}
	run, err := scanUpgradeRun(r.pool.QueryRow(ctx, `SELECT `+upgradeRunColumns+`
 FROM stack_upgrade_runs WHERE stack_id=$1 AND idempotency_key=$2`, stackID, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

func (r *PostgresUpgradeRepository) HasActiveRun(ctx context.Context, stackID string) (bool, error) {
	var found bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stack_upgrade_runs
 WHERE stack_id=$1 AND status IN ('preflighting','upgrading','rolling_back'))`, stackID).Scan(&found)
	return found, err
}

func scanUpgradeRun(row rowScanner) (*domain.UpgradeRun, error) {
	var run domain.UpgradeRun
	var checks []byte
	err := row.Scan(&run.ID, &run.StackID, &run.BundleID, &run.Tool, &run.Status,
		&run.CurrentStep, &run.RequestedBy, &run.Reason, &run.IdempotencyKey,
		&run.SourceChartVersion, &run.TargetChartVersion, &run.PreviousRevision,
		&run.ResultRevision, &checks, &run.Error, &run.StartedAt, &run.FinishedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(checks, &run.Checks); err != nil {
		return nil, err
	}
	return &run, nil
}
