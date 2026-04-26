package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// PostgresClusterReader implements port.ClusterReader by querying the
// clusters table the admin module owns. This is safe inside the modular
// monolith because both modules share one database; a future microservice
// split would replace this with a gRPC/HTTP client.
type PostgresClusterReader struct {
	pool *pgxpool.Pool
}

// NewPostgresClusterReader constructs a PostgresClusterReader.
func NewPostgresClusterReader(pool *pgxpool.Pool) *PostgresClusterReader {
	return &PostgresClusterReader{pool: pool}
}

// GetClusterSummary returns the subset of cluster fields the Pre-Deploy Gate
// needs. Returns nil, nil when the cluster row is missing.
func (r *PostgresClusterReader) GetClusterSummary(ctx context.Context, clusterID string) (*port.ClusterSummary, error) {
	const q = `
		SELECT id, org_id, COALESCE(node_architectures, ARRAY[]::text[])
		FROM clusters
		WHERE id = $1`

	var s port.ClusterSummary
	var archs []string
	err := r.pool.QueryRow(ctx, q, clusterID).Scan(&s.ID, &s.OrgID, &archs)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query cluster summary: %w", err)
	}
	if len(archs) > 0 {
		s.NodeArchitectures = archs
	}
	return &s, nil
}
