package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// PostgresDeliveryRepository 는 notification_history 다.
type PostgresDeliveryRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresDeliveryRepository(pool *pgxpool.Pool) *PostgresDeliveryRepository {
	return &PostgresDeliveryRepository{pool: pool}
}

var _ port.DeliveryRepository = (*PostgresDeliveryRepository)(nil)

type deliveryPayload struct {
	Title       string `json:"title,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
}

func (r *PostgresDeliveryRepository) Record(ctx context.Context, d *domain.Delivery) error {
	payload, _ := json.Marshal(deliveryPayload{Title: d.Title, ChannelName: d.ChannelName})
	var channelID *string
	if d.ChannelID != "" {
		channelID = &d.ChannelID
	}
	var errText *string
	if d.Error != "" {
		errText = &d.Error
	}
	const q = `
		INSERT INTO notification_history (org_id, channel_id, channel, event, status, payload, error, attempts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text, created_at`
	return r.pool.QueryRow(ctx, q, d.OrgID, channelID, string(d.ChannelType), string(d.Event), string(d.Status),
		payload, errText, d.Attempts).Scan(&d.ID, &d.CreatedAt)
}

func (r *PostgresDeliveryRepository) List(ctx context.Context, orgID string, limit int) ([]*domain.Delivery, error) {
	const q = `
		SELECT h.id::text, h.org_id, COALESCE(h.channel_id::text, ''), h.channel, h.event, h.status,
		       COALESCE(h.payload, '{}'::jsonb), COALESCE(h.error, ''), h.attempts, h.created_at,
		       COALESCE(c.name, '')
		FROM notification_history h
		LEFT JOIN notification_configs c ON c.id = h.channel_id
		WHERE h.org_id = $1
		ORDER BY h.created_at DESC, h.id
		LIMIT $2`
	rows, err := r.pool.Query(ctx, q, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Delivery
	for rows.Next() {
		var (
			d                  domain.Delivery
			typ, event, status string
			raw                []byte
			currentName        string
		)
		if err := rows.Scan(&d.ID, &d.OrgID, &d.ChannelID, &typ, &event, &status, &raw, &d.Error, &d.Attempts,
			&d.CreatedAt, &currentName); err != nil {
			return nil, err
		}
		var p deliveryPayload
		_ = json.Unmarshal(raw, &p)
		d.ChannelType = domain.ChannelType(typ)
		d.Event = sharednotify.EventType(event)
		d.Status = domain.DeliveryStatus(status)
		d.Title = p.Title
		// 지금 이름이 있으면 그것을, 채널을 지웠으면 보낼 때의 이름을 보인다.
		d.ChannelName = currentName
		if d.ChannelName == "" {
			d.ChannelName = p.ChannelName
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}
