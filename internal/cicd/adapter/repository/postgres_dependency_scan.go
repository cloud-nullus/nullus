package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PostgresDependencyScanResultRepository 는 port.DependencyScanResultRepository 의 pgx 구현이다.
type PostgresDependencyScanResultRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresDependencyScanResultRepository 는 저장소를 만든다.
func NewPostgresDependencyScanResultRepository(pool *pgxpool.Pool) *PostgresDependencyScanResultRepository {
	return &PostgresDependencyScanResultRepository{pool: pool}
}

const dependencyScanColumns = `id, pipeline_id, COALESCE(deployment_id, ''),
		       scanner, scanner_version, db_updated_at,
		       critical_count, high_count, medium_count, low_count, unknown_count, targets,
		       gate_result, report_uri, scanned_at, report_ref`

// Upsert 는 같은 ID 면 덮어쓴다. 건수·검사 대상 수는 모르면 NULL 이다 — 0 은 "취약점 0건"·"대상 없음" 으로 읽힌다.
func (r *PostgresDependencyScanResultRepository) Upsert(ctx context.Context, s *domain.DependencyScanResult) error {
	const q = `
		INSERT INTO dependency_scan_results (
			id, pipeline_id, deployment_id,
			scanner, scanner_version, db_updated_at,
			critical_count, high_count, medium_count, low_count, unknown_count, targets,
			gate_result, report_uri, scanned_at, report_ref)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (id) DO UPDATE SET
			deployment_id = EXCLUDED.deployment_id,
			scanner = EXCLUDED.scanner,
			scanner_version = EXCLUDED.scanner_version,
			db_updated_at = EXCLUDED.db_updated_at,
			critical_count = EXCLUDED.critical_count,
			high_count = EXCLUDED.high_count,
			medium_count = EXCLUDED.medium_count,
			low_count = EXCLUDED.low_count,
			unknown_count = EXCLUDED.unknown_count,
			targets = EXCLUDED.targets,
			gate_result = EXCLUDED.gate_result,
			report_uri = EXCLUDED.report_uri,
			scanned_at = EXCLUDED.scanned_at,
			report_ref = EXCLUDED.report_ref`

	var crit, high, med, low, unk *int
	if s.Counts != nil {
		crit, high, med, low, unk = &s.Counts.Critical, &s.Counts.High, &s.Counts.Medium, &s.Counts.Low, &s.Counts.Unknown
	}
	ref, err := marshalReportRef(s.ReportRef)
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, q,
		s.ID, s.PipelineID, nilIfEmpty(s.DeploymentID),
		s.Scanner, s.ScannerVersion, s.DBUpdatedAt,
		crit, high, med, low, unk, s.Targets,
		string(s.GateResult), s.ReportURI, s.ScannedAt, ref,
	); err != nil {
		return fmt.Errorf("upsert dependency scan result: %w", err)
	}
	return nil
}

// Delete 는 기록을 지운다. 없으면 아무 일도 하지 않는다.
func (r *PostgresDependencyScanResultRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM dependency_scan_results WHERE id = $1`, strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("delete dependency scan result: %w", err)
	}
	return nil
}

// ListByPipelineID 는 최신 결과부터 돌려준다.
func (r *PostgresDependencyScanResultRepository) ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.DependencyScanResult, error) {
	q := `SELECT ` + dependencyScanColumns + `
		FROM dependency_scan_results
		WHERE pipeline_id = $1
		ORDER BY scanned_at DESC
		LIMIT 100`

	rows, err := r.pool.Query(ctx, q, strings.TrimSpace(pipelineID))
	if err != nil {
		return nil, fmt.Errorf("query dependency scan results: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.DependencyScanResult, 0)
	for rows.Next() {
		s, err := scanDependencyScanResult(rows)
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

func scanDependencyScanResult(row pgx.Row) (*domain.DependencyScanResult, error) {
	var (
		s                         domain.DependencyScanResult
		dbUpdatedAt               *time.Time
		crit, high, med, low, unk *int
		gate                      string
		ref                       []byte
	)
	if err := row.Scan(
		&s.ID, &s.PipelineID, &s.DeploymentID,
		&s.Scanner, &s.ScannerVersion, &dbUpdatedAt,
		&crit, &high, &med, &low, &unk, &s.Targets,
		&gate, &s.ReportURI, &s.ScannedAt, &ref,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan dependency scan result: %w", err)
	}
	s.ScannedAt = s.ScannedAt.UTC()
	if dbUpdatedAt != nil {
		t := dbUpdatedAt.UTC()
		s.DBUpdatedAt = &t
	}
	s.GateResult = domain.GateResult(gate)
	// 건수는 다섯 칸이 함께 채워지거나 함께 비어 있다. 하나라도 비었으면 모른다.
	if crit != nil && high != nil && med != nil && low != nil && unk != nil {
		s.Counts = &domain.SeverityCounts{Critical: *crit, High: *high, Medium: *med, Low: *low, Unknown: *unk}
	}
	parsed, err := unmarshalReportRef(ref)
	if err != nil {
		return nil, err
	}
	s.ReportRef = parsed
	return &s, nil
}

// marshalReportRef 는 리포트 위치를 JSONB 로 옮긴다. 없으면 NULL 이다.
func marshalReportRef(ref *domain.ScanReportRef) (any, error) {
	if ref == nil {
		return nil, nil
	}
	raw, err := json.Marshal(ref)
	if err != nil {
		return nil, fmt.Errorf("marshal report ref: %w", err)
	}
	return raw, nil
}

func unmarshalReportRef(raw []byte) (*domain.ScanReportRef, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var r domain.ScanReportRef
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("unmarshal report ref: %w", err)
	}
	return &r, nil
}
