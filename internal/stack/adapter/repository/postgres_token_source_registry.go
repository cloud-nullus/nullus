package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

type PostgresTokenSourceRegistry struct {
	pool *pgxpool.Pool
}

func NewPostgresTokenSourceRegistry(pool *pgxpool.Pool) *PostgresTokenSourceRegistry {
	return &PostgresTokenSourceRegistry{pool: pool}
}

func (r *PostgresTokenSourceRegistry) Upsert(ctx context.Context, input port.TokenSourceInput) error {
	const q = `
		INSERT INTO token_sources (org_id, module, provider, path, token_type, status, next_check_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, now() + interval '24 hours', '{}'::jsonb)
		ON CONFLICT (org_id, provider, path) WHERE deleted_at IS NULL
		DO UPDATE SET
			module = EXCLUDED.module,
			token_type = EXCLUDED.token_type,
			status = EXCLUDED.status,
			next_check_at = EXCLUDED.next_check_at,
			updated_at = now()`
	_, err := r.pool.Exec(ctx, q, input.OrgID, input.Module, input.Provider, input.Path, input.TokenType, input.Status)
	return err
}
