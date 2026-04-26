package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/admin/domain"
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

	// Convert empty DefaultAdminID to nil for nullable UUID column
	var adminID interface{} = org.DefaultAdminID
	if org.DefaultAdminID == "" {
		adminID = nil
	}

	_, err := r.pool.Exec(ctx, q,
		org.ID, org.Name, org.Slug, org.Domain, org.Status,
		adminID, org.CreatedAt, org.UpdatedAt,
	)
	return err
}

// GetByID retrieves an organization by its ID. Returns nil if not found.
func (r *PostgresOrgRepository) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	const q = `
		SELECT id, name, slug, domain, status, default_admin_id, cluster_access_scope, created_at, updated_at
		FROM organizations WHERE id = $1`

	org := &domain.Organization{}
	var adminID *string
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Domain, &org.Status,
		&adminID, &org.ClusterAccessScope, &org.CreatedAt, &org.UpdatedAt,
	)
	if adminID != nil {
		org.DefaultAdminID = *adminID
	}
	if org.ClusterAccessScope == nil {
		org.ClusterAccessScope = []string{}
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return org, nil
}

func (r *PostgresOrgRepository) List(ctx context.Context, limit, offset int) ([]*domain.Organization, error) {
	const q = `
		SELECT id, name, slug, domain, status, default_admin_id, cluster_access_scope, created_at, updated_at
		FROM organizations
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2`

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orgs := make([]*domain.Organization, 0)
	for rows.Next() {
		org := &domain.Organization{}
		var adminID *string
		if err := rows.Scan(
			&org.ID, &org.Name, &org.Slug, &org.Domain, &org.Status,
			&adminID, &org.ClusterAccessScope, &org.CreatedAt, &org.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if adminID != nil {
			org.DefaultAdminID = *adminID
		}
		if org.ClusterAccessScope == nil {
			org.ClusterAccessScope = []string{}
		}
		orgs = append(orgs, org)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orgs, nil
}

// Update persists changes to an existing organization.
func (r *PostgresOrgRepository) Update(ctx context.Context, org *domain.Organization) error {
	const q = `
		UPDATE organizations
		SET name = $1, domain = $2, status = $3, cluster_access_scope = $4, updated_at = $5
		WHERE id = $6`

	_, err := r.pool.Exec(ctx, q, org.Name, org.Domain, org.Status, org.ClusterAccessScope, org.UpdatedAt, org.ID)
	return err
}

// GetBySlug retrieves an organization by its slug. Returns nil if not found.
func (r *PostgresOrgRepository) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	const q = `
		SELECT id, name, slug, domain, status, default_admin_id, cluster_access_scope, created_at, updated_at
		FROM organizations WHERE slug = $1`

	org := &domain.Organization{}
	var slugAdminID *string
	err := r.pool.QueryRow(ctx, q, slug).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Domain, &org.Status,
		&slugAdminID, &org.ClusterAccessScope, &org.CreatedAt, &org.UpdatedAt,
	)
	if slugAdminID != nil {
		org.DefaultAdminID = *slugAdminID
	}
	if org.ClusterAccessScope == nil {
		org.ClusterAccessScope = []string{}
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return org, nil
}
