package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

type PostgresTokenSourceRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTokenSourceRepository(pool *pgxpool.Pool) *PostgresTokenSourceRepository {
	return &PostgresTokenSourceRepository{pool: pool}
}

func (r *PostgresTokenSourceRepository) ListSources(ctx context.Context, orgID string) ([]*domain.TokenSource, error) {
	const q = `
		SELECT id, org_id, module, provider, path, token_type, status,
		       expires_at, last_rotated_at, next_check_at, requires_approval,
		       metadata, created_at, updated_at
		FROM token_sources
		WHERE deleted_at IS NULL AND ($1 = '' OR org_id::text = $1)
		ORDER BY updated_at DESC`
	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.TokenSource, 0)
	for rows.Next() {
		item := &domain.TokenSource{}
		var metadataRaw []byte
		if err := rows.Scan(&item.ID, &item.OrgID, &item.Module, &item.Provider, &item.Path, &item.TokenType, &item.Status,
			&item.ExpiresAt, &item.LastRotatedAt, &item.NextCheckAt, &item.RequiresApproval, &metadataRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresTokenSourceRepository) ListEvents(ctx context.Context, tokenSourceID string) ([]*domain.TokenRotationEvent, error) {
	const q = `
		SELECT id, token_source_id, event_type, result, reason_code, detail_json, trace_id, created_at
		FROM token_rotation_events
		WHERE token_source_id = $1
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, tokenSourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.TokenRotationEvent, 0)
	for rows.Next() {
		item := &domain.TokenRotationEvent{}
		var detailsRaw []byte
		if err := rows.Scan(&item.ID, &item.TokenSourceID, &item.EventType, &item.Result, &item.ReasonCode, &detailsRaw, &item.TraceID, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(detailsRaw) > 0 {
			if err := json.Unmarshal(detailsRaw, &item.DetailJSON); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresTokenSourceRepository) GetSource(ctx context.Context, tokenSourceID string) (*domain.TokenSource, error) {
	const q = `
		SELECT id, org_id, module, provider, path, token_type, status,
		       expires_at, last_rotated_at, next_check_at, requires_approval,
		       metadata, created_at, updated_at
		FROM token_sources
		WHERE id = $1 AND deleted_at IS NULL`
	item := &domain.TokenSource{}
	var metadataRaw []byte
	err := r.pool.QueryRow(ctx, q, tokenSourceID).Scan(&item.ID, &item.OrgID, &item.Module, &item.Provider, &item.Path, &item.TokenType, &item.Status,
		&item.ExpiresAt, &item.LastRotatedAt, &item.NextCheckAt, &item.RequiresApproval, &metadataRaw, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (r *PostgresTokenSourceRepository) UpdateSourceStatus(ctx context.Context, tokenSourceID, status string) error {
	const q = `UPDATE token_sources SET status=$1, updated_at=now() WHERE id=$2 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, status, tokenSourceID)
	return err
}

func (r *PostgresTokenSourceRepository) InsertEvent(ctx context.Context, event *domain.TokenRotationEvent) error {
	detailsJSON, err := json.Marshal(event.DetailJSON)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO token_rotation_events (token_source_id, event_type, result, reason_code, detail_json, trace_id)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = r.pool.Exec(ctx, q, event.TokenSourceID, event.EventType, event.Result, event.ReasonCode, detailsJSON, event.TraceID)
	return err
}
