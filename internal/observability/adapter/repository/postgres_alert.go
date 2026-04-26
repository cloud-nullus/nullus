package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/observability/domain"
)

type PostgresAlertRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAlertRepository(pool *pgxpool.Pool) *PostgresAlertRepository {
	return &PostgresAlertRepository{pool: pool}
}

func (r *PostgresAlertRepository) Create(ctx context.Context, alert *domain.Alert) error {
	const q = `
		INSERT INTO alerts (id, rule_id, severity, message, fired_at, resolved_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, q,
		alert.ID,
		alert.RuleID,
		string(alert.Severity),
		alert.Message,
		alert.FiredAt,
		alert.ResolvedAt,
	)
	return err
}

func (r *PostgresAlertRepository) List(ctx context.Context) ([]*domain.Alert, error) {
	const q = `
		SELECT id, rule_id, severity, message, fired_at, resolved_at
		FROM alerts
		ORDER BY fired_at DESC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*domain.Alert
	for rows.Next() {
		var (
			a        domain.Alert
			severity string
		)

		if err := rows.Scan(
			&a.ID,
			&a.RuleID,
			&severity,
			&a.Message,
			&a.FiredAt,
			&a.ResolvedAt,
		); err != nil {
			return nil, err
		}

		a.Severity = domain.AlertSeverity(severity)
		alerts = append(alerts, &a)
	}

	return alerts, rows.Err()
}
