package repository

import (
	"context"
	"errors"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresOrgRepository implements port.OrgRepository using pgx.
type PostgresOrgRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresOrgRepository creates a new PostgresOrgRepository.
func NewPostgresOrgRepository(pool *pgxpool.Pool) *PostgresOrgRepository {
	return &PostgresOrgRepository{pool: pool}
}

// Create inserts a new organization into the database.
func (r *PostgresOrgRepository) Create(ctx context.Context, org *domain.Organization) error {
	const q = `
		INSERT INTO organizations (id, name, slug, domain, status, default_admin_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		org.ID, org.Name, org.Slug, org.Domain, org.Status,
		org.DefaultAdminID, org.CreatedAt, org.UpdatedAt,
	)
	return err
}

// GetByID retrieves an organization by its ID. Returns nil if not found.
func (r *PostgresOrgRepository) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	const q = `
		SELECT id, name, slug, domain, status, default_admin_id, created_at, updated_at
		FROM organizations WHERE id = $1`

	org := &domain.Organization{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Domain, &org.Status,
		&org.DefaultAdminID, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return org, nil
}

// Update persists changes to an existing organization.
func (r *PostgresOrgRepository) Update(ctx context.Context, org *domain.Organization) error {
	const q = `
		UPDATE organizations
		SET name = $1, domain = $2, status = $3, updated_at = $4
		WHERE id = $5`

	_, err := r.pool.Exec(ctx, q, org.Name, org.Domain, org.Status, org.UpdatedAt, org.ID)
	return err
}

// GetBySlug retrieves an organization by its slug. Returns nil if not found.
func (r *PostgresOrgRepository) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	const q = `
		SELECT id, name, slug, domain, status, default_admin_id, created_at, updated_at
		FROM organizations WHERE slug = $1`

	org := &domain.Organization{}
	err := r.pool.QueryRow(ctx, q, slug).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Domain, &org.Status,
		&org.DefaultAdminID, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return org, nil
}
