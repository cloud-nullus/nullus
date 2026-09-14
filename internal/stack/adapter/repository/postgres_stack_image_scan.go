package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// PostgresStackImageScanRepository 는 설치 이미지 스캔 결과를 보관한다.
type PostgresStackImageScanRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresStackImageScanRepository 는 저장소를 만든다.
func NewPostgresStackImageScanRepository(pool *pgxpool.Pool) *PostgresStackImageScanRepository {
	return &PostgresStackImageScanRepository{pool: pool}
}

// ReplaceForStack 은 스택의 결과를 이번 스캔으로 통째로 바꾼다.
func (r *PostgresStackImageScanRepository) ReplaceForStack(ctx context.Context, stackID string, scans []domain.StackImageScan) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin stack image scan replace: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM stack_image_scans WHERE stack_id = $1`, stackID); err != nil {
		return fmt.Errorf("clear stack image scans: %w", err)
	}

	const q = `
		INSERT INTO stack_image_scans (
			stack_id, image_digest, image, release_name, workloads, status, error,
			counts, fixable_counts, scanner, scanner_version, db_updated_at, scanned_at,
			vulnerabilities_recorded
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'trivy', $10, $11, $12, $13)
		ON CONFLICT (stack_id, image_digest) DO NOTHING`
	for _, s := range scans {
		workloads := s.Workloads
		if workloads == nil {
			workloads = []string{}
		}
		workloadsJSON, err := json.Marshal(workloads)
		if err != nil {
			return fmt.Errorf("marshal workloads: %w", err)
		}
		counts, err := countsJSON(s.Counts)
		if err != nil {
			return err
		}
		fixable, err := countsJSON(s.FixableCounts)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, q,
			stackID, s.ImageDigest, s.Image, s.Release, workloadsJSON, string(s.Status), s.Error,
			counts, fixable, s.ScannerVersion, s.DBUpdatedAt, s.ScannedAt, s.VulnerabilitiesRecorded,
		); err != nil {
			return fmt.Errorf("insert stack image scan (%s): %w", s.ImageDigest, err)
		}
	}

	// 취약점 목록. GitLab 스택은 이미지 서른 개에 수만 건이라 COPY 로 한 번에 넣는다.
	// 스캔 결과 행을 지우면 목록은 함께 지워진다(ON DELETE CASCADE).
	var vulnRows [][]any
	seen := map[string]bool{}
	for _, s := range scans {
		if !s.VulnerabilitiesRecorded || seen[s.ImageDigest] {
			continue
		}
		seen[s.ImageDigest] = true
		for _, v := range s.Vulnerabilities {
			vulnRows = append(vulnRows, []any{
				stackID, s.ImageDigest, v.ID, v.PkgName, v.InstalledVersion, v.FixedVersion,
				v.Severity, string(v.Class), v.Target, v.PrimaryURL,
			})
		}
	}
	if len(vulnRows) > 0 {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"stack_image_vulnerabilities"},
			[]string{"stack_id", "image_digest", "vulnerability_id", "pkg_name", "installed_version",
				"fixed_version", "severity", "class", "target", "primary_url"},
			pgx.CopyFromRows(vulnRows),
		); err != nil {
			return fmt.Errorf("copy stack image vulnerabilities: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// ListByStack 은 스택의 결과를 돌려준다.
func (r *PostgresStackImageScanRepository) ListByStack(ctx context.Context, stackID string) ([]domain.StackImageScan, error) {
	const q = `
		SELECT stack_id, image_digest, image, release_name, workloads, status, error,
			counts, fixable_counts, scanner_version, db_updated_at, scanned_at, vulnerabilities_recorded
		FROM stack_image_scans
		WHERE stack_id = $1
		ORDER BY image_digest`
	rows, err := r.pool.Query(ctx, q, stackID)
	if err != nil {
		return nil, fmt.Errorf("query stack image scans: %w", err)
	}
	defer rows.Close()

	var out []domain.StackImageScan
	for rows.Next() {
		var (
			s                     domain.StackImageScan
			status                string
			workloads             []byte
			counts, fixableCounts []byte
		)
		if err := rows.Scan(&s.StackID, &s.ImageDigest, &s.Image, &s.Release, &workloads, &status, &s.Error,
			&counts, &fixableCounts, &s.ScannerVersion, &s.DBUpdatedAt, &s.ScannedAt, &s.VulnerabilitiesRecorded); err != nil {
			return nil, fmt.Errorf("scan stack image scan: %w", err)
		}
		s.Status = domain.ImageScanStatus(status)
		if err := json.Unmarshal(workloads, &s.Workloads); err != nil {
			return nil, fmt.Errorf("unmarshal workloads: %w", err)
		}
		if s.Counts, err = parseCounts(counts); err != nil {
			return nil, err
		}
		if s.FixableCounts, err = parseCounts(fixableCounts); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListVulnerabilities 는 이미지 하나의 저장된 취약점 목록이다.
func (r *PostgresStackImageScanRepository) ListVulnerabilities(ctx context.Context, stackID, digest string) (domain.StackImageVulnerabilities, error) {
	var out domain.StackImageVulnerabilities
	err := r.pool.QueryRow(ctx,
		`SELECT vulnerabilities_recorded FROM stack_image_scans WHERE stack_id = $1 AND image_digest = $2`,
		stackID, digest).Scan(&out.Recorded)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("query stack image scan: %w", err)
	}
	out.Found = true
	out.Items = []shareddomain.ImageVulnerability{}
	if !out.Recorded {
		return out, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT vulnerability_id, pkg_name, installed_version, fixed_version, severity, class, target, primary_url
		FROM stack_image_vulnerabilities
		WHERE stack_id = $1 AND image_digest = $2`, stackID, digest)
	if err != nil {
		return out, fmt.Errorf("query stack image vulnerabilities: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			v     shareddomain.ImageVulnerability
			class string
		)
		if err := rows.Scan(&v.ID, &v.PkgName, &v.InstalledVersion, &v.FixedVersion, &v.Severity, &class,
			&v.Target, &v.PrimaryURL); err != nil {
			return out, fmt.Errorf("scan stack image vulnerability: %w", err)
		}
		v.Class = shareddomain.VulnerabilityClass(class)
		out.Items = append(out.Items, v)
	}
	return out, rows.Err()
}

// countsJSON 은 건수를 jsonb 로 옮긴다. nil 은 NULL 이다 — 모르는 것을 0 으로 쓰지 않는다.
func countsJSON(c *shareddomain.SeverityCounts) (any, error) {
	if c == nil {
		return nil, nil
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal severity counts: %w", err)
	}
	return raw, nil
}

func parseCounts(raw []byte) (*shareddomain.SeverityCounts, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var c shareddomain.SeverityCounts
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("unmarshal severity counts: %w", err)
	}
	return &c, nil
}
