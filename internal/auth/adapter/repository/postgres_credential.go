// Package repository 는 인증 컨텍스트의 저장소 구현이다.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/auth/domain"
)

type PostgresCredentialRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCredentialRepository(pool *pgxpool.Pool) *PostgresCredentialRepository {
	return &PostgresCredentialRepository{pool: pool}
}

// FindByEmail 은 이메일로 자격을 찾는다.
//
// 조직은 org_members 로 이어진다. 여러 조직에 속한 사용자는 가장 먼저 가입한
// 조직을 쓴다 — 조직 전환 UI 가 아직 없어 결정적인 값이 필요하다.
// 없으면 (nil, nil) 이다. "없음" 은 저장소 장애와 구분해야 한다.
func (r *PostgresCredentialRepository) FindByEmail(ctx context.Context, email string) (*domain.Credential, error) {
	const query = `
		SELECT u.id::text, u.email, u.name, u.role::text, u.is_active,
		       COALESCE(u.password_hash, ''),
		       COALESCE((
		           SELECT m.org_id::text FROM org_members m
		           WHERE m.user_id = u.id
		           ORDER BY m.joined_at ASC
		           LIMIT 1
		       ), '')
		FROM users u
		WHERE lower(u.email) = lower($1)
	`

	var c domain.Credential
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&c.UserID, &c.Email, &c.Name, &c.Role, &c.IsActive, &c.PasswordHash, &c.OrgID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user credential: %w", err)
	}
	return &c, nil
}
