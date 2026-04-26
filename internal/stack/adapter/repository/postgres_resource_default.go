package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type PostgresResourceDefaultRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresResourceDefaultRepository(pool *pgxpool.Pool) *PostgresResourceDefaultRepository {
	return &PostgresResourceDefaultRepository{pool: pool}
}

func (r *PostgresResourceDefaultRepository) List(ctx context.Context) ([]*domain.ResourceDefault, error) {
	const q = `
		SELECT tool_key, display_name, cpu_request, cpu_limit, memory_request_gi, memory_limit_gi, storage_request_gi, storage_limit_gi, is_default, updated_at
		FROM stack_resource_defaults
		ORDER BY tool_key ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list resource defaults: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.ResourceDefault, 0)
	for rows.Next() {
		item := &domain.ResourceDefault{}
		if err := rows.Scan(
			&item.ToolKey,
			&item.DisplayName,
			&item.CPURequest,
			&item.CPULimit,
			&item.MemoryRequestGi,
			&item.MemoryLimitGi,
			&item.StorageRequestGi,
			&item.StorageLimitGi,
			&item.IsDefault,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan resource default: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource defaults: %w", err)
	}

	return items, nil
}

func (r *PostgresResourceDefaultRepository) Upsert(ctx context.Context, resource *domain.ResourceDefault) error {
	const q = `
		INSERT INTO stack_resource_defaults (
			tool_key,
			display_name,
			cpu_request,
			cpu_limit,
			memory_request_gi,
			memory_limit_gi,
			storage_request_gi,
			storage_limit_gi,
			is_default,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (tool_key) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			cpu_request = EXCLUDED.cpu_request,
			cpu_limit = EXCLUDED.cpu_limit,
			memory_request_gi = EXCLUDED.memory_request_gi,
			memory_limit_gi = EXCLUDED.memory_limit_gi,
			storage_request_gi = EXCLUDED.storage_request_gi,
			storage_limit_gi = EXCLUDED.storage_limit_gi,
			is_default = EXCLUDED.is_default,
			updated_at = NOW()`

	_, err := r.pool.Exec(
		ctx,
		q,
		resource.ToolKey,
		resource.DisplayName,
		resource.CPURequest,
		resource.CPULimit,
		resource.MemoryRequestGi,
		resource.MemoryLimitGi,
		resource.StorageRequestGi,
		resource.StorageLimitGi,
		resource.IsDefault,
	)
	if err != nil {
		return fmt.Errorf("upsert resource default: %w", err)
	}

	return nil
}
