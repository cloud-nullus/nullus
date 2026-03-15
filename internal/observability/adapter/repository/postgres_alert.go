package repository

import (
	"context"
	"errors"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAlertRuleRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAlertRuleRepository(pool *pgxpool.Pool) *PostgresAlertRuleRepository {
	return &PostgresAlertRuleRepository{pool: pool}
}

func (r *PostgresAlertRuleRepository) Create(ctx context.Context, rule *domain.AlertRule) error {
	const q = `
		INSERT INTO alert_rules (id, name, condition, threshold, channel, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, q,
		rule.ID,
		rule.Name,
		rule.Condition,
		rule.Threshold,
		string(rule.Channel),
		rule.Enabled,
	)
	return err
}

func (r *PostgresAlertRuleRepository) GetByID(ctx context.Context, id string) (*domain.AlertRule, error) {
	const q = `
		SELECT id, name, condition, threshold, channel, enabled
		FROM alert_rules
		WHERE id = $1`

	var (
		rule    domain.AlertRule
		channel string
	)

	err := r.pool.QueryRow(ctx, q, id).Scan(
		&rule.ID,
		&rule.Name,
		&rule.Condition,
		&rule.Threshold,
		&channel,
		&rule.Enabled,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAlertRuleNotFound
		}
		return nil, err
	}

	rule.Channel = domain.AlertChannel(channel)

	return &rule, nil
}

func (r *PostgresAlertRuleRepository) List(ctx context.Context) ([]*domain.AlertRule, error) {
	const q = `
		SELECT id, name, condition, threshold, channel, enabled
		FROM alert_rules
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*domain.AlertRule
	for rows.Next() {
		var (
			r       domain.AlertRule
			channel string
		)

		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.Condition,
			&r.Threshold,
			&channel,
			&r.Enabled,
		); err != nil {
			return nil, err
		}

		r.Channel = domain.AlertChannel(channel)
		rules = append(rules, &r)
	}

	return rules, rows.Err()
}

func (r *PostgresAlertRuleRepository) Update(ctx context.Context, rule *domain.AlertRule) error {
	const q = `
		UPDATE alert_rules
		SET name = $1, condition = $2, threshold = $3, channel = $4, enabled = $5, updated_at = NOW()
		WHERE id = $6`

	result, err := r.pool.Exec(ctx, q,
		rule.Name,
		rule.Condition,
		rule.Threshold,
		string(rule.Channel),
		rule.Enabled,
		rule.ID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrAlertRuleNotFound
	}

	return nil
}

func (r *PostgresAlertRuleRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM alert_rules WHERE id = $1`

	result, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrAlertRuleNotFound
	}

	return nil
}

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
