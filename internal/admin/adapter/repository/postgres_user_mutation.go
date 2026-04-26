package repository

import (
	"context"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresUserRepository) Update(ctx context.Context, user *domain.User) error {
	const updateUserQuery = `
		UPDATE users
		SET email = $2, name = $3, role = $4, org_id = $5, is_active = $6, updated_at = $7
		WHERE id = $1`

	const updateMembershipRoleQuery = `
		UPDATE org_members
		SET role = $3
		WHERE org_id = $1 AND user_id = $2`

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, updateUserQuery,
		user.ID, user.Email, user.Name, user.Role,
		user.OrgID, user.IsActive, user.UpdatedAt,
	); err != nil {
		return err
	}

	if user.OrgID != "" {
		if _, err := tx.Exec(ctx, updateMembershipRoleQuery, user.OrgID, user.ID, user.Role); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresUserRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM users WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, id)
	return err
}
