package repository

import (
	"context"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

func (r *PostgresUserRepository) Update(ctx context.Context, user *domain.User) error {
	const q = `
		UPDATE users
		SET email = $2, name = $3, role = $4, org_id = $5, is_active = $6, updated_at = $7
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q,
		user.ID, user.Email, user.Name, user.Role,
		user.OrgID, user.IsActive, user.UpdatedAt,
	)
	return err
}

func (r *PostgresUserRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM users WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, id)
	return err
}
