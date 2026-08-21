package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/stack/port"
)

// PostgresDeployLogStore 는 설치 로그를 stack_deploy_logs 에 남긴다.
//
// 로그가 프로세스 메모리에만 있으면 파드가 재시작되는 순간 사라진다. 설치는
// 20~30분짜리라 그 사이 재시작이 겹칠 확률이 낮지 않고, 그러면 무엇이 왜
// 멈췄는지 사후에 알 방법이 없다.
type PostgresDeployLogStore struct {
	pool *pgxpool.Pool
}

func NewPostgresDeployLogStore(pool *pgxpool.Pool) *PostgresDeployLogStore {
	return &PostgresDeployLogStore{pool: pool}
}

// Append 는 항목 하나를 남긴다.
func (r *PostgresDeployLogStore) Append(ctx context.Context, deploymentID string, entry port.LogEntry) error {
	const q = `
		INSERT INTO stack_deploy_logs (deployment_id, logged_at, level, step, phase, message)
		VALUES ($1, $2, $3, $4, $5, $6)`

	if _, err := r.pool.Exec(ctx, q,
		deploymentID, entry.Timestamp, entry.Level, entry.Step, entry.Phase, entry.Message,
	); err != nil {
		return fmt.Errorf("append deploy log for %s: %w", deploymentID, err)
	}
	return nil
}

// List 는 최근 limit 줄을 기록 순서로 돌려준다.
//
// 상한을 넘으면 오래된 쪽을 버린다 — 화면이 필요로 하는 것은 최근이고, 진행률은
// 마지막 항목에서 복원되기 때문이다. 그래서 안쪽 질의는 seq 역순으로 자르고
// 바깥에서 다시 오름차순으로 세운다.
func (r *PostgresDeployLogStore) List(ctx context.Context, deploymentID string, limit int) ([]port.LogEntry, error) {
	if limit <= 0 {
		limit = 1000
	}

	const q = `
		SELECT logged_at, level, step, phase, message
		FROM (
			SELECT seq, logged_at, level, step, phase, message
			FROM stack_deploy_logs
			WHERE deployment_id = $1
			ORDER BY seq DESC
			LIMIT $2
		) recent
		ORDER BY seq ASC`

	rows, err := r.pool.Query(ctx, q, deploymentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list deploy logs for %s: %w", deploymentID, err)
	}
	defer rows.Close()

	entries := make([]port.LogEntry, 0, 64)
	for rows.Next() {
		var entry port.LogEntry
		if err := rows.Scan(&entry.Timestamp, &entry.Level, &entry.Step, &entry.Phase, &entry.Message); err != nil {
			return nil, fmt.Errorf("scan deploy log for %s: %w", deploymentID, err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read deploy logs for %s: %w", deploymentID, err)
	}
	return entries, nil
}

// Delete 는 이 배포의 로그를 지운다.
func (r *PostgresDeployLogStore) Delete(ctx context.Context, deploymentID string) error {
	const q = `DELETE FROM stack_deploy_logs WHERE deployment_id = $1`
	if _, err := r.pool.Exec(ctx, q, deploymentID); err != nil {
		return fmt.Errorf("delete deploy logs for %s: %w", deploymentID, err)
	}
	return nil
}
