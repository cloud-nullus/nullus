package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

type PostgresTemplateRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTemplateRepository(pool *pgxpool.Pool) *PostgresTemplateRepository {
	return &PostgresTemplateRepository{pool: pool}
}

// estimated_install_time 컬럼은 분 단위다 — 시드 마이그레이션이 110 같은 분 값을
// 넣는다. 도메인은 time.Duration 이라 경계에서 바꾼다. 나노초를 그대로 쓰면 int4 를
// 넘쳐 템플릿 생성이 거절되고, 시드 템플릿은 110ns 로 읽힌다.
func installMinutes(d time.Duration) int {
	return int(d / time.Minute)
}

func installDuration(minutes int) time.Duration {
	return time.Duration(minutes) * time.Minute
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
			min_resources,
			planning_profile
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = r.pool.Exec(
		ctx,
		q,
		template.ID,
		template.Name,
		template.Description,
		toolsJSON,
		installMinutes(template.EstimatedInstallTime),
		template.RecommendedUseCase,
		template.MinResources,
		domain.NormalizePlanningProfile(template.PlanningProfile),
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
			planning_profile = $8,
			updated_at = NOW()
		WHERE id = $1`

	ct, err := r.pool.Exec(
		ctx,
		q,
		template.ID,
		template.Name,
		template.Description,
		toolsJSON,
		installMinutes(template.EstimatedInstallTime),
		template.RecommendedUseCase,
		template.MinResources,
		domain.NormalizePlanningProfile(template.PlanningProfile),
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
		SELECT id, name, description, tools, estimated_install_time, recommended_use_case, min_resources, planning_profile
		FROM golden_path_templates
		WHERE id = $1`

	var (
		t                domain.Template
		toolsJSON        []byte
		estimatedMinutes int
	)

	err := r.pool.QueryRow(ctx, q, id).Scan(
		&t.ID,
		&t.Name,
		&t.Description,
		&toolsJSON,
		&estimatedMinutes,
		&t.RecommendedUseCase,
		&t.MinResources,
		&t.PlanningProfile,
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
	t.EstimatedInstallTime = installDuration(estimatedMinutes)

	return &t, nil
}

func (r *PostgresTemplateRepository) List(ctx context.Context) ([]*domain.Template, error) {
	const q = `
		SELECT id, name, description, tools, estimated_install_time, recommended_use_case, min_resources, planning_profile
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
			t                domain.Template
			toolsJSON        []byte
			estimatedMinutes int
		)

		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Description,
			&toolsJSON,
			&estimatedMinutes,
			&t.RecommendedUseCase,
			&t.MinResources,
			&t.PlanningProfile,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(toolsJSON, &t.Tools); err != nil {
			return nil, fmt.Errorf("unmarshal tools: %w", err)
		}
		t.EstimatedInstallTime = installDuration(estimatedMinutes)

		templates = append(templates, &t)
	}

	return templates, rows.Err()
}
