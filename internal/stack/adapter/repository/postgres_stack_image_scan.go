package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
			vulnerabilities_recorded, exposed, exposed_hosts, namespace, shared
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'trivy', $10, $11, $12, $13, $14, $15, $16, $17)
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
		hosts := s.ExposedHosts
		if hosts == nil {
			hosts = []string{}
		}
		hostsJSON, err := json.Marshal(hosts)
		if err != nil {
			return fmt.Errorf("marshal exposed hosts: %w", err)
		}
		if _, err := tx.Exec(ctx, q,
			stackID, s.ImageDigest, s.Image, s.Release, workloadsJSON, string(s.Status), s.Error,
			counts, fixable, s.ScannerVersion, s.DBUpdatedAt, s.ScannedAt, s.VulnerabilitiesRecorded,
			s.Exposed, hostsJSON, s.Namespace, s.Shared,
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
			counts, fixable_counts, scanner_version, db_updated_at, scanned_at, vulnerabilities_recorded,
			exposed, exposed_hosts, namespace, shared
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
			hosts                 []byte
		)
		if err := rows.Scan(&s.StackID, &s.ImageDigest, &s.Image, &s.Release, &workloads, &status, &s.Error,
			&counts, &fixableCounts, &s.ScannerVersion, &s.DBUpdatedAt, &s.ScannedAt, &s.VulnerabilitiesRecorded,
			&s.Exposed, &hosts, &s.Namespace, &s.Shared); err != nil {
			return nil, fmt.Errorf("scan stack image scan: %w", err)
		}
		s.Status = domain.ImageScanStatus(status)
		if err := json.Unmarshal(workloads, &s.Workloads); err != nil {
			return nil, fmt.Errorf("unmarshal workloads: %w", err)
		}
		if err := json.Unmarshal(hosts, &s.ExposedHosts); err != nil {
			return nil, fmt.Errorf("unmarshal exposed hosts: %w", err)
		}
		if len(s.ExposedHosts) == 0 {
			s.ExposedHosts = nil
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

// NotifiedCVEIDs 는 스택에 이미 알린 CVE 다.
func (r *PostgresStackImageScanRepository) NotifiedCVEIDs(ctx context.Context, stackID string) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT vulnerability_id FROM stack_cve_notices WHERE stack_id = $1`, stackID)
	if err != nil {
		return nil, fmt.Errorf("query stack cve notices: %w", err)
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan vulnerability id: %w", err)
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// ClaimCVENotices 는 ids 중 아직 남지 않은 것을 남기고 이번에 남긴 것만 돌려준다. 같은 행을 두
// 레플리카가 동시에 넣으면 기본 키가 하나만 받는다.
func (r *PostgresStackImageScanRepository) ClaimCVENotices(ctx context.Context, stackID string, ids []string, at time.Time) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		INSERT INTO stack_cve_notices (stack_id, vulnerability_id, first_notified_at)
		SELECT $1, id, $3 FROM unnest($2::text[]) AS id
		ON CONFLICT (stack_id, vulnerability_id) DO NOTHING
		RETURNING vulnerability_id`, stackID, ids, at)
	if err != nil {
		return nil, fmt.Errorf("claim stack cve notices: %w", err)
	}
	defer rows.Close()
	claimed := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan claimed vulnerability id: %w", err)
		}
		claimed = append(claimed, id)
	}
	return claimed, rows.Err()
}

// ReleaseCVENotices 는 보내지 못한 선점을 되돌린다.
func (r *PostgresStackImageScanRepository) ReleaseCVENotices(ctx context.Context, stackID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM stack_cve_notices WHERE stack_id = $1 AND vulnerability_id = ANY($2::text[])`, stackID, ids); err != nil {
		return fmt.Errorf("release stack cve notices: %w", err)
	}
	return nil
}

// VulnerabilityIDsByRelease 는 스택의 한 릴리스가 설치한 이미지에서 지금 발견된 취약점 ID 다. 업그레이드
// 대상이 아닌 공용 구성요소는 같은 이름의 릴리스라도 뺀다.
func (r *PostgresStackImageScanRepository) VulnerabilityIDsByRelease(ctx context.Context, stackID, release string) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT v.vulnerability_id
		FROM stack_image_vulnerabilities v
		JOIN stack_image_scans s ON s.stack_id = v.stack_id AND s.image_digest = v.image_digest
		WHERE s.stack_id = $1 AND s.release_name = $2 AND NOT s.shared`, stackID, release)
	if err != nil {
		return nil, fmt.Errorf("query release vulnerabilities: %w", err)
	}
	defer rows.Close()
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan vulnerability id: %w", err)
		}
		ids[id] = true
	}
	return ids, rows.Err()
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
