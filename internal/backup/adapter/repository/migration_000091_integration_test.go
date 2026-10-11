//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
)

// 000091 — backup_runs.org_id 가 organizations 를 참조한다.
//
// FK 가 없어 어떤 조직 ID 로도 백업이 만들어졌고, 없는 조직으로 만든 백업은 어떤
// 화면에도 보이지 않았다(목록은 로그인 사용자의 조직으로만 거른다). 문서가 적어
// 둔 유령 조직 00000000-…-0001 이 실제로 그랬다. 이미 들어간 고아 행은 시드 기본
// 조직으로 옮겨 다시 보이게 하고, 이후로는 DB 가 거절한다.
func TestMigration000091_고아_백업을_기본_조직으로_옮기고_FK를_건다(t *testing.T) {
	pool, cleanup := startBackupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	repo := NewPostgresBackupRepository(pool)
	const ghost = "00000000-0000-0000-0000-000000000001"

	// FK 가 없던 시절: 유령 조직의 백업이 그냥 들어간다.
	require.NoError(t, applyMigrationRange(ctx, pool, "", "000091_"))
	orphan := domain.NewBackupRun(ghost, domain.ModePlatformOnly, domain.TriggerManual,
		domain.ModeComponents(domain.ModePlatformOnly))
	require.NoError(t, repo.CreateRun(ctx, orphan), "FK 이전에는 들어가야 한다")

	require.NoError(t, applyMigrationRange(ctx, pool, "000091_", ""))

	got, err := repo.GetRun(ctx, orphan.ID)
	require.NoError(t, err)
	assert.Equal(t, shareddomain.SeededDefaultOrgID, got.OrgID, "고아 행은 시드 조직으로 옮겨진다")

	again := domain.NewBackupRun(ghost, domain.ModePlatformOnly, domain.TriggerManual,
		domain.ModeComponents(domain.ModePlatformOnly))
	err = repo.CreateRun(ctx, again)
	require.Error(t, err, "이제 없는 조직으로는 만들 수 없다")
	assert.Contains(t, err.Error(), "backup_runs_org_id_fkey")
}
