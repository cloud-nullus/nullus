package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// PostgresStackReader implements port.StackReader by querying the stacks
// table directly. This is acceptable within a Modular Monolith since both
// modules share the same database. When splitting into microservices,
// replace this with an HTTP/gRPC client calling the Stack service.
type PostgresStackReader struct {
	pool *pgxpool.Pool
}

// NewPostgresStackReader constructs a PostgresStackReader.
func NewPostgresStackReader(pool *pgxpool.Pool) *PostgresStackReader {
	return &PostgresStackReader{pool: pool}
}

// GetStackSummary retrieves minimal stack information by ID.
// Returns nil and no error if the stack does not exist.
func (r *PostgresStackReader) GetStackSummary(ctx context.Context, stackID string) (*port.StackSummary, error) {
	// 도구 이름과 접근 도메인은 config JSONB 안에 있다. 없을 수 있으므로
	// COALESCE 로 빈 문자열을 돌려준다 — 호출부가 nil 검사를 하지 않아도 되게.
	const q = `
		SELECT id, org_id, cluster_id, state,
		       COALESCE(namespace, ''),
		       COALESCE(config->'artifacts'->'source_repository'->>'name', ''),
		       COALESCE(config->'artifacts'->'container_registry'->>'name', ''),
		       COALESCE(config->>'access_domain', '')
		FROM stacks
		WHERE id = $1`

	var s port.StackSummary
	err := r.pool.QueryRow(ctx, q, stackID).Scan(
		&s.ID, &s.OrgID, &s.ClusterID, &s.State,
		&s.Namespace, &s.SourceRepository, &s.ContainerRegistry, &s.AccessDomain,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query stack summary: %w", err)
	}
	return &s, nil
}
