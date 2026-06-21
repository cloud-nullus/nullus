package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// PostgresHelmStepMetadataRepository stores DB-backed Helm chart metadata for stack steps.
type PostgresHelmStepMetadataRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresHelmStepMetadataRepository(pool *pgxpool.Pool) *PostgresHelmStepMetadataRepository {
	return &PostgresHelmStepMetadataRepository{pool: pool}
}

func (r *PostgresHelmStepMetadataRepository) Create(ctx context.Context, item *domain.HelmStepMetadata) error {
	if item == nil {
		return fmt.Errorf("helm step metadata is nil")
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}

	const q = `
		INSERT INTO stack_helm_step_configs (
			step_name, release_name, chart_name, repo_url, version, namespace, phase, sort_order, wait, is_enabled, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.pool.Exec(ctx, q,
		item.StepName,
		nullableString(item.ReleaseName),
		item.ChartName,
		nullableString(item.RepoURL),
		nullableString(item.Version),
		nullableString(item.Namespace),
		nullableString(item.Phase),
		item.SortOrder,
		item.Wait,
		item.IsEnabled,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create helm step metadata: %w", err)
	}
	return nil
}

func (r *PostgresHelmStepMetadataRepository) Update(ctx context.Context, item *domain.HelmStepMetadata) error {
	if item == nil {
		return fmt.Errorf("helm step metadata is nil")
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}

	const q = `
		UPDATE stack_helm_step_configs
		SET release_name = $2,
			chart_name = $3,
			repo_url = $4,
			version = $5,
			namespace = $6,
			phase = $7,
			sort_order = $8,
			wait = $9,
			is_enabled = $10,
			updated_at = $11
		WHERE step_name = $1`

	ct, err := r.pool.Exec(ctx, q,
		item.StepName,
		nullableString(item.ReleaseName),
		item.ChartName,
		nullableString(item.RepoURL),
		nullableString(item.Version),
		nullableString(item.Namespace),
		nullableString(item.Phase),
		item.SortOrder,
		item.Wait,
		item.IsEnabled,
		item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update helm step metadata: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("helm step metadata %q not found", item.StepName)
	}
	return nil
}

func (r *PostgresHelmStepMetadataRepository) Delete(ctx context.Context, stepName string) error {
	const q = `DELETE FROM stack_helm_step_configs WHERE step_name = $1`
	ct, err := r.pool.Exec(ctx, q, stepName)
	if err != nil {
		return fmt.Errorf("delete helm step metadata: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("helm step metadata %q not found", stepName)
	}
	return nil
}

func (r *PostgresHelmStepMetadataRepository) GetByStep(ctx context.Context, stepName string) (*domain.HelmStepMetadata, error) {
	const q = `
		SELECT step_name, release_name, chart_name, repo_url, version, namespace, phase, sort_order, wait, is_enabled, created_at, updated_at
		FROM stack_helm_step_configs
		WHERE step_name = $1`

	item := &domain.HelmStepMetadata{}
	var (
		releaseName sql.NullString
		repoURL     sql.NullString
		version     sql.NullString
		namespace   sql.NullString
		phase       sql.NullString
	)
	if err := r.pool.QueryRow(ctx, q, stepName).Scan(
		&item.StepName,
		&releaseName,
		&item.ChartName,
		&repoURL,
		&version,
		&namespace,
		&phase,
		&item.SortOrder,
		&item.Wait,
		&item.IsEnabled,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("helm step metadata %q not found", stepName)
		}
		return nil, fmt.Errorf("get helm step metadata: %w", err)
	}
	if releaseName.Valid {
		item.ReleaseName = releaseName.String
	}
	if repoURL.Valid {
		item.RepoURL = repoURL.String
	}
	if version.Valid {
		item.Version = version.String
	}
	if namespace.Valid {
		item.Namespace = namespace.String
	}
	if phase.Valid {
		item.Phase = phase.String
	}
	return item, nil
}

func (r *PostgresHelmStepMetadataRepository) List(ctx context.Context) ([]*domain.HelmStepMetadata, error) {
	const q = `
		SELECT step_name, release_name, chart_name, repo_url, version, namespace, phase, sort_order, wait, is_enabled, created_at, updated_at
		FROM stack_helm_step_configs
		ORDER BY sort_order ASC, step_name ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list helm step metadata: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.HelmStepMetadata, 0)
	for rows.Next() {
		item := &domain.HelmStepMetadata{}
		var (
			releaseName sql.NullString
			repoURL     sql.NullString
			version     sql.NullString
			namespace   sql.NullString
			phase       sql.NullString
		)
		if err := rows.Scan(
			&item.StepName,
			&releaseName,
			&item.ChartName,
			&repoURL,
			&version,
			&namespace,
			&phase,
			&item.SortOrder,
			&item.Wait,
			&item.IsEnabled,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan helm step metadata: %w", err)
		}
		if releaseName.Valid {
			item.ReleaseName = releaseName.String
		}
		if repoURL.Valid {
			item.RepoURL = repoURL.String
		}
		if version.Valid {
			item.Version = version.String
		}
		if namespace.Valid {
			item.Namespace = namespace.String
		}
		if phase.Valid {
			item.Phase = phase.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate helm step metadata: %w", err)
	}
	return items, nil
}
