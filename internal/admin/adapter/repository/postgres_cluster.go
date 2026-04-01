package repository

import (
	"context"
	"errors"
	"fmt"

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
		INSERT INTO clusters (id, name, type, types, cloud_provider, endpoint, connection_status, org_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4::cluster_type[], $5, $6, $7, $8, $9, $10)`

	clusterTypes := clusterTypesToStrings(domain.NormalizeClusterTypes(cluster.Types, cluster.Type))
	cloudProvider := cluster.CloudProvider
	if cloudProvider == "" {
		cloudProvider = domain.CloudProviderOnPremise
	}

	_, err := r.pool.Exec(ctx, q,
		cluster.ID, cluster.Name, cluster.Type, clusterTypes, cloudProvider, cluster.Endpoint,
		cluster.ConnectionStatus, cluster.OrgID, cluster.CreatedAt, cluster.UpdatedAt,
	)
	return err
}

// GetByID retrieves a cluster by its ID. Returns nil if not found.
func (r *PostgresClusterRepository) GetByID(ctx context.Context, id string) (*domain.Cluster, error) {
	const q = `
		SELECT id, name, type, COALESCE(types::text[], ARRAY[]::text[]), cloud_provider, endpoint, connection_status, org_id, created_at, updated_at
		FROM clusters WHERE id = $1`

	cluster := &domain.Cluster{}
	var rawTypes []string
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&cluster.ID, &cluster.Name, &cluster.Type, &rawTypes, &cluster.CloudProvider, &cluster.Endpoint,
		&cluster.ConnectionStatus, &cluster.OrgID, &cluster.CreatedAt, &cluster.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	cluster.Types = clusterTypesFromStrings(rawTypes)
	return cluster, nil
}

// List retrieves clusters. If orgID is empty, returns all clusters.
func (r *PostgresClusterRepository) List(ctx context.Context, orgID string) ([]*domain.Cluster, error) {
	var rows pgx.Rows
	var err error

	if orgID == "" {
		const q = `
			SELECT id, name, type, COALESCE(types::text[], ARRAY[]::text[]), cloud_provider, endpoint, connection_status, org_id, created_at, updated_at
			FROM clusters ORDER BY created_at DESC`
		rows, err = r.pool.Query(ctx, q)
	} else {
		const q = `
			SELECT id, name, type, COALESCE(types::text[], ARRAY[]::text[]), cloud_provider, endpoint, connection_status, org_id, created_at, updated_at
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
		var rawTypes []string
		if err := rows.Scan(
			&cluster.ID, &cluster.Name, &cluster.Type, &rawTypes, &cluster.CloudProvider, &cluster.Endpoint,
			&cluster.ConnectionStatus, &cluster.OrgID, &cluster.CreatedAt, &cluster.UpdatedAt,
		); err != nil {
			return nil, err
		}
		cluster.Types = clusterTypesFromStrings(rawTypes)
		clusters = append(clusters, cluster)
	}
	return clusters, rows.Err()
}

// Update persists changes to an existing cluster.
func (r *PostgresClusterRepository) Update(ctx context.Context, cluster *domain.Cluster) error {
	const q = `
		UPDATE clusters
		SET name = $1, type = $2, types = $3::cluster_type[], cloud_provider = $4, endpoint = $5, connection_status = $6, updated_at = $7
		WHERE id = $8`

	clusterTypes := clusterTypesToStrings(domain.NormalizeClusterTypes(cluster.Types, cluster.Type))
	cloudProvider := cluster.CloudProvider
	if cloudProvider == "" {
		cloudProvider = domain.CloudProviderOnPremise
	}

	_, err := r.pool.Exec(ctx, q,
		cluster.Name, cluster.Type, clusterTypes, cloudProvider, cluster.Endpoint, cluster.ConnectionStatus, cluster.UpdatedAt, cluster.ID,
	)
	return err
}

// Delete removes a cluster by ID.
func (r *PostgresClusterRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM clusters WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	return err
}

func (r *PostgresClusterRepository) SaveKubeconfig(ctx context.Context, id string, kubeconfig []byte) error {
	const q = `UPDATE clusters SET kubeconfig = $1, updated_at = NOW() WHERE id = $2`
	ct, err := r.pool.Exec(ctx, q, kubeconfig, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("cluster %q not found", id)
	}
	return nil
}

func (r *PostgresClusterRepository) GetKubeconfig(ctx context.Context, id string) ([]byte, error) {
	const q = `SELECT kubeconfig FROM clusters WHERE id = $1`

	var kubeconfig []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(&kubeconfig)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return kubeconfig, nil
}

func clusterTypesToStrings(types []domain.ClusterType) []string {
	out := make([]string, 0, len(types))
	for _, clusterType := range types {
		if clusterType == "" {
			continue
		}
		out = append(out, string(clusterType))
	}
	return out
}

func clusterTypesFromStrings(values []string) []domain.ClusterType {
	out := make([]domain.ClusterType, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		out = append(out, domain.ClusterType(value))
	}
	return out
}
