package repository

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

type PostgresTokenSourceRegistry struct {
	pool   *pgxpool.Pool
	secret *secrets.Router
}

func NewPostgresTokenSourceRegistry(pool *pgxpool.Pool, secret *secrets.Router) *PostgresTokenSourceRegistry {
	return &PostgresTokenSourceRegistry{pool: pool, secret: secret}
}

func (r *PostgresTokenSourceRegistry) Upsert(ctx context.Context, input port.TokenSourceInput) error {
	manager := strings.TrimSpace(input.SecretManager)
	if manager == "" {
		manager = "openbao"
	}
	if r.secret != nil && strings.TrimSpace(input.TokenValue) != "" {
		if err := r.secret.PutToken(ctx, manager, input.Path, input.TokenValue); err != nil {
			return err
		}
	}

	const q = `
		INSERT INTO token_sources (org_id, module, provider, path, token_type, status, next_check_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, now() + interval '24 hours', jsonb_build_object('secret_manager', $7::text))
		ON CONFLICT (org_id, provider, path) WHERE deleted_at IS NULL
		DO UPDATE SET
			module = EXCLUDED.module,
			token_type = EXCLUDED.token_type,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			next_check_at = EXCLUDED.next_check_at,
			updated_at = now()`
	_, err := r.pool.Exec(ctx, q, input.OrgID, input.Module, input.Provider, input.Path, input.TokenType, input.Status, manager)
	return err
}
