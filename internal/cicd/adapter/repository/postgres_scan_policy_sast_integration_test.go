//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// Quality Gate 실패 시 동작을 저장하고 되읽는다. 이 열이 생기기 전에 저장된 행은 차단으로
// 읽힌다 — 빈 값을 경고로 읽으면 조용히 게이트가 풀린다.
func TestPostgresScanPolicyRepository_SASTGateAction(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)

	newStack := func() string {
		id := "stack-" + uuid.NewString()
		_, err := pool.Exec(ctx,
			`INSERT INTO stacks (id, name, template_id, org_id, cluster_id) VALUES ($1, $2, $3, $4, $5)`,
			id, "Policy Stack", "gitlab-argocd-sonarqube-v1", orgID, clusterID)
		require.NoError(t, err)
		return id
	}
	repo := NewPostgresScanPolicyRepository(pool)

	warnStack := newStack()
	policy := domain.DefaultScanPolicy()
	policy.SASTOnGateFailure = domain.SASTGateWarn
	require.NoError(t, repo.Upsert(ctx, warnStack, policy, "alice"))
	got, found, err := repo.Get(ctx, warnStack)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, domain.SASTGateWarn, got.SASTOnGateFailure)

	// 열을 모르는 옛 쓰기(000088 이전에 저장된 행)와 같은 모양.
	legacyStack := newStack()
	_, err = pool.Exec(ctx,
		`INSERT INTO image_scan_policies (stack_id, block_severity, ignore_unfixed, on_scanner_unreachable)
		 VALUES ($1, 'HIGH', true, 'block')`, legacyStack)
	require.NoError(t, err)
	got, found, err = repo.Get(ctx, legacyStack)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, domain.SASTGateBlock, got.SASTOnGateFailure)

	// 허용하지 않는 값은 DB 도 받지 않는다.
	_, err = pool.Exec(ctx, `UPDATE image_scan_policies SET sast_on_gate_failure = 'maybe' WHERE stack_id = $1`, legacyStack)
	assert.Error(t, err)
}
