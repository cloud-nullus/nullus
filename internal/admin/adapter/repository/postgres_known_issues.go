package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/admin/port"
)

type PostgresKnownIssuesRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresKnownIssuesRepository(pool *pgxpool.Pool) *PostgresKnownIssuesRepository {
	return &PostgresKnownIssuesRepository{pool: pool}
}

func (r *PostgresKnownIssuesRepository) List(ctx context.Context) ([]port.KnownIssue, error) {
	const q = `
		SELECT id::text, severity, title, description, COALESCE(workaround, ''), status
		FROM known_issues
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]port.KnownIssue, 0)
	for rows.Next() {
		var item port.KnownIssue
		if err := rows.Scan(
			&item.ID,
			&item.Severity,
			&item.Title,
			&item.Description,
			&item.Workaround,
			&item.Status,
		); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	return items, rows.Err()
}
