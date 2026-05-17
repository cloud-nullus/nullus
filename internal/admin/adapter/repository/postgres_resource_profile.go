package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/admin/domain"
)

type PostgresResourceProfileRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresResourceProfileRepository(pool *pgxpool.Pool) *PostgresResourceProfileRepository {
	return &PostgresResourceProfileRepository{pool: pool}
}

func (r *PostgresResourceProfileRepository) List(ctx context.Context, orgID string) ([]*domain.OrgResourceProfile, error) {
	const q = `
		SELECT id, name, org_id, base_profile, option_overrides, applied_resource_overrides, row_units, created_at
		FROM org_resource_profiles
		WHERE org_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := make([]*domain.OrgResourceProfile, 0)
	for rows.Next() {
		profile := &domain.OrgResourceProfile{}
		var optionOverrides, appliedResourceOverrides, rowUnits []byte
		if err := rows.Scan(
			&profile.ID,
			&profile.Name,
			&profile.OrgID,
			&profile.BaseProfile,
			&optionOverrides,
			&appliedResourceOverrides,
			&rowUnits,
			&profile.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := decodeResourceProfileJSON(profile, optionOverrides, appliedResourceOverrides, rowUnits); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}

	return profiles, rows.Err()
}

func (r *PostgresResourceProfileRepository) Create(ctx context.Context, profile *domain.OrgResourceProfile) error {
	const q = `
		INSERT INTO org_resource_profiles (
			id, name, org_id, base_profile, option_overrides, applied_resource_overrides, row_units, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	optionOverrides, err := json.Marshal(profile.OptionOverrides)
	if err != nil {
		return err
	}
	appliedResourceOverrides, err := json.Marshal(profile.AppliedResourceOverrides)
	if err != nil {
		return err
	}
	rowUnits, err := json.Marshal(profile.RowUnits)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(
		ctx,
		q,
		profile.ID,
		profile.Name,
		profile.OrgID,
		profile.BaseProfile,
		optionOverrides,
		appliedResourceOverrides,
		rowUnits,
		profile.CreatedAt,
	)
	return err
}

func (r *PostgresResourceProfileRepository) Update(ctx context.Context, profile *domain.OrgResourceProfile) (bool, error) {
	const q = `
		UPDATE org_resource_profiles
		SET name = $1,
			base_profile = $2,
			option_overrides = $3,
			applied_resource_overrides = $4,
			row_units = $5
		WHERE org_id = $6 AND id = $7`

	optionOverrides, err := json.Marshal(profile.OptionOverrides)
	if err != nil {
		return false, err
	}
	appliedResourceOverrides, err := json.Marshal(profile.AppliedResourceOverrides)
	if err != nil {
		return false, err
	}
	rowUnits, err := json.Marshal(profile.RowUnits)
	if err != nil {
		return false, err
	}

	ct, err := r.pool.Exec(
		ctx,
		q,
		profile.Name,
		profile.BaseProfile,
		optionOverrides,
		appliedResourceOverrides,
		rowUnits,
		profile.OrgID,
		profile.ID,
	)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() > 0, nil
}

func (r *PostgresResourceProfileRepository) Delete(ctx context.Context, orgID, id string) error {
	const q = `DELETE FROM org_resource_profiles WHERE org_id = $1 AND id = $2`
	_, err := r.pool.Exec(ctx, q, orgID, id)
	return err
}

func decodeResourceProfileJSON(profile *domain.OrgResourceProfile, optionOverrides, appliedResourceOverrides, rowUnits []byte) error {
	if len(optionOverrides) > 0 {
		if err := json.Unmarshal(optionOverrides, &profile.OptionOverrides); err != nil {
			return err
		}
	}
	if profile.OptionOverrides == nil {
		profile.OptionOverrides = map[string]map[string]float64{}
	}

	if len(appliedResourceOverrides) > 0 {
		if err := json.Unmarshal(appliedResourceOverrides, &profile.AppliedResourceOverrides); err != nil {
			return err
		}
	}
	if profile.AppliedResourceOverrides == nil {
		profile.AppliedResourceOverrides = map[string]domain.ResourceVector{}
	}

	if len(rowUnits) > 0 {
		if err := json.Unmarshal(rowUnits, &profile.RowUnits); err != nil {
			return err
		}
	}
	if profile.RowUnits == nil {
		profile.RowUnits = map[string]domain.PlanningRowUnit{}
	}

	return nil
}
