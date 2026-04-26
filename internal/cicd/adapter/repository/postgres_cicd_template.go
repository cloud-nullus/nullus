package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

type PostgresCICDTemplateRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCICDTemplateRepository(pool *pgxpool.Pool) *PostgresCICDTemplateRepository {
	return &PostgresCICDTemplateRepository{pool: pool}
}

func (r *PostgresCICDTemplateRepository) GetByID(ctx context.Context, id string) (*domain.PipelineTemplate, error) {
	const q = `
		SELECT id, name, description, app_type, stages,
		       COALESCE(git_repo_url, ''), COALESCE(dockerfile_path, ''), COALESCE(docker_context, ''), COALESCE(env_vars, '{}'::jsonb)
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
		SELECT id, name, description, app_type, stages,
		       COALESCE(git_repo_url, ''), COALESCE(dockerfile_path, ''), COALESCE(docker_context, ''), COALESCE(env_vars, '{}'::jsonb)
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

// Create inserts a new pipeline template into the database.
// TODO: implement when pipeline_templates table supports mutations.
func (r *PostgresCICDTemplateRepository) Create(_ context.Context, tmpl *domain.PipelineTemplate) error {
	// TODO: INSERT INTO pipeline_templates (id, name, description, app_type, stages, created_by) VALUES (...)
	_ = tmpl
	return nil
}

// Update modifies an existing pipeline template in the database.
// TODO: implement when pipeline_templates table supports mutations.
func (r *PostgresCICDTemplateRepository) Update(_ context.Context, tmpl *domain.PipelineTemplate) error {
	// TODO: UPDATE pipeline_templates SET ... WHERE id = $1
	_ = tmpl
	return nil
}

// Delete removes a pipeline template from the database.
// TODO: implement when pipeline_templates table supports mutations.
func (r *PostgresCICDTemplateRepository) Delete(_ context.Context, id string) error {
	// TODO: DELETE FROM pipeline_templates WHERE id = $1
	_ = id
	return nil
}

type pipelineTemplateScanner interface {
	Scan(dest ...any) error
}

func scanPipelineTemplate(row pipelineTemplateScanner) (*domain.PipelineTemplate, error) {
	var (
		t          domain.PipelineTemplate
		appType    string
		stagesJSON []byte
		envJSON    []byte
	)

	err := row.Scan(
		&t.ID,
		&t.Name,
		&t.Description,
		&appType,
		&stagesJSON,
		&t.GitRepoURL,
		&t.DockerfilePath,
		&t.DockerContext,
		&envJSON,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(stagesJSON, &t.Stages); err != nil {
		return nil, fmt.Errorf("unmarshal stages: %w", err)
	}
	if len(envJSON) > 0 {
		_ = json.Unmarshal(envJSON, &t.EnvVars)
	}
	t.AppType = domain.AppType(appType)

	return &t, nil
}
