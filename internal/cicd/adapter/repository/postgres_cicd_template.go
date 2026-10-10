package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// pgUniqueViolation 은 PostgreSQL 의 unique_violation SQLSTATE 다.
const pgUniqueViolation = "23505"

type PostgresCICDTemplateRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCICDTemplateRepository(pool *pgxpool.Pool) *PostgresCICDTemplateRepository {
	return &PostgresCICDTemplateRepository{pool: pool}
}

// pipelineTemplateColumns 는 GetByID 와 List 가 같은 순서로 읽는 열이다. scanPipelineTemplate 과 맞춘다.
const pipelineTemplateColumns = `
		id, name, COALESCE(description, ''), app_type, stages,
		COALESCE(git_repo_url, ''), COALESCE(dockerfile_path, ''), COALESCE(docker_context, ''),
		COALESCE(env_vars, '{}'::jsonb), COALESCE(created_by, '')`

func (r *PostgresCICDTemplateRepository) GetByID(ctx context.Context, id string) (*domain.PipelineTemplate, error) {
	const q = `SELECT ` + pipelineTemplateColumns + `
		FROM pipeline_templates
		WHERE id = $1`

	t, err := scanPipelineTemplate(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, templateNotFound(id)
		}
		return nil, err
	}

	return t, nil
}

func (r *PostgresCICDTemplateRepository) List(ctx context.Context) ([]*domain.PipelineTemplate, error) {
	const q = `SELECT ` + pipelineTemplateColumns + `
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

// Create 는 새 템플릿을 넣는다. 같은 ID 가 있으면 덮어쓰지 않고 domain.ErrTemplateAlreadyExists 를 돌려준다.
func (r *PostgresCICDTemplateRepository) Create(ctx context.Context, tmpl *domain.PipelineTemplate) error {
	const q = `
		INSERT INTO pipeline_templates
			(id, name, description, app_type, stages, git_repo_url, dockerfile_path, docker_context, env_vars, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	stages, envVars, err := marshalTemplateJSON(tmpl)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, q,
		tmpl.ID, tmpl.Name, tmpl.Description, string(tmpl.AppType), stages,
		tmpl.GitRepoURL, tmpl.DockerfilePath, tmpl.DockerContext, envVars, nilIfEmpty(tmpl.CreatedBy),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("pipeline template %q: %w", tmpl.ID, domain.ErrTemplateAlreadyExists)
		}
		return fmt.Errorf("create pipeline template: %w", err)
	}
	return nil
}

// Update 는 ID 가 같은 템플릿의 모든 열을 바꾼다. 없으면 domain.ErrTemplateNotFound 다.
func (r *PostgresCICDTemplateRepository) Update(ctx context.Context, tmpl *domain.PipelineTemplate) error {
	const q = `
		UPDATE pipeline_templates
		SET name = $2, description = $3, app_type = $4, stages = $5,
		    git_repo_url = $6, dockerfile_path = $7, docker_context = $8, env_vars = $9, created_by = $10
		WHERE id = $1`

	stages, envVars, err := marshalTemplateJSON(tmpl)
	if err != nil {
		return err
	}
	res, err := r.pool.Exec(ctx, q,
		tmpl.ID, tmpl.Name, tmpl.Description, string(tmpl.AppType), stages,
		tmpl.GitRepoURL, tmpl.DockerfilePath, tmpl.DockerContext, envVars, nilIfEmpty(tmpl.CreatedBy),
	)
	if err != nil {
		return fmt.Errorf("update pipeline template: %w", err)
	}
	if res.RowsAffected() == 0 {
		return templateNotFound(tmpl.ID)
	}
	return nil
}

// Delete 는 템플릿을 지운다. 없으면 domain.ErrTemplateNotFound 다.
func (r *PostgresCICDTemplateRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM pipeline_templates WHERE id = $1`
	res, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete pipeline template: %w", err)
	}
	if res.RowsAffected() == 0 {
		return templateNotFound(id)
	}
	return nil
}

func templateNotFound(id string) error {
	return fmt.Errorf("pipeline template %q: %w", id, domain.ErrTemplateNotFound)
}

// marshalTemplateJSON 은 stages 와 env_vars 를 JSONB 열에 넣을 모양으로 만든다.
// nil 은 [] 와 {} 로 둔다 — 열이 NOT NULL 이거나 읽는 쪽이 객체를 기대한다.
func marshalTemplateJSON(tmpl *domain.PipelineTemplate) ([]byte, []byte, error) {
	stageList := tmpl.Stages
	if stageList == nil {
		stageList = []string{}
	}
	stages, err := json.Marshal(stageList)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal stages: %w", err)
	}
	env := tmpl.EnvVars
	if env == nil {
		env = map[string]string{}
	}
	envVars, err := json.Marshal(env)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal env_vars: %w", err)
	}
	return stages, envVars, nil
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
		&t.CreatedBy,
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
