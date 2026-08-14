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

type PostgresDeploymentRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresDeploymentRepository(pool *pgxpool.Pool) *PostgresDeploymentRepository {
	return &PostgresDeploymentRepository{pool: pool}
}

func (r *PostgresDeploymentRepository) Create(ctx context.Context, d *domain.Deployment) error {
	const q = `
		INSERT INTO pipeline_deployments (id, pipeline_id, version, status, started_at, completed_at, deployed_by, steps)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	steps, err := marshalSteps(d.Steps)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, q,
		d.ID,
		d.PipelineID,
		d.Version,
		string(d.Status),
		d.StartedAt,
		d.CompletedAt,
		d.DeployedBy,
		steps,
	)
	return err
}

func (r *PostgresDeploymentRepository) GetByID(ctx context.Context, id string) (*domain.Deployment, error) {
	const q = `
		SELECT id, pipeline_id, version, status, started_at, completed_at, deployed_by, steps
		FROM pipeline_deployments
		WHERE id = $1`

	d, err := scanDeployment(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("deployment %q not found", id)
		}
		return nil, err
	}

	return d, nil
}

func (r *PostgresDeploymentRepository) ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.Deployment, error) {
	const q = `
		SELECT id, pipeline_id, version, status, started_at, completed_at, deployed_by, steps
		FROM pipeline_deployments
		WHERE pipeline_id = $1
		ORDER BY started_at DESC`

	rows, err := r.pool.Query(ctx, q, pipelineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*domain.Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}

	return deployments, rows.Err()
}

func (r *PostgresDeploymentRepository) Update(ctx context.Context, d *domain.Deployment) error {
	const q = `
		UPDATE pipeline_deployments
		SET version = $2, status = $3, started_at = $4, completed_at = $5, deployed_by = $6, steps = $7
		WHERE id = $1`

	steps, err := marshalSteps(d.Steps)
	if err != nil {
		return err
	}

	res, err := r.pool.Exec(ctx, q,
		d.ID,
		d.Version,
		string(d.Status),
		d.StartedAt,
		d.CompletedAt,
		d.DeployedBy,
		steps,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("deployment %q not found", d.ID)
	}

	return nil
}

type deploymentScanner interface {
	Scan(dest ...any) error
}

func scanDeployment(row deploymentScanner) (*domain.Deployment, error) {
	var (
		d      domain.Deployment
		status string
	)

	var steps []byte
	err := row.Scan(
		&d.ID,
		&d.PipelineID,
		&d.Version,
		&status,
		&d.StartedAt,
		&d.CompletedAt,
		&d.DeployedBy,
		&steps,
	)
	if err != nil {
		return nil, err
	}

	d.Status = domain.DeploymentStatus(status)
	// 단계 해석 실패로 배포 기록 전체를 잃지 않는다 — 단계는 부가 정보다.
	if len(steps) > 0 {
		_ = json.Unmarshal(steps, &d.Steps)
	}

	return &d, nil
}

// marshalSteps 는 단계를 JSONB 로 옮긴다. 비면 빈 배열이다 —
// NULL 을 넣으면 스캔 쪽에서 매번 분기해야 한다.
func marshalSteps(steps []domain.DeployStep) ([]byte, error) {
	if len(steps) == 0 {
		return []byte("[]"), nil
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		return nil, fmt.Errorf("marshal deployment steps: %w", err)
	}
	return raw, nil
}
