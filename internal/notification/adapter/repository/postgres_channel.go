// Package repository 는 알림 채널과 발송 이력의 Postgres 구현이다.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/notification/domain"
	"github.com/cloud-nullus/draft/internal/notification/port"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
	"github.com/cloud-nullus/draft/pkg/crypto"
)

// ErrEncryptionKeyMissing 는 비밀을 저장·복원하려는데 ENCRYPTION_KEY 가 없거나 32바이트가 아닐 때다.
var ErrEncryptionKeyMissing = errors.New("ENCRYPTION_KEY 가 없거나 32바이트가 아닙니다 — 알림 채널의 비밀을 암호화할 수 없습니다")

// legacySecretKeys 는 000096 전에 config 에 평문으로 들어가던 비밀의 자리다.
var legacySecretKeys = []string{"webhook_url", "bot_token", "token", "password", "smtp_password"}

// PostgresChannelRepository 는 notification_configs 다. 비밀은 secret_encrypted 에 암호화해 둔다.
type PostgresChannelRepository struct {
	pool *pgxpool.Pool
	key  []byte
}

func NewPostgresChannelRepository(pool *pgxpool.Pool, encryptionKey []byte) *PostgresChannelRepository {
	return &PostgresChannelRepository{pool: pool, key: encryptionKey}
}

var _ port.ChannelRepository = (*PostgresChannelRepository)(nil)

const channelColumns = `id::text, org_id, name, channel, config, secret_encrypted, events, is_active, created_at, updated_at`

func (r *PostgresChannelRepository) Create(ctx context.Context, ch *domain.Channel) error {
	settings, secret, err := r.encode(ch)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO notification_configs (org_id, name, channel, config, secret_encrypted, events, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, created_at, updated_at`
	return r.pool.QueryRow(ctx, q, ch.OrgID, ch.Name, string(ch.Type), settings, secret, eventStrings(ch.Events), ch.Active).
		Scan(&ch.ID, &ch.CreatedAt, &ch.UpdatedAt)
}

func (r *PostgresChannelRepository) Get(ctx context.Context, orgID, id string) (*domain.Channel, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrChannelNotFound
	}
	q := `SELECT ` + channelColumns + ` FROM notification_configs WHERE id = $1 AND org_id = $2`
	ch, err := r.scan(r.pool.QueryRow(ctx, q, id, orgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrChannelNotFound
	}
	return ch, err
}

func (r *PostgresChannelRepository) List(ctx context.Context, orgID string) ([]*domain.Channel, error) {
	q := `SELECT ` + channelColumns + ` FROM notification_configs WHERE org_id = $1 ORDER BY created_at, id`
	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Channel
	for rows.Next() {
		ch, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (r *PostgresChannelRepository) Update(ctx context.Context, ch *domain.Channel) error {
	settings, secret, err := r.encode(ch)
	if err != nil {
		return err
	}
	const q = `
		UPDATE notification_configs
		SET name = $3, config = $4, secret_encrypted = $5, events = $6, is_active = $7, updated_at = NOW()
		WHERE id = $1 AND org_id = $2
		RETURNING updated_at`
	err = r.pool.QueryRow(ctx, q, ch.ID, ch.OrgID, ch.Name, settings, secret, eventStrings(ch.Events), ch.Active).
		Scan(&ch.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrChannelNotFound
	}
	return err
}

func (r *PostgresChannelRepository) Delete(ctx context.Context, orgID, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrChannelNotFound
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM notification_configs WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrChannelNotFound
	}
	return nil
}

// MigrateLegacySecrets 는 000096 전에 config 에 평문으로 저장된 비밀을 secret_encrypted 로 옮겨
// 암호화하고 config 에서 지운다. 옮긴 행 수를 돌려준다. 키가 없으면 아무것도 하지 않는다.
func (r *PostgresChannelRepository) MigrateLegacySecrets(ctx context.Context) (int, error) {
	if len(r.key) != 32 {
		return 0, nil
	}
	const q = `SELECT id::text, config FROM notification_configs WHERE secret_encrypted IS NULL AND config ?| $1`
	rows, err := r.pool.Query(ctx, q, legacySecretKeys)
	if err != nil {
		return 0, err
	}
	type legacy struct {
		id     string
		config map[string]any
	}
	var items []legacy
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return 0, err
		}
		cfg := map[string]any{}
		_ = json.Unmarshal(raw, &cfg)
		items = append(items, legacy{id: id, config: cfg})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	moved := 0
	for _, it := range items {
		secret := takeLegacySecret(it.config)
		if secret == "" {
			continue
		}
		enc, err := crypto.Encrypt(r.key, []byte(secret))
		if err != nil {
			return moved, err
		}
		cfg, _ := json.Marshal(it.config)
		if _, err := r.pool.Exec(ctx,
			`UPDATE notification_configs SET secret_encrypted = $2, config = $3, updated_at = NOW() WHERE id = $1`,
			it.id, enc, cfg); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

// takeLegacySecret 은 평문 비밀을 꺼내고 config 에서 지운다.
func takeLegacySecret(cfg map[string]any) string {
	var secret string
	for _, k := range legacySecretKeys {
		if v, ok := cfg[k].(string); ok && strings.TrimSpace(v) != "" && secret == "" {
			secret = strings.TrimSpace(v)
		}
		delete(cfg, k)
	}
	return secret
}

func (r *PostgresChannelRepository) encode(ch *domain.Channel) ([]byte, *string, error) {
	settings, err := json.Marshal(ch.Settings)
	if err != nil {
		return nil, nil, fmt.Errorf("채널 설정 직렬화: %w", err)
	}
	if ch.Secret == "" {
		return settings, nil, nil
	}
	if len(r.key) != 32 {
		return nil, nil, ErrEncryptionKeyMissing
	}
	enc, err := crypto.Encrypt(r.key, []byte(ch.Secret))
	if err != nil {
		return nil, nil, fmt.Errorf("채널 비밀 암호화: %w", err)
	}
	return settings, &enc, nil
}

func (r *PostgresChannelRepository) scan(row pgx.Row) (*domain.Channel, error) {
	var (
		ch     domain.Channel
		typ    string
		raw    []byte
		secret *string
		events []string
	)
	if err := row.Scan(&ch.ID, &ch.OrgID, &ch.Name, &typ, &raw, &secret, &events, &ch.Active, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		return nil, err
	}
	ch.Type = domain.ChannelType(typ)
	if ch.Name == "" {
		// 000096 전의 행은 이름이 없다 — 화면에서 고를 수 있게 채널 종류를 이름으로 보인다.
		ch.Name = typ
	}
	cfg := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &ch.Settings); err != nil {
			return nil, fmt.Errorf("채널 %s 설정 읽기: %w", ch.ID, err)
		}
		_ = json.Unmarshal(raw, &cfg)
	}
	for _, e := range events {
		ch.Events = append(ch.Events, sharednotify.EventType(e))
	}
	switch {
	case secret != nil && *secret != "":
		if len(r.key) != 32 {
			return nil, ErrEncryptionKeyMissing
		}
		plain, err := crypto.Decrypt(r.key, *secret)
		if err != nil {
			return nil, fmt.Errorf("채널 %s 비밀 복호화 — ENCRYPTION_KEY 가 바뀌었는지 확인하세요: %w", ch.ID, err)
		}
		ch.Secret = string(plain)
	default:
		// 아직 옮기지 않은 평문 비밀(MigrateLegacySecrets 전).
		ch.Secret = takeLegacySecret(cfg)
	}
	return &ch, nil
}

func eventStrings(events []sharednotify.EventType) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, string(e))
	}
	return out
}
