package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// PostgresScanPolicyRepository 는 port.ScanPolicyRepository 의 pgx 구현이다.
type PostgresScanPolicyRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresScanPolicyRepository 는 저장소를 만든다.
func NewPostgresScanPolicyRepository(pool *pgxpool.Pool) *PostgresScanPolicyRepository {
	return &PostgresScanPolicyRepository{pool: pool}
}

// Get 은 저장된 정책이다. 저장한 적 없으면 found=false 다.
func (r *PostgresScanPolicyRepository) Get(ctx context.Context, stackID string) (domain.ScanPolicy, bool, error) {
	const q = `
		SELECT block_severity, ignore_unfixed, on_scanner_unreachable
		FROM image_scan_policies
		WHERE stack_id = $1`

	var severity, action string
	var ignoreUnfixed bool
	err := r.pool.QueryRow(ctx, q, strings.TrimSpace(stackID)).Scan(&severity, &ignoreUnfixed, &action)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ScanPolicy{}, false, nil
	}
	if err != nil {
		return domain.ScanPolicy{}, false, fmt.Errorf("스캔 정책 조회 실패 (%s): %w", stackID, err)
	}
	return domain.ScanPolicy{
		BlockSeverity:        domain.Severity(severity),
		IgnoreUnfixed:        ignoreUnfixed,
		OnScannerUnreachable: domain.UnreachableAction(action),
	}, true, nil
}

// Upsert 는 스택의 정책을 저장하거나 덮어쓴다.
func (r *PostgresScanPolicyRepository) Upsert(
	ctx context.Context,
	stackID string,
	policy domain.ScanPolicy,
	updatedBy string,
) error {
	const q = `
		INSERT INTO image_scan_policies (
			stack_id, block_severity, ignore_unfixed, on_scanner_unreachable, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (stack_id) DO UPDATE SET
			block_severity = EXCLUDED.block_severity,
			ignore_unfixed = EXCLUDED.ignore_unfixed,
			on_scanner_unreachable = EXCLUDED.on_scanner_unreachable,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()`

	if _, err := r.pool.Exec(ctx, q,
		strings.TrimSpace(stackID),
		string(policy.BlockSeverity),
		policy.IgnoreUnfixed,
		string(policy.UnreachableActionOrDefault()),
		strings.TrimSpace(updatedBy),
	); err != nil {
		return fmt.Errorf("스캔 정책 저장 실패 (%s): %w", stackID, err)
	}
	return nil
}
