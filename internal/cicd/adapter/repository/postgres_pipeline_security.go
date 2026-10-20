package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PostgresPipelineSecurityReader 는 port.PipelineSecurityReader 의 pgx 구현이다.
//
// 보안 대시보드(#65)가 조직의 앱 전체를 한 번에 본다. 파이프라인마다 네 결과를 따로 물으면 질의 수가
// 앱 수에 비례하므로, 결과 종류마다 한 번씩 "파이프라인별 최신 한 행"(DISTINCT ON)을 읽어 붙인다.
// 각 결과 테이블의 (pipeline_id, scanned_at DESC) 인덱스를 그대로 탄다. 대시보드용 테이블은 두지 않는다.
type PostgresPipelineSecurityReader struct {
	pool *pgxpool.Pool
}

// NewPostgresPipelineSecurityReader 는 읽기 저장소를 만든다.
func NewPostgresPipelineSecurityReader(pool *pgxpool.Pool) *PostgresPipelineSecurityReader {
	return &PostgresPipelineSecurityReader{pool: pool}
}

// ListPipelineSecurity 는 조직의 파이프라인을 최근 만든 것부터 최대 limit 개, 단계별 최신 결과와 함께 돌려준다.
func (r *PostgresPipelineSecurityReader) ListPipelineSecurity(ctx context.Context, orgID string, limit int) ([]*domain.PipelineSecuritySnapshot, error) {
	if limit <= 0 {
		return []*domain.PipelineSecuritySnapshot{}, nil
	}
	snapshots, err := r.listPipelines(ctx, strings.TrimSpace(orgID), limit)
	if err != nil || len(snapshots) == 0 {
		return snapshots, err
	}
	byID := make(map[string]*domain.PipelineSecuritySnapshot, len(snapshots))
	ids := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		byID[s.PipelineID] = s
		ids = append(ids, s.PipelineID)
	}

	if err := r.attachLatestRuns(ctx, ids, byID); err != nil {
		return nil, err
	}
	// 결과 종류마다 파이프라인별 최신 한 행이다. 열 목록 · 스캔 함수는 단건 조회와 같은 것을 쓴다 —
	// 한쪽만 고치면 대시보드와 실행 기록 화면이 같은 결과를 다르게 읽는다.
	if err := latestPerPipeline(ctx, r.pool, `SELECT DISTINCT ON (pipeline_id) `+imageScanColumns+`
		FROM image_scan_results WHERE pipeline_id = ANY($1)
		ORDER BY pipeline_id, scanned_at DESC, id DESC`, ids,
		func(row pgx.Row) error {
			s, err := scanImageScanResult(row)
			if err == nil {
				byID[s.PipelineID].ImageScan = s
			}
			return err
		}); err != nil {
		return nil, fmt.Errorf("latest image scans: %w", err)
	}
	if err := latestPerPipeline(ctx, r.pool, `SELECT DISTINCT ON (pipeline_id) `+sastResultColumns+`
		FROM sast_results WHERE pipeline_id = ANY($1)
		ORDER BY pipeline_id, analyzed_at DESC, id DESC`, ids,
		func(row pgx.Row) error {
			s, err := scanSASTResult(row)
			if err == nil {
				byID[s.PipelineID].SAST = s
			}
			return err
		}); err != nil {
		return nil, fmt.Errorf("latest sast results: %w", err)
	}
	if err := latestPerPipeline(ctx, r.pool, `SELECT DISTINCT ON (pipeline_id) `+dependencyScanColumns+`
		FROM dependency_scan_results WHERE pipeline_id = ANY($1)
		ORDER BY pipeline_id, scanned_at DESC, id DESC`, ids,
		func(row pgx.Row) error {
			s, err := scanDependencyScanResult(row)
			if err == nil {
				byID[s.PipelineID].DependencyScan = s
			}
			return err
		}); err != nil {
		return nil, fmt.Errorf("latest dependency scans: %w", err)
	}
	if err := latestPerPipeline(ctx, r.pool, `SELECT DISTINCT ON (pipeline_id) `+secretScanColumns+`
		FROM secret_scan_results WHERE pipeline_id = ANY($1)
		ORDER BY pipeline_id, scanned_at DESC, id DESC`, ids,
		func(row pgx.Row) error {
			s, err := scanSecretScanResult(row)
			if err == nil {
				byID[s.PipelineID].SecretScan = s
			}
			return err
		}); err != nil {
		return nil, fmt.Errorf("latest secret scans: %w", err)
	}
	if err := r.attachSecretPolicies(ctx, snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (r *PostgresPipelineSecurityReader) listPipelines(ctx context.Context, orgID string, limit int) ([]*domain.PipelineSecuritySnapshot, error) {
	const q = `
		SELECT id, name, COALESCE(stack_id, ''), cluster_id, COALESCE(stages, '[]'::jsonb)
		FROM pipelines
		WHERE org_id = $1
		ORDER BY created_at DESC, id
		LIMIT $2`
	rows, err := r.pool.Query(ctx, q, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("query pipelines for security: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.PipelineSecuritySnapshot, 0)
	for rows.Next() {
		var (
			s      domain.PipelineSecuritySnapshot
			stages []byte
		)
		if err := rows.Scan(&s.PipelineID, &s.Name, &s.StackID, &s.ClusterID, &stages); err != nil {
			return nil, fmt.Errorf("scan pipeline for security: %w", err)
		}
		if len(stages) > 0 {
			if err := json.Unmarshal(stages, &s.Stages); err != nil {
				return nil, fmt.Errorf("unmarshal pipeline stages: %w", err)
			}
		}
		if len(s.Stages) == 0 {
			s.Stages = nil
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return out, nil
}

// attachLatestRuns 는 파이프라인별 최근 실행(시작 시각 기준)을 붙인다.
func (r *PostgresPipelineSecurityReader) attachLatestRuns(ctx context.Context, ids []string, byID map[string]*domain.PipelineSecuritySnapshot) error {
	const q = `
		SELECT DISTINCT ON (pipeline_id) id, pipeline_id, version, status::text, started_at
		FROM pipeline_deployments
		WHERE pipeline_id = ANY($1)
		ORDER BY pipeline_id, started_at DESC, id DESC`
	return latestPerPipeline(ctx, r.pool, q, ids, func(row pgx.Row) error {
		var (
			run        domain.RunSummary
			pipelineID string
			status     string
		)
		if err := row.Scan(&run.DeploymentID, &pipelineID, &run.Version, &status, &run.StartedAt); err != nil {
			return fmt.Errorf("scan latest run: %w", err)
		}
		run.Status = domain.DeploymentStatus(status)
		run.StartedAt = run.StartedAt.UTC()
		byID[pipelineID].LatestRun = &run
		return nil
	})
}

// attachSecretPolicies 는 스택의 시크릿 탐지 정책을 붙인다. 정책 행이 없으면 기본값(차단)이다 — 스택에
// 묶이지 않은 파이프라인은 정책이 없으므로 빈 값으로 둔다.
func (r *PostgresPipelineSecurityReader) attachSecretPolicies(ctx context.Context, snapshots []*domain.PipelineSecuritySnapshot) error {
	seen := map[string]bool{}
	stackIDs := make([]string, 0)
	for _, s := range snapshots {
		if s.StackID != "" && !seen[s.StackID] {
			seen[s.StackID] = true
			stackIDs = append(stackIDs, s.StackID)
		}
	}
	if len(stackIDs) == 0 {
		return nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT stack_id, secret_detection FROM image_scan_policies WHERE stack_id = ANY($1)`, stackIDs)
	if err != nil {
		return fmt.Errorf("query secret detection policies: %w", err)
	}
	defer rows.Close()
	actions := map[string]domain.SecretDetectionAction{}
	for rows.Next() {
		var stackID, action string
		if err := rows.Scan(&stackID, &action); err != nil {
			return fmt.Errorf("scan secret detection policy: %w", err)
		}
		actions[stackID] = domain.SecretDetectionAction(action)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}
	for _, s := range snapshots {
		if s.StackID == "" {
			continue
		}
		s.SecretDetection = domain.ScanPolicy{SecretDetection: actions[s.StackID]}.SecretDetectionOrDefault()
	}
	return nil
}

// latestPerPipeline 은 파이프라인 ID 목록을 인자로 받는 질의의 행을 하나씩 넘긴다.
func latestPerPipeline(ctx context.Context, pool *pgxpool.Pool, q string, ids []string, each func(pgx.Row) error) error {
	rows, err := pool.Query(ctx, q, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
