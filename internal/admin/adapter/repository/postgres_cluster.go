package repository

import (
	"context"
	"errors"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresClusterRepository implements port.ClusterRepository using pgx.
type PostgresClusterRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresClusterRepository creates a new PostgresClusterRepository.
func NewPostgresClusterRepository(pool *pgxpool.Pool) *PostgresClusterRepository {
	return &PostgresClusterRepository{pool: pool}
}

// Create inserts a new cluster into the database.
func (r *PostgresClusterRepository) Create(ctx context.Context, cluster *domain.Cluster) error {
	const q = `
		INSERT INTO clusters (id, name, type, endpoint, connection_status, org_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		cluster.ID, cluster.Name, cluster.Type, cluster.Endpoint,
		cluster.ConnectionStatus, cluster.OrgID, cluster.CreatedAt, cluster.UpdatedAt,
	)
	return err
}

// GetByID retrieves a cluster by its ID. Returns nil if not found.
func (r *PostgresClusterRepository) GetByID(ctx context.Context, id string) (*domain.Cluster, error) {
	const q = `
		SELECT id, name, type, endpoint, connection_status, org_id, created_at, updated_at
		FROM clusters WHERE id = $1`

	cluster := &domain.Cluster{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&cluster.ID, &cluster.Name, &cluster.Type, &cluster.Endpoint,
		&cluster.ConnectionStatus, &cluster.OrgID, &cluster.CreatedAt, &cluster.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return cluster, nil
}

// List retrieves clusters. If orgID is empty, returns all clusters.
func (r *PostgresClusterRepository) List(ctx context.Context, orgID string) ([]*domain.Cluster, error) {
	var rows pgx.Rows
	var err error

	if orgID == "" {
		const q = `
			SELECT id, name, type, endpoint, connection_status, org_id, created_at, updated_at
			FROM clusters ORDER BY created_at DESC`
		rows, err = r.pool.Query(ctx, q)
	} else {
		const q = `
			SELECT id, name, type, endpoint, connection_status, org_id, created_at, updated_at
			FROM clusters WHERE org_id = $1 ORDER BY created_at DESC`
		rows, err = r.pool.Query(ctx, q, orgID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []*domain.Cluster
	for rows.Next() {
		cluster := &domain.Cluster{}
		if err := rows.Scan(
			&cluster.ID, &cluster.Name, &cluster.Type, &cluster.Endpoint,
			&cluster.ConnectionStatus, &cluster.OrgID, &cluster.CreatedAt, &cluster.UpdatedAt,
		); err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)
	}
	return clusters, rows.Err()
}

// Update persists changes to an existing cluster.
func (r *PostgresClusterRepository) Update(ctx context.Context, cluster *domain.Cluster) error {
	const q = `
		UPDATE clusters
		SET name = $1, endpoint = $2, connection_status = $3, updated_at = $4
		WHERE id = $5`

	_, err := r.pool.Exec(ctx, q,
		cluster.Name, cluster.Endpoint, cluster.ConnectionStatus, cluster.UpdatedAt, cluster.ID,
	)
	return err
}

// Delete removes a cluster by ID.
func (r *PostgresClusterRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM clusters WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	return err
}
