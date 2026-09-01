// Package repository 는 backup 컨텍스트의 저장 어댑터다.
//
// 이 모듈이 소유하는 테이블만 읽고 쓴다 — backup_runs / backup_artifacts /
// restore_runs (CLAUDE.md 모듈별 테이블 소유 원칙).
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

type PostgresBackupRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresBackupRepository(pool *pgxpool.Pool) *PostgresBackupRepository {
	return &PostgresBackupRepository{pool: pool}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *PostgresBackupRepository) CreateRun(ctx context.Context, run *domain.BackupRun) error {
	scope := make([]string, 0, len(run.Scope))
	for _, c := range run.Scope {
		scope = append(scope, string(c))
	}
	manifest, err := json.Marshal(run.Manifest)
	if err != nil {
		return fmt.Errorf("매니페스트 직렬화: %w", err)
	}
	const q = `
		INSERT INTO backup_runs (org_id, stack_id, trigger, mode, scope, status, manifest)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, q,
		run.OrgID, nullable(run.StackID), string(run.Trigger), string(run.Mode), scope,
		string(run.Status), manifest,
	).Scan(&run.ID, &run.CreatedAt)
}

func (r *PostgresBackupRepository) UpdateRun(ctx context.Context, run *domain.BackupRun) error {
	manifest, err := json.Marshal(run.Manifest)
	if err != nil {
		return fmt.Errorf("매니페스트 직렬화: %w", err)
	}
	const q = `
		UPDATE backup_runs
		   SET status = $2, schema_version = $3, quiesce_started_at = $4, quiesce_ended_at = $5,
		       manifest = $6, total_bytes = $7, error = $8, started_at = $9, finished_at = $10
		 WHERE id = $1`
	_, err = r.pool.Exec(ctx, q, run.ID, string(run.Status), run.SchemaVersion,
		run.QuiesceStartedAt, run.QuiesceEndedAt, manifest, run.TotalBytes,
		nullable(run.Error), run.StartedAt, run.FinishedAt)
	return err
}

const runColumns = `id, org_id, COALESCE(stack_id::text, ''), trigger, mode, scope, status,
	COALESCE(schema_version, 0), quiesce_started_at, quiesce_ended_at, manifest,
	COALESCE(total_bytes, 0), COALESCE(error, ''), started_at, finished_at, created_at`

func scanRun(row pgx.Row) (*domain.BackupRun, error) {
	run := &domain.BackupRun{}
	var scope []string
	var manifestRaw []byte
	var trigger, mode, status string
	if err := row.Scan(&run.ID, &run.OrgID, &run.StackID, &trigger, &mode, &scope, &status,
		&run.SchemaVersion, &run.QuiesceStartedAt, &run.QuiesceEndedAt, &manifestRaw,
		&run.TotalBytes, &run.Error, &run.StartedAt, &run.FinishedAt, &run.CreatedAt); err != nil {
		return nil, err
	}
	run.Trigger = domain.Trigger(trigger)
	run.Mode = domain.Mode(mode)
	run.Status = domain.Status(status)
	for _, s := range scope {
		run.Scope = append(run.Scope, domain.Component(s))
	}
	if len(manifestRaw) > 0 {
		if err := json.Unmarshal(manifestRaw, &run.Manifest); err != nil {
			return nil, fmt.Errorf("매니페스트 역직렬화: %w", err)
		}
	}
	return run, nil
}

func (r *PostgresBackupRepository) GetRun(ctx context.Context, id string) (*domain.BackupRun, error) {
	run, err := scanRun(r.pool.QueryRow(ctx, `SELECT `+runColumns+` FROM backup_runs WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrBackupNotFound(id)
	}
	return run, err
}

func (r *PostgresBackupRepository) ListRuns(ctx context.Context, orgID string, limit int) ([]*domain.BackupRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+runColumns+` FROM backup_runs
		  WHERE ($1 = '' OR org_id::text = $1)
		  ORDER BY created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.BackupRun, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (r *PostgresBackupRepository) ListSummaries(ctx context.Context, orgID string) ([]domain.RunSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, created_at, COALESCE(total_bytes, 0), status FROM backup_runs
		  WHERE ($1 = '' OR org_id::text = $1)
		  ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.RunSummary, 0)
	for rows.Next() {
		var s domain.RunSummary
		var status string
		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.TotalBytes, &status); err != nil {
			return nil, err
		}
		s.Status = domain.Status(status)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *PostgresBackupRepository) AddArtifact(ctx context.Context, a *domain.Artifact) error {
	const q = `
		INSERT INTO backup_artifacts
			(backup_run_id, component, resource_name, location, size_bytes, checksum_sha256, encryption_key_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, q, a.BackupRunID, string(a.Component), a.ResourceName,
		a.Location, a.SizeBytes, a.ChecksumSHA256, a.EncryptionKeyID).Scan(&a.ID, &a.CreatedAt)
}

func (r *PostgresBackupRepository) ListArtifacts(ctx context.Context, backupRunID string) ([]*domain.Artifact, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, backup_run_id, component, resource_name, location, size_bytes,
		        checksum_sha256, encryption_key_id, created_at
		   FROM backup_artifacts WHERE backup_run_id = $1 ORDER BY created_at`, backupRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.Artifact, 0)
	for rows.Next() {
		a := &domain.Artifact{}
		var comp string
		if err := rows.Scan(&a.ID, &a.BackupRunID, &comp, &a.ResourceName, &a.Location,
			&a.SizeBytes, &a.ChecksumSHA256, &a.EncryptionKeyID, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Component = domain.Component(comp)
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteArtifacts 는 산출물 기록만 지운다. backup_runs 행은 남는다 —
// "언제 백업이 끊겼나" 의 근거가 사라지면 안 된다 (§4.5).
func (r *PostgresBackupRepository) DeleteArtifacts(ctx context.Context, backupRunID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM backup_artifacts WHERE backup_run_id = $1`, backupRunID)
	return err
}

func (r *PostgresBackupRepository) CreateRestore(ctx context.Context, run *domain.RestoreRun) error {
	const q = `
		INSERT INTO restore_runs (backup_run_id, mode, status)
		VALUES ($1, $2, $3) RETURNING id, created_at`
	return r.pool.QueryRow(ctx, q, nullable(run.BackupRunID), string(run.Mode), string(run.Status)).
		Scan(&run.ID, &run.CreatedAt)
}

func (r *PostgresBackupRepository) UpdateRestore(ctx context.Context, run *domain.RestoreRun) error {
	schemaCheck, err := json.Marshal(run.SchemaCheck)
	if err != nil {
		return err
	}
	report, err := json.Marshal(run.IntegrityReport)
	if err != nil {
		return err
	}
	const q = `
		UPDATE restore_runs
		   SET status = $2, schema_check = $3, integrity_report = $4,
		       error = $5, started_at = $6, finished_at = $7
		 WHERE id = $1`
	_, err = r.pool.Exec(ctx, q, run.ID, string(run.Status), schemaCheck, report,
		nullable(run.Error), run.StartedAt, run.FinishedAt)
	return err
}

func (r *PostgresBackupRepository) GetRestore(ctx context.Context, id string) (*domain.RestoreRun, error) {
	const q = `
		SELECT id, COALESCE(backup_run_id::text, ''), mode, status, schema_check, integrity_report,
		       COALESCE(error, ''), started_at, finished_at, created_at
		  FROM restore_runs WHERE id = $1`
	run := &domain.RestoreRun{}
	var mode, status string
	var schemaRaw, reportRaw []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(&run.ID, &run.BackupRunID, &mode, &status,
		&schemaRaw, &reportRaw, &run.Error, &run.StartedAt, &run.FinishedAt, &run.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrBackupNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	run.Mode = domain.RestoreMode(mode)
	run.Status = domain.RestoreStatus(status)
	if len(schemaRaw) > 0 {
		_ = json.Unmarshal(schemaRaw, &run.SchemaCheck)
	}
	if len(reportRaw) > 0 {
		_ = json.Unmarshal(reportRaw, &run.IntegrityReport)
	}
	return run, nil
}

var _ port.BackupRepository = (*PostgresBackupRepository)(nil)
