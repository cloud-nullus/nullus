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
	const q = `
		SELECT id, org_id, cluster_id, state
		FROM stacks
		WHERE id = $1`

	var s port.StackSummary
	err := r.pool.QueryRow(ctx, q, stackID).Scan(
		&s.ID, &s.OrgID, &s.ClusterID, &s.State,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query stack summary: %w", err)
	}
	return &s, nil
}
