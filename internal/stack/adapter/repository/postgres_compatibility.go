package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloud-nullus/draft/internal/stack/domain"
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
