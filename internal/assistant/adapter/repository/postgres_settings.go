// Package repository 는 어시스턴트 설정의 Postgres 저장소다. API 키는 ENCRYPTION_KEY 로 암호화한다.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/assistant/domain"
	"github.com/cloud-nullus/draft/internal/assistant/port"
	"github.com/cloud-nullus/draft/pkg/crypto"
)

// ErrEncryptionKeyMissing 은 ENCRYPTION_KEY 가 없어 API 키를 암호화할 수 없다는 뜻이다 — 평문으로 남기지 않는다.
var ErrEncryptionKeyMissing = fmt.Errorf("%w: ENCRYPTION_KEY 가 없어 API 키를 저장할 수 없습니다", domain.ErrKeyStorageUnavailable)

type PostgresSettingsRepository struct {
	pool *pgxpool.Pool
	key  []byte
}

var _ port.SettingsRepository = (*PostgresSettingsRepository)(nil)

func NewPostgresSettingsRepository(pool *pgxpool.Pool, encryptionKey []byte) *PostgresSettingsRepository {
	return &PostgresSettingsRepository{pool: pool, key: encryptionKey}
}

func (r *PostgresSettingsRepository) Get(ctx context.Context, orgID string) (*domain.Settings, error) {
	var s domain.Settings
	var enc string
	err := r.pool.QueryRow(ctx, `SELECT org_id, enabled, provider, base_url, model, api_key_encrypted, updated_at, updated_by
		FROM assistant_settings WHERE org_id = $1`, orgID).
		Scan(&s.OrgID, &s.Enabled, &s.Provider, &s.BaseURL, &s.Model, &enc, &s.UpdatedAt, &s.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("어시스턴트 설정 조회: %w", err)
	}
	if enc != "" {
		// 복호화하지 못하면(ENCRYPTION_KEY 가 바뀜) 키만 비우고 설정은 돌려준다 — 관리 화면에서 다시 넣게.
		if plain, err := crypto.Decrypt(r.key, enc); err != nil {
			s.KeyUnreadable = true
		} else {
			s.APIKey = string(plain)
		}
	}
	return &s, nil
}

func (r *PostgresSettingsRepository) Save(ctx context.Context, s domain.Settings) error {
	enc := ""
	if s.APIKey != "" {
		if len(r.key) != 32 {
			return ErrEncryptionKeyMissing
		}
		var err error
		if enc, err = crypto.Encrypt(r.key, []byte(s.APIKey)); err != nil {
			return fmt.Errorf("어시스턴트 API 키 암호화: %w", err)
		}
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO assistant_settings
		(org_id, enabled, provider, base_url, model, api_key_encrypted, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (org_id) DO UPDATE SET enabled = EXCLUDED.enabled, provider = EXCLUDED.provider,
			base_url = EXCLUDED.base_url, model = EXCLUDED.model, api_key_encrypted = EXCLUDED.api_key_encrypted,
			updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by`,
		s.OrgID, s.Enabled, s.Provider, s.BaseURL, s.Model, enc, s.UpdatedAt, s.UpdatedBy)
	if err != nil {
		return fmt.Errorf("어시스턴트 설정 저장: %w", err)
	}
	return nil
}

func (r *PostgresSettingsRepository) Delete(ctx context.Context, orgID string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM assistant_settings WHERE org_id = $1`, orgID); err != nil {
		return fmt.Errorf("어시스턴트 설정 삭제: %w", err)
	}
	return nil
}
