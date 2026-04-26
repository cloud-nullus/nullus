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

type PostgresHistoryRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresHistoryRepository(pool *pgxpool.Pool) *PostgresHistoryRepository {
	return &PostgresHistoryRepository{pool: pool}
}

func (r *PostgresHistoryRepository) SaveVersion(ctx context.Context, version *domain.StackVersion) error {
	configJSON, err := json.Marshal(version.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	const q = `
		INSERT INTO stack_config_versions (id, stack_id, version, config, changed_by, change_reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = r.pool.Exec(ctx, q,
		version.ID,
		version.StackID,
		version.Version,
		configJSON,
		version.ChangedBy,
		version.ChangeReason,
		version.CreatedAt,
	)
	return err
}

func (r *PostgresHistoryRepository) ListVersions(ctx context.Context, stackID string) ([]*domain.StackVersion, error) {
	const q = `
		SELECT id, stack_id, version, config, changed_by, change_reason, created_at
		FROM stack_config_versions
		WHERE stack_id = $1
		ORDER BY version ASC`

	rows, err := r.pool.Query(ctx, q, stackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*domain.StackVersion
	for rows.Next() {
		v, err := scanStackVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

func (r *PostgresHistoryRepository) GetVersion(ctx context.Context, stackID, versionID string) (*domain.StackVersion, error) {
	const q = `
		SELECT id, stack_id, version, config, changed_by, change_reason, created_at
		FROM stack_config_versions
		WHERE stack_id = $1 AND id = $2`

	v, err := scanStackVersion(r.pool.QueryRow(ctx, q, stackID, versionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("version %q not found for stack %q", versionID, stackID)
		}
		return nil, err
	}

	return v, nil
}

func (r *PostgresHistoryRepository) GetDiff(ctx context.Context, stackID, versionID string) ([]domain.ConfigDiff, error) {
	target, err := r.GetVersion(ctx, stackID, versionID)
	if err != nil {
		return nil, err
	}

	versions, err := r.ListVersions(ctx, stackID)
	if err != nil {
		return nil, err
	}

	var prev *domain.StackVersion
	for _, v := range versions {
		if v.Version < target.Version {
			prev = v
		}
	}

	return computeDiff(prev, target), nil
}

type stackVersionScanner interface {
	Scan(dest ...any) error
}

func scanStackVersion(row stackVersionScanner) (*domain.StackVersion, error) {
	var (
		v          domain.StackVersion
		configJSON []byte
	)

	err := row.Scan(
		&v.ID,
		&v.StackID,
		&v.Version,
		&configJSON,
		&v.ChangedBy,
		&v.ChangeReason,
		&v.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &v.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &v, nil
}
