//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// historyTables 는 복구가 보존하는 백업 이력 테이블이다(usecase 의 PreserveTables 와 같다).
var historyTables = []string{"backup_runs", "backup_artifacts", "restore_runs"}

// crossFKs 는 이력 테이블에서 이력 밖 테이블로 나가는 FK 이름을 돌려준다.
func crossFKs(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_class src ON src.oid = c.conrelid
		JOIN pg_class dst ON dst.oid = c.confrelid
		WHERE c.contype = 'f'
		  AND src.relname = ANY($1)
		  AND NOT (dst.relname = ANY($1))
		ORDER BY c.conname`, historyTables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// 000092 — 백업 이력 테이블은 플랫폼 테이블을 FK 로 참조하지 않는다.
//
// 복구는 이력을 보존한 채 나머지를 pg_restore --clean 으로 되돌린다. 이력 테이블이 플랫폼
// 테이블을 참조하면 --clean 이 그 PK 를 지우는 순간 "cannot drop constraint organizations_pkey …
// backup_runs_org_id_fkey depends on" 으로 복구가 죽는다 — 2026-10-12 실환경 리허설에서 000091 의
// FK 가 그렇게 full 복구를 막았다. 이 테스트는 그 원칙을 스키마 전체에 대해 지킨다: 앞으로 어떤
// 마이그레이션이 이력 테이블에 바깥 FK 를 걸면 여기서 드러난다.
func TestMigration000092_이력_테이블은_플랫폼_테이블을_FK_로_참조하지_않는다(t *testing.T) {
	pool, cleanup := startBackupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	// 000091 까지는 FK 가 있다 — 이 질의가 그것을 집어낸다는 것을 먼저 확인한다(빈 결과가
	// "질의가 틀렸다" 가 아니라 "FK 가 없다" 임을 보장한다).
	require.NoError(t, applyMigrationRange(ctx, pool, "", "000092_"))
	before, err := crossFKs(ctx, pool)
	require.NoError(t, err)
	assert.Equal(t, []string{"backup_runs_org_id_fkey"}, before, "000091 의 FK 를 질의가 집어내야 한다")

	// 000092 이후 — 그리고 그 뒤의 모든 마이그레이션 뒤에도 — 바깥 FK 는 없다.
	require.NoError(t, applyMigrationRange(ctx, pool, "000092_", ""))
	after, err := crossFKs(ctx, pool)
	require.NoError(t, err)
	assert.Empty(t, after, "백업 이력 테이블은 플랫폼 테이블을 FK 로 참조하지 않는다 — 복구(--clean)를 막는다")
}
