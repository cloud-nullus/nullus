package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCICDTemplateRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCICDTemplateRepository(pool *pgxpool.Pool) *PostgresCICDTemplateRepository {
	return &PostgresCICDTemplateRepository{pool: pool}
}

func (r *PostgresCICDTemplateRepository) GetByID(ctx context.Context, id string) (*domain.PipelineTemplate, error) {
	const q = `
		SELECT id, name, description, app_type, stages
		FROM pipeline_templates
		WHERE id = $1`

	t, err := scanPipelineTemplate(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("pipeline template %q not found", id)
		}
		return nil, err
	}

	return t, nil
}

func (r *PostgresCICDTemplateRepository) List(ctx context.Context) ([]*domain.PipelineTemplate, error) {
	const q = `
		SELECT id, name, description, app_type, stages
		FROM pipeline_templates
		ORDER BY id ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*domain.PipelineTemplate
	for rows.Next() {
		t, err := scanPipelineTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}

	return templates, rows.Err()
}

type pipelineTemplateScanner interface {
	Scan(dest ...any) error
}

func scanPipelineTemplate(row pipelineTemplateScanner) (*domain.PipelineTemplate, error) {
	var (
		t          domain.PipelineTemplate
		appType    string
		stagesJSON []byte
	)

	err := row.Scan(
		&t.ID,
		&t.Name,
		&t.Description,
		&appType,
		&stagesJSON,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(stagesJSON, &t.Stages); err != nil {
		return nil, fmt.Errorf("unmarshal stages: %w", err)
	}
	t.AppType = domain.AppType(appType)

	return &t, nil
}
