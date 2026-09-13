package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresPipelineRepository implements port.PipelineRepository using pgx.
type PostgresPipelineRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPipelineRepository constructs a PostgresPipelineRepository.
func NewPostgresPipelineRepository(pool *pgxpool.Pool) *PostgresPipelineRepository {
	return &PostgresPipelineRepository{pool: pool}
}

// Create inserts a new pipeline record.
func (r *PostgresPipelineRepository) Create(ctx context.Context, p *domain.Pipeline) error {
	const q = `
		INSERT INTO pipelines (id, name, execution_mode, template_id, org_id, cluster_id, namespace, app_type, git_repo_url, dockerfile_path, docker_context, env_vars, status, created_at, stack_id, stages)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	stages, err := marshalStages(p.Stages)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, q,
		p.ID, p.Name, p.ExecutionMode, p.TemplateID, p.OrgID, p.ClusterID,
		p.Namespace, string(p.AppType), p.GitRepoURL, p.DockerfilePath, p.DockerContext, p.EnvVars, string(p.Status), p.CreatedAt,
		nilIfEmpty(p.StackID), stages,
	)
	if err != nil {
		return fmt.Errorf("insert pipeline: %w", err)
	}
	return nil
}

// marshalStages 는 단계 목록을 JSONB 로 옮긴다. nil 은 빈 배열로 저장한다 —
// 컬럼이 NOT NULL 이고, "모름" 은 빈 배열로 충분히 표현된다.
func marshalStages(stages []string) ([]byte, error) {
	if stages == nil {
		stages = []string{}
	}
	b, err := json.Marshal(stages)
	if err != nil {
		return nil, fmt.Errorf("marshal pipeline stages: %w", err)
	}
	return b, nil
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GetByID retrieves a pipeline by its ID.
func (r *PostgresPipelineRepository) GetByID(ctx context.Context, id string) (*domain.Pipeline, error) {
	const q = `
		SELECT id, name, execution_mode, template_id, org_id, cluster_id, namespace, app_type, git_repo_url,
		       COALESCE(dockerfile_path, ''), COALESCE(docker_context, ''), COALESCE(env_vars, '{}'::jsonb),
		       status, created_at, COALESCE(stack_id, ''), COALESCE(stages, '[]'::jsonb)
		FROM pipelines WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	return scanPipeline(row)
}

// List returns all pipelines belonging to an organization.
func (r *PostgresPipelineRepository) List(ctx context.Context, orgID string) ([]*domain.Pipeline, error) {
	const q = `
		SELECT id, name, execution_mode, template_id, org_id, cluster_id, namespace, app_type, git_repo_url,
		       COALESCE(dockerfile_path, ''), COALESCE(docker_context, ''), COALESCE(env_vars, '{}'::jsonb),
		       status, created_at, COALESCE(stack_id, ''), COALESCE(stages, '[]'::jsonb)
		FROM pipelines WHERE org_id = $1 ORDER BY created_at DESC LIMIT 100`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("query pipelines: %w", err)
	}
	defer rows.Close()

	var pipelines []*domain.Pipeline
	for rows.Next() {
		p, err := scanPipeline(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pipeline: %w", err)
		}
		pipelines = append(pipelines, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return pipelines, nil
}

// ListByStackID returns all pipelines linked to a specific stack.
func (r *PostgresPipelineRepository) ListByStackID(ctx context.Context, stackID string) ([]*domain.Pipeline, error) {
	const q = `
		SELECT id, name, execution_mode, template_id, org_id, cluster_id, namespace, app_type, git_repo_url,
		       COALESCE(dockerfile_path, ''), COALESCE(docker_context, ''), COALESCE(env_vars, '{}'::jsonb),
		       status, created_at, COALESCE(stack_id, ''), COALESCE(stages, '[]'::jsonb)
		FROM pipelines WHERE stack_id = $1 ORDER BY created_at DESC LIMIT 100`

	rows, err := r.pool.Query(ctx, q, stackID)
	if err != nil {
		return nil, fmt.Errorf("query pipelines by stack: %w", err)
	}
	defer rows.Close()

	var pipelines []*domain.Pipeline
	for rows.Next() {
		p, err := scanPipeline(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pipeline: %w", err)
		}
		pipelines = append(pipelines, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return pipelines, nil
}

// Update persists changes to an existing pipeline.
func (r *PostgresPipelineRepository) Update(ctx context.Context, p *domain.Pipeline) error {
	const q = `
		UPDATE pipelines
		SET name = $2, execution_mode = $3, template_id = $4, cluster_id = $5, namespace = $6,
		    app_type = $7, git_repo_url = $8, dockerfile_path = $9, docker_context = $10, env_vars = $11, status = $12, stack_id = $13,
		    stages = $14
		WHERE id = $1`

	stages, err := marshalStages(p.Stages)
	if err != nil {
		return err
	}
	res, err := r.pool.Exec(ctx, q,
		p.ID, p.Name, p.ExecutionMode, p.TemplateID, p.ClusterID,
		p.Namespace, string(p.AppType), p.GitRepoURL, p.DockerfilePath, p.DockerContext, p.EnvVars, string(p.Status),
		nilIfEmpty(p.StackID), stages,
	)
	if err != nil {
		return fmt.Errorf("update pipeline: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("pipeline %q not found", p.ID)
	}
	return nil
}

// Delete removes a pipeline by ID. Cascades to pipeline_deployments via FK.
func (r *PostgresPipelineRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM pipelines WHERE id = $1`
	res, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete pipeline: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("pipeline %q not found", id)
	}
	return nil
}

// pipelineScanner abstracts pgx row and rows scanning.
type pipelineScanner interface {
	Scan(dest ...any) error
}

func scanPipeline(row pipelineScanner) (*domain.Pipeline, error) {
	var (
		p         domain.Pipeline
		appType   string
		status    string
		createdAt time.Time
		stages    []byte
	)
	if err := row.Scan(
		&p.ID, &p.Name, &p.ExecutionMode, &p.TemplateID, &p.OrgID, &p.ClusterID,
		&p.Namespace, &appType, &p.GitRepoURL, &p.DockerfilePath, &p.DockerContext, &p.EnvVars, &status, &createdAt,
		&p.StackID, &stages,
	); err != nil {
		return nil, fmt.Errorf("scan pipeline: %w", err)
	}
	if len(stages) > 0 {
		if err := json.Unmarshal(stages, &p.Stages); err != nil {
			return nil, fmt.Errorf("unmarshal pipeline stages: %w", err)
		}
		// 빈 배열은 "모름" 이다. nil 로 두어 화면이 템플릿으로 떨어지게 한다.
		if len(p.Stages) == 0 {
			p.Stages = nil
		}
	}
	p.AppType = domain.AppType(appType)
	p.Status = domain.PipelineStatus(status)
	p.CreatedAt = createdAt
	return &p, nil
}
