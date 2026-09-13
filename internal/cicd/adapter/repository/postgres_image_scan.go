package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PostgresImageScanResultRepository 는 port.ImageScanResultRepository 의 pgx 구현이다.
type PostgresImageScanResultRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresImageScanResultRepository 는 저장소를 만든다.
func NewPostgresImageScanResultRepository(pool *pgxpool.Pool) *PostgresImageScanResultRepository {
	return &PostgresImageScanResultRepository{pool: pool}
}

// Upsert 는 같은 ID 면 덮어쓴다. 같은 실행을 여러 번 동기화해도 기록이 늘지 않는다.
//
// 건수는 Counts 가 nil 이면 NULL 로 둔다 — 0 은 "취약점 0건" 으로 읽힌다.
func (r *PostgresImageScanResultRepository) Upsert(ctx context.Context, s *domain.ImageScanResult) error {
	const q = `
		INSERT INTO image_scan_results (
			id, pipeline_id, deployment_id,
			image_repository, image_tag, image_digest,
			scan_source, scanner, scanner_version, db_updated_at,
			critical_count, high_count, medium_count, low_count, unknown_count,
			gate_result, report_uri, scanned_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (id) DO UPDATE SET
			deployment_id = EXCLUDED.deployment_id,
			image_repository = EXCLUDED.image_repository,
			image_tag = EXCLUDED.image_tag,
			image_digest = EXCLUDED.image_digest,
			scan_source = EXCLUDED.scan_source,
			scanner = EXCLUDED.scanner,
			scanner_version = EXCLUDED.scanner_version,
			db_updated_at = EXCLUDED.db_updated_at,
			critical_count = EXCLUDED.critical_count,
			high_count = EXCLUDED.high_count,
			medium_count = EXCLUDED.medium_count,
			low_count = EXCLUDED.low_count,
			unknown_count = EXCLUDED.unknown_count,
			gate_result = EXCLUDED.gate_result,
			report_uri = EXCLUDED.report_uri,
			scanned_at = EXCLUDED.scanned_at`

	var crit, high, med, low, unk *int
	if s.Counts != nil {
		crit, high, med, low, unk = &s.Counts.Critical, &s.Counts.High, &s.Counts.Medium, &s.Counts.Low, &s.Counts.Unknown
	}
	_, err := r.pool.Exec(ctx, q,
		s.ID, s.PipelineID, nilIfEmpty(s.DeploymentID),
		s.ImageRepository, s.ImageTag, s.ImageDigest,
		s.ScanSource, s.Scanner, s.ScannerVersion, s.DBUpdatedAt,
		crit, high, med, low, unk,
		string(s.GateResult), s.ReportURI, s.ScannedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert image scan result: %w", err)
	}
	return nil
}

// Delete 는 기록을 지운다. 없으면 아무 일도 하지 않는다.
func (r *PostgresImageScanResultRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM image_scan_results WHERE id = $1`, strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("delete image scan result: %w", err)
	}
	return nil
}

// ListByPipelineID 는 최신 결과부터 돌려준다.
func (r *PostgresImageScanResultRepository) ListByPipelineID(ctx context.Context, pipelineID string) ([]*domain.ImageScanResult, error) {
	const q = `
		SELECT id, pipeline_id, COALESCE(deployment_id, ''),
		       image_repository, image_tag, image_digest,
		       scan_source, scanner, scanner_version, db_updated_at,
		       critical_count, high_count, medium_count, low_count, unknown_count,
		       gate_result, report_uri, scanned_at
		FROM image_scan_results
		WHERE pipeline_id = $1
		ORDER BY scanned_at DESC
		LIMIT 100`

	rows, err := r.pool.Query(ctx, q, strings.TrimSpace(pipelineID))
	if err != nil {
		return nil, fmt.Errorf("query image scan results: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.ImageScanResult, 0)
	for rows.Next() {
		var (
			s                         domain.ImageScanResult
			dbUpdatedAt               *time.Time
			crit, high, med, low, unk *int
			gate                      string
		)
		if err := rows.Scan(
			&s.ID, &s.PipelineID, &s.DeploymentID,
			&s.ImageRepository, &s.ImageTag, &s.ImageDigest,
			&s.ScanSource, &s.Scanner, &s.ScannerVersion, &dbUpdatedAt,
			&crit, &high, &med, &low, &unk,
			&gate, &s.ReportURI, &s.ScannedAt,
		); err != nil {
			return nil, fmt.Errorf("scan image scan result: %w", err)
		}
		s.DBUpdatedAt = dbUpdatedAt
		s.GateResult = domain.GateResult(gate)
		// 건수는 다섯 칸이 함께 채워지거나 함께 비어 있다. 하나라도 비었으면 모른다.
		if crit != nil && high != nil && med != nil && low != nil && unk != nil {
			s.Counts = &domain.SeverityCounts{Critical: *crit, High: *high, Medium: *med, Low: *low, Unknown: *unk}
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return out, nil
}
