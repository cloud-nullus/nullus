package repository

import (
	"context"
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
		INSERT INTO pipelines (id, name, template_id, org_id, cluster_id, namespace, app_type, git_repo_url, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, q,
		p.ID, p.Name, p.TemplateID, p.OrgID, p.ClusterID,
		p.Namespace, string(p.AppType), p.GitRepoURL, string(p.Status), p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert pipeline: %w", err)
	}
	return nil
}

// GetByID retrieves a pipeline by its ID.
func (r *PostgresPipelineRepository) GetByID(ctx context.Context, id string) (*domain.Pipeline, error) {
	const q = `
		SELECT id, name, template_id, org_id, cluster_id, namespace, app_type, git_repo_url, status, created_at
		FROM pipelines WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	return scanPipeline(row)
}

// List returns all pipelines belonging to an organization.
func (r *PostgresPipelineRepository) List(ctx context.Context, orgID string) ([]*domain.Pipeline, error) {
	const q = `
		SELECT id, name, template_id, org_id, cluster_id, namespace, app_type, git_repo_url, status, created_at
		FROM pipelines WHERE org_id = $1 ORDER BY created_at DESC`

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

// Update persists changes to an existing pipeline.
func (r *PostgresPipelineRepository) Update(ctx context.Context, p *domain.Pipeline) error {
	const q = `
		UPDATE pipelines
		SET name = $2, template_id = $3, cluster_id = $4, namespace = $5,
		    app_type = $6, git_repo_url = $7, status = $8
		WHERE id = $1`

	res, err := r.pool.Exec(ctx, q,
		p.ID, p.Name, p.TemplateID, p.ClusterID,
		p.Namespace, string(p.AppType), p.GitRepoURL, string(p.Status),
	)
	if err != nil {
		return fmt.Errorf("update pipeline: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("pipeline %q not found", p.ID)
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
	)
	if err := row.Scan(
		&p.ID, &p.Name, &p.TemplateID, &p.OrgID, &p.ClusterID,
		&p.Namespace, &appType, &p.GitRepoURL, &status, &createdAt,
	); err != nil {
		return nil, fmt.Errorf("scan pipeline: %w", err)
	}
	p.AppType = domain.AppType(appType)
	p.Status = domain.PipelineStatus(status)
	p.CreatedAt = createdAt
	return &p, nil
}
