package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCompatibilityRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCompatibilityRepository(pool *pgxpool.Pool) *PostgresCompatibilityRepository {
	return &PostgresCompatibilityRepository{pool: pool}
}

func (r *PostgresCompatibilityRepository) GetAll(ctx context.Context) ([]*domain.CompatibilityMatrix, error) {
	const q = `
		SELECT id, name, status, k8s_min, k8s_max, k8s_recommended, tools
		FROM compatibility_matrices
		ORDER BY id ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matrices []*domain.CompatibilityMatrix
	for rows.Next() {
		m, err := scanCompatibilityMatrix(rows)
		if err != nil {
			return nil, err
		}
		matrices = append(matrices, m)
	}

	return matrices, rows.Err()
}

func (r *PostgresCompatibilityRepository) GetByID(ctx context.Context, id string) (*domain.CompatibilityMatrix, error) {
	const q = `
		SELECT id, name, status, k8s_min, k8s_max, k8s_recommended, tools
		FROM compatibility_matrices
		WHERE id = $1`

	m, err := scanCompatibilityMatrix(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("compatibility matrix %q not found", id)
		}
		return nil, err
	}

	return m, nil
}

func (r *PostgresCompatibilityRepository) Validate(ctx context.Context, tools map[string]string) (*domain.CompatibilityMatrix, error) {
	matrices, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, m := range matrices {
		if matchesMatrix(m, tools) {
			return m, nil
		}
	}

	return nil, fmt.Errorf("no compatible matrix found for the given tool combination")
}

// Create inserts a new matrix. Returns port.ErrCompatibilityMatrixExists
// when the id is already present (ON CONFLICT DO NOTHING + rows-affected check).
func (r *PostgresCompatibilityRepository) Create(ctx context.Context, m *domain.CompatibilityMatrix) error {
	toolsJSON, err := json.Marshal(m.Tools)
	if err != nil {
		return fmt.Errorf("marshal tools: %w", err)
	}
	const q = `
		INSERT INTO compatibility_matrices
			(id, name, status, k8s_min, k8s_max, k8s_recommended, tools)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING`
	tag, err := r.pool.Exec(ctx, q,
		m.ID, m.Name, m.Status,
		m.Kubernetes.Min, m.Kubernetes.Max, m.Kubernetes.Recommended,
		toolsJSON,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return port.ErrCompatibilityMatrixExists
	}
	return nil
}

// Update replaces every mutable column. Returns
// port.ErrCompatibilityMatrixNotFound when no row matched.
func (r *PostgresCompatibilityRepository) Update(ctx context.Context, m *domain.CompatibilityMatrix) error {
	toolsJSON, err := json.Marshal(m.Tools)
	if err != nil {
		return fmt.Errorf("marshal tools: %w", err)
	}
	const q = `
		UPDATE compatibility_matrices
		SET name = $2, status = $3, k8s_min = $4, k8s_max = $5,
		    k8s_recommended = $6, tools = $7, updated_at = NOW()
		WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q,
		m.ID, m.Name, m.Status,
		m.Kubernetes.Min, m.Kubernetes.Max, m.Kubernetes.Recommended,
		toolsJSON,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return port.ErrCompatibilityMatrixNotFound
	}
	return nil
}

// Delete removes a matrix. Idempotent — missing id is not an error.
func (r *PostgresCompatibilityRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM compatibility_matrices WHERE id = $1`, id)
	return err
}

type compatibilityScanner interface {
	Scan(dest ...any) error
}

func scanCompatibilityMatrix(row compatibilityScanner) (*domain.CompatibilityMatrix, error) {
	var (
		m         domain.CompatibilityMatrix
		toolsJSON []byte
	)

	err := row.Scan(
		&m.ID,
		&m.Name,
		&m.Status,
		&m.Kubernetes.Min,
		&m.Kubernetes.Max,
		&m.Kubernetes.Recommended,
		&toolsJSON,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(toolsJSON, &m.Tools); err != nil {
		return nil, fmt.Errorf("unmarshal tools: %w", err)
	}

	return &m, nil
}
