package repository

import (
	"context"
	"errors"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresUserRepository implements port.UserRepository using pgx.
type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresUserRepository creates a new PostgresUserRepository.
func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

// Create inserts a new user into the database.
func (r *PostgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	const insertUserQuery = `
		INSERT INTO users (id, email, name, role, org_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	const insertMemberQuery = `
		INSERT INTO org_members (org_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (org_id, user_id) DO NOTHING`

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, insertUserQuery,
		user.ID, user.Email, user.Name, user.Role,
		user.OrgID, user.IsActive, user.CreatedAt, user.UpdatedAt,
	); err != nil {
		return err
	}

	if user.OrgID != "" {
		if _, err := tx.Exec(ctx, insertMemberQuery, user.OrgID, user.ID, user.Role, user.CreatedAt); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetByID retrieves a user by their ID. Returns nil if not found.
func (r *PostgresUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	const q = `
		SELECT id, email, name, role, org_id, is_active, created_at, updated_at
		FROM users WHERE id = $1`

	user := &domain.User{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role,
		&user.OrgID, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

// GetByEmail retrieves a user by their email. Returns nil if not found.
func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, email, name, role, org_id, is_active, created_at, updated_at
		FROM users WHERE email = $1`

	user := &domain.User{}
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role,
		&user.OrgID, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *PostgresUserRepository) SearchByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.GetByEmail(ctx, email)
}

// ListByOrg retrieves all users belonging to a given organization.
func (r *PostgresUserRepository) ListByOrg(ctx context.Context, orgID string) ([]*domain.User, error) {
	const q = `
		SELECT
			u.id,
			u.email,
			u.name,
			om.role,
			om.org_id,
			u.is_active,
			u.created_at,
			u.updated_at
		FROM users u
		JOIN org_members om ON u.id = om.user_id
		WHERE om.org_id = $1
		ORDER BY om.joined_at ASC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user := &domain.User{}
		if err := rows.Scan(
			&user.ID, &user.Email, &user.Name, &user.Role,
			&user.OrgID, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *PostgresUserRepository) AddMember(ctx context.Context, orgID, userID string, role domain.Role) error {
	const q = `
		INSERT INTO org_members (org_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (org_id, user_id) DO NOTHING`

	_, err := r.pool.Exec(ctx, q, orgID, userID, role)
	return err
}

func (r *PostgresUserRepository) IsMember(ctx context.Context, orgID, userID string) (bool, error) {
	const q = `
		SELECT EXISTS(
			SELECT 1 FROM org_members WHERE org_id = $1 AND user_id = $2
		)`

	var exists bool
	if err := r.pool.QueryRow(ctx, q, orgID, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
