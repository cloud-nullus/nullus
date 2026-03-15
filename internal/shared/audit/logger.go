package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditLogger struct {
	pool    *pgxpool.Pool
	querier auditQuerier
}

type AuditEntry struct {
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	Details      map[string]any
	IPAddress    string
}

type auditQuerier interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) (auditRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) auditRow
}

type auditRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

type auditRow interface {
	Scan(dest ...any) error
}

type pgxAuditQuerier struct {
	pool *pgxpool.Pool
}

func (q *pgxAuditQuerier) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := q.pool.Exec(ctx, sql, args...)
	return err
}

func (q *pgxAuditQuerier) Query(ctx context.Context, sql string, args ...any) (auditRows, error) {
	return q.pool.Query(ctx, sql, args...)
}

func (q *pgxAuditQuerier) QueryRow(ctx context.Context, sql string, args ...any) auditRow {
	return q.pool.QueryRow(ctx, sql, args...)
}

func NewAuditLogger(pool *pgxpool.Pool) *AuditLogger {
	return &AuditLogger{pool: pool, querier: &pgxAuditQuerier{pool: pool}}
}

func NewAuditLoggerWithQuerier(pool *pgxpool.Pool, querier auditQuerier) *AuditLogger {
	return &AuditLogger{pool: pool, querier: querier}
}

func (l *AuditLogger) Log(ctx context.Context, entry AuditEntry) error {
	const q = `
		INSERT INTO audit_logs (user_id, action, resource_type, resource_id, details, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)`

	detailsJSON, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}

	if err := l.querier.Exec(ctx, q,
		entry.UserID,
		entry.Action,
		entry.ResourceType,
		entry.ResourceID,
		detailsJSON,
		entry.IPAddress,
	); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

func (l *AuditLogger) List(ctx context.Context, limit, offset int) ([]AuditEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	const listQ = `
		SELECT user_id, action, resource_type, resource_id, details, ip_address, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := l.querier.Query(ctx, listQ, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	items := make([]AuditEntry, 0, limit)
	for rows.Next() {
		var (
			item      AuditEntry
			details   []byte
			createdAt time.Time
		)

		if err := rows.Scan(
			&item.UserID,
			&item.Action,
			&item.ResourceType,
			&item.ResourceID,
			&details,
			&item.IPAddress,
			&createdAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit logs: %w", err)
		}

		if len(details) > 0 {
			if err := json.Unmarshal(details, &item.Details); err != nil {
				return nil, 0, fmt.Errorf("unmarshal audit details: %w", err)
			}
		}

		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit logs: %w", err)
	}

	const countQ = `SELECT COUNT(*) FROM audit_logs`
	var total int
	if err := l.querier.QueryRow(ctx, countQ).Scan(&total); err != nil {
		if err == pgx.ErrNoRows {
			return items, 0, nil
		}
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	return items, total, nil
}
