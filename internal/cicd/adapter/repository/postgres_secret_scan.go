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

// PostgresSecretScanResultRepository 는 port.SecretScanResultRepository 의 pgx 구현이다.
type PostgresSecretScanResultRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresSecretScanResultRepository 는 저장소를 만든다.
func NewPostgresSecretScanResultRepository(pool *pgxpool.Pool) *PostgresSecretScanResultRepository {
	return &PostgresSecretScanResultRepository{pool: pool}
}

const secretScanColumns = `id, pipeline_id, COALESCE(deployment_id, ''),
		       scanner, scanner_version, findings_count, rules, findings,
		       gate_result, report_uri, scanned_at, report_ref`

// Upsert 는 같은 ID 면 덮어쓴다. 탐지 건수는 모르면 NULL 이다 — 0 은 "탐지 없음" 으로 읽힌다.
//
// 탐지 위치는 상한(domain.MaxStoredSecretFindings)까지만 둔다. 동기화가 이미 자르지만 저장소도 지킨다 —
// 다른 경로가 생겨도 행이 커지지 않는다.
func (r *PostgresSecretScanResultRepository) Upsert(ctx context.Context, s *domain.SecretScanResult) error {
	const q = `
		INSERT INTO secret_scan_results (
			id, pipeline_id, deployment_id,
			scanner, scanner_version, findings_count, rules, findings,
			gate_result, report_uri, scanned_at, report_ref)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET
			deployment_id = EXCLUDED.deployment_id,
			scanner = EXCLUDED.scanner,
			scanner_version = EXCLUDED.scanner_version,
			findings_count = EXCLUDED.findings_count,
			rules = EXCLUDED.rules,
			findings = EXCLUDED.findings,
			gate_result = EXCLUDED.gate_result,
			report_uri = EXCLUDED.report_uri,
			scanned_at = EXCLUDED.scanned_at,
			report_ref = EXCLUDED.report_ref`

	rules := s.Rules
	if rules == nil {
		rules = []domain.SecretRuleCount{}
	}
	findings := s.Findings
	if findings == nil {
		findings = []domain.SecretFinding{}
	}
	if len(findings) > domain.MaxStoredSecretFindings {
		findings = findings[:domain.MaxStoredSecretFindings]
	}
	rawRules, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("marshal secret scan rules: %w", err)
	}
	rawFindings, err := json.Marshal(findings)
	if err != nil {
		return fmt.Errorf("marshal secret scan findings: %w", err)
	}
	ref, err := marshalReportRef(s.ReportRef)
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, q,
		s.ID, s.PipelineID, nilIfEmpty(s.DeploymentID),
		s.Scanner, s.ScannerVersion, s.FindingsCount, rawRules, rawFindings,
		string(s.GateResult), s.ReportURI, s.ScannedAt, ref,
	); err != nil {
		return fmt.Errorf("upsert secret scan result: %w", err)
	}
	return nil
}

// Delete 는 기록을 지운다. 없으면 아무 일도 하지 않는다.
func (r *PostgresSecretScanResultRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM secret_scan_results WHERE id = $1`, strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("delete secret scan result: %w", err)
	}
	return nil
}

// ListByPipelineID 는 최신 결과부터 돌려준다.
func (r *PostgresSecretScanResultRepository) ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.SecretScanResult, error) {
	q := `SELECT ` + secretScanColumns + `
		FROM secret_scan_results
		WHERE pipeline_id = $1
		ORDER BY scanned_at DESC
		LIMIT 100`

	rows, err := r.pool.Query(ctx, q, strings.TrimSpace(pipelineID))
	if err != nil {
		return nil, fmt.Errorf("query secret scan results: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.SecretScanResult, 0)
	for rows.Next() {
		s, err := scanSecretScanResult(rows)
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

func scanSecretScanResult(row pgx.Row) (*domain.SecretScanResult, error) {
	var (
		s                          domain.SecretScanResult
		gate                       string
		rawRules, rawFindings, ref []byte
	)
	if err := row.Scan(
		&s.ID, &s.PipelineID, &s.DeploymentID,
		&s.Scanner, &s.ScannerVersion, &s.FindingsCount, &rawRules, &rawFindings,
		&gate, &s.ReportURI, &s.ScannedAt, &ref,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan secret scan result: %w", err)
	}
	s.ScannedAt = s.ScannedAt.UTC()
	s.GateResult = domain.GateResult(gate)
	s.Rules = []domain.SecretRuleCount{}
	if len(rawRules) > 0 {
		if err := json.Unmarshal(rawRules, &s.Rules); err != nil {
			return nil, fmt.Errorf("unmarshal secret scan rules: %w", err)
		}
	}
	s.Findings = []domain.SecretFinding{}
	if len(rawFindings) > 0 {
		if err := json.Unmarshal(rawFindings, &s.Findings); err != nil {
			return nil, fmt.Errorf("unmarshal secret scan findings: %w", err)
		}
	}
	if s.Rules == nil {
		s.Rules = []domain.SecretRuleCount{}
	}
	if s.Findings == nil {
		s.Findings = []domain.SecretFinding{}
	}
	parsed, err := unmarshalReportRef(ref)
	if err != nil {
		return nil, err
	}
	s.ReportRef = parsed
	return &s, nil
}
