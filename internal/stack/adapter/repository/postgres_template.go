package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresTemplateRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTemplateRepository(pool *pgxpool.Pool) *PostgresTemplateRepository {
	return &PostgresTemplateRepository{pool: pool}
}

func (r *PostgresTemplateRepository) Create(ctx context.Context, template *domain.Template) error {
	toolsJSON, err := json.Marshal(template.Tools)
	if err != nil {
		return fmt.Errorf("marshal tools: %w", err)
	}

	const q = `
		INSERT INTO golden_path_templates (
			id,
			name,
			description,
			tools,
			estimated_install_time,
			recommended_use_case,
			min_resources
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = r.pool.Exec(
		ctx,
		q,
		template.ID,
		template.Name,
		template.Description,
		toolsJSON,
		int64(template.EstimatedInstallTime),
		template.RecommendedUseCase,
		template.MinResources,
	)
	if err != nil {
		return fmt.Errorf("create template: %w", err)
	}

	return nil
}

func (r *PostgresTemplateRepository) Update(ctx context.Context, template *domain.Template) error {
	toolsJSON, err := json.Marshal(template.Tools)
	if err != nil {
		return fmt.Errorf("marshal tools: %w", err)
	}

	const q = `
		UPDATE golden_path_templates
		SET
			name = $2,
			description = $3,
			tools = $4,
			estimated_install_time = $5,
			recommended_use_case = $6,
			min_resources = $7,
			updated_at = NOW()
		WHERE id = $1`

	ct, err := r.pool.Exec(
		ctx,
		q,
		template.ID,
		template.Name,
		template.Description,
		toolsJSON,
		int64(template.EstimatedInstallTime),
		template.RecommendedUseCase,
		template.MinResources,
	)
	if err != nil {
		return fmt.Errorf("update template: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("template %q not found", template.ID)
	}

	return nil
}

func (r *PostgresTemplateRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM golden_path_templates WHERE id = $1`

	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("template %q not found", id)
	}

	return nil
}

func (r *PostgresTemplateRepository) GetByID(ctx context.Context, id string) (*domain.Template, error) {
	const q = `
		SELECT id, name, description, tools, estimated_install_time, recommended_use_case, min_resources
		FROM golden_path_templates
		WHERE id = $1`

	var (
		t                 domain.Template
		toolsJSON         []byte
		estimatedDuration int64
	)

	err := r.pool.QueryRow(ctx, q, id).Scan(
		&t.ID,
		&t.Name,
		&t.Description,
		&toolsJSON,
		&estimatedDuration,
		&t.RecommendedUseCase,
		&t.MinResources,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("template %q not found", id)
		}
		return nil, err
	}

	if err := json.Unmarshal(toolsJSON, &t.Tools); err != nil {
		return nil, fmt.Errorf("unmarshal tools: %w", err)
	}
	t.EstimatedInstallTime = time.Duration(estimatedDuration)

	return &t, nil
}

func (r *PostgresTemplateRepository) List(ctx context.Context) ([]*domain.Template, error) {
	const q = `
		SELECT id, name, description, tools, estimated_install_time, recommended_use_case, min_resources
		FROM golden_path_templates
		ORDER BY id ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*domain.Template
	for rows.Next() {
		var (
			t                 domain.Template
			toolsJSON         []byte
			estimatedDuration int64
		)

		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Description,
			&toolsJSON,
			&estimatedDuration,
			&t.RecommendedUseCase,
			&t.MinResources,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(toolsJSON, &t.Tools); err != nil {
			return nil, fmt.Errorf("unmarshal tools: %w", err)
		}
		t.EstimatedInstallTime = time.Duration(estimatedDuration)

		templates = append(templates, &t)
	}

	return templates, rows.Err()
}
