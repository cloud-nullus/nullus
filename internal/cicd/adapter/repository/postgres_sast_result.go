package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PostgresSASTResultRepository 는 port.SASTResultRepository 의 pgx 구현이다.
type PostgresSASTResultRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresSASTResultRepository 는 저장소를 만든다.
func NewPostgresSASTResultRepository(pool *pgxpool.Pool) *PostgresSASTResultRepository {
	return &PostgresSASTResultRepository{pool: pool}
}

const sastResultColumns = `id, pipeline_id, COALESCE(deployment_id, ''),
		       project_key, analysis_id, quality_gate_status, gate_result, conditions,
		       bugs, vulnerabilities, code_smells, security_hotspots,
		       coverage, duplicated_lines_density, ncloc,
		       dashboard_url, analyzed_at`

// Upsert 는 같은 ID 면 덮어쓴다. 지표는 Metrics 가 nil 이거나 그 지표가 없으면 NULL 이다.
func (r *PostgresSASTResultRepository) Upsert(ctx context.Context, s *domain.SASTResult) error {
	const q = `
		INSERT INTO sast_results (
			id, pipeline_id, deployment_id,
			project_key, analysis_id, quality_gate_status, gate_result, conditions,
			bugs, vulnerabilities, code_smells, security_hotspots,
			coverage, duplicated_lines_density, ncloc,
			dashboard_url, analyzed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (id) DO UPDATE SET
			deployment_id = EXCLUDED.deployment_id,
			project_key = EXCLUDED.project_key,
			analysis_id = EXCLUDED.analysis_id,
			quality_gate_status = EXCLUDED.quality_gate_status,
			gate_result = EXCLUDED.gate_result,
			conditions = EXCLUDED.conditions,
			bugs = EXCLUDED.bugs,
			vulnerabilities = EXCLUDED.vulnerabilities,
			code_smells = EXCLUDED.code_smells,
			security_hotspots = EXCLUDED.security_hotspots,
			coverage = EXCLUDED.coverage,
			duplicated_lines_density = EXCLUDED.duplicated_lines_density,
			ncloc = EXCLUDED.ncloc,
			dashboard_url = EXCLUDED.dashboard_url,
			analyzed_at = EXCLUDED.analyzed_at`

	conditions := s.Conditions
	if conditions == nil {
		conditions = []domain.SASTCondition{}
	}
	rawConditions, err := json.Marshal(conditions)
	if err != nil {
		return fmt.Errorf("marshal sast conditions: %w", err)
	}
	m := s.Metrics
	if m == nil {
		m = &domain.SASTMetrics{}
	}
	if _, err := r.pool.Exec(ctx, q,
		s.ID, s.PipelineID, nilIfEmpty(s.DeploymentID),
		s.ProjectKey, s.AnalysisID, string(s.QualityGateStatus), string(s.GateResult), rawConditions,
		m.Bugs, m.Vulnerabilities, m.CodeSmells, m.SecurityHotspots,
		m.Coverage, m.DuplicatedLinesDensity, m.Ncloc,
		s.DashboardURL, s.AnalyzedAt,
	); err != nil {
		return fmt.Errorf("upsert sast result: %w", err)
	}
	return nil
}

// ListByPipelineID 는 최신 분석부터 돌려준다.
func (r *PostgresSASTResultRepository) ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.SASTResult, error) {
	q := `SELECT ` + sastResultColumns + `
		FROM sast_results
		WHERE pipeline_id = $1
		ORDER BY analyzed_at DESC
		LIMIT 100`

	rows, err := r.pool.Query(ctx, q, strings.TrimSpace(pipelineID))
	if err != nil {
		return nil, fmt.Errorf("query sast results: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.SASTResult, 0)
	for rows.Next() {
		s, err := scanSASTResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return out, nil
}

func scanSASTResult(row pgx.Row) (*domain.SASTResult, error) {
	var (
		s             domain.SASTResult
		qg, gate      string
		rawConditions []byte
		m             domain.SASTMetrics
	)
	if err := row.Scan(
		&s.ID, &s.PipelineID, &s.DeploymentID,
		&s.ProjectKey, &s.AnalysisID, &qg, &gate, &rawConditions,
		&m.Bugs, &m.Vulnerabilities, &m.CodeSmells, &m.SecurityHotspots,
		&m.Coverage, &m.DuplicatedLinesDensity, &m.Ncloc,
		&s.DashboardURL, &s.AnalyzedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan sast result: %w", err)
	}
	s.AnalyzedAt = s.AnalyzedAt.UTC()
	s.QualityGateStatus = domain.QualityGateStatus(qg)
	s.GateResult = domain.GateResult(gate)
	if len(rawConditions) > 0 {
		if err := json.Unmarshal(rawConditions, &s.Conditions); err != nil {
			return nil, fmt.Errorf("unmarshal sast conditions: %w", err)
		}
	}
	if len(s.Conditions) == 0 {
		s.Conditions = nil
	}
	// 지표가 하나도 없으면 모르는 것이다(분석하지 못한 실행).
	if m != (domain.SASTMetrics{}) {
		s.Metrics = &m
	}
	return &s, nil
}
