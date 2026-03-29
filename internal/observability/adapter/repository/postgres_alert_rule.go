package repository

import (
	"context"
	"errors"
	"strings"

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
		INSERT INTO alert_rules (id, name, condition, threshold, warning_threshold, critical_threshold, channel, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		rule.ID,
		rule.Name,
		normalizeAlertRuleCondition(rule),
		rule.CriticalThreshold,
		rule.WarningThreshold,
		rule.CriticalThreshold,
		string(rule.Channel),
		rule.Enabled,
	)
	return err
}

func (r *PostgresAlertRuleRepository) GetByID(ctx context.Context, id string) (*domain.AlertRule, error) {
	const q = `
		SELECT id, name, condition, threshold, warning_threshold, critical_threshold, channel, enabled
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
		&rule.WarningThreshold,
		&rule.CriticalThreshold,
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
	rule.MetricName = extractMetricName(rule.Condition)
	if rule.CriticalThreshold == 0 {
		rule.CriticalThreshold = rule.Threshold
	}
	if rule.WarningThreshold == 0 {
		rule.WarningThreshold = rule.Threshold
	}

	return &rule, nil
}

func (r *PostgresAlertRuleRepository) List(ctx context.Context) ([]*domain.AlertRule, error) {
	const q = `
		SELECT id, name, condition, threshold, warning_threshold, critical_threshold, channel, enabled
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
			&r.WarningThreshold,
			&r.CriticalThreshold,
			&channel,
			&r.Enabled,
		); err != nil {
			return nil, err
		}

		r.Channel = domain.AlertChannel(channel)
		r.MetricName = extractMetricName(r.Condition)
		if r.CriticalThreshold == 0 {
			r.CriticalThreshold = r.Threshold
		}
		if r.WarningThreshold == 0 {
			r.WarningThreshold = r.Threshold
		}
		rules = append(rules, &r)
	}

	return rules, rows.Err()
}

func (r *PostgresAlertRuleRepository) Update(ctx context.Context, rule *domain.AlertRule) error {
	const q = `
		UPDATE alert_rules
		SET name = $1, condition = $2, threshold = $3, warning_threshold = $4, critical_threshold = $5, channel = $6, enabled = $7, updated_at = NOW()
		WHERE id = $8`

	result, err := r.pool.Exec(ctx, q,
		rule.Name,
		normalizeAlertRuleCondition(rule),
		rule.CriticalThreshold,
		rule.WarningThreshold,
		rule.CriticalThreshold,
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

func extractMetricName(condition string) string {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return ""
	}
	for idx, r := range condition {
		switch r {
		case ' ', '>', '<', '=':
			return strings.TrimSpace(condition[:idx])
		}
	}
	return condition
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
