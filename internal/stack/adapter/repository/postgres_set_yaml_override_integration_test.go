//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 오버라이드 키 하나만 바꾼다 — 같은 행의 상태·단계, 다른 오버라이드 키, StackConfig 에 없는
// config.tools 까지 그대로 둔다. Update 로 덮어쓰면 오래된 읽기가 그 사이의 변경을 되돌리고 tools 를 지운다.
func TestPostgresStackRepository_SetYAMLOverride(t *testing.T) {
	t.Parallel()
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	repo := NewPostgresStackRepository(pool)

	stackID := "stk-" + uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, repo.Create(ctx, &domain.Stack{
		ID: stackID, Name: "override-" + stackID[4:12], TemplateID: "gitlab-argocd-v1", OrgID: orgID, ClusterID: clusterID,
		State: domain.StateCompleted, CurrentStep: "done",
		Config: domain.StackConfig{
			AccessDomain:  "nullus.local",
			YAMLOverrides: map[string]string{"installing_argocd": "old", "installing_gitlab": "keep"},
		},
		CreatedAt: now, UpdatedAt: now,
	}))
	_, err := pool.Exec(ctx, `UPDATE stacks SET config = config || '{"tools":[{"name":"gitlab"}]}'::jsonb WHERE id = $1`, stackID)
	require.NoError(t, err)

	require.NoError(t, repo.SetYAMLOverride(ctx, stackID, "installing_argocd", "new"))
	require.NoError(t, repo.SetYAMLOverride(ctx, stackID, "runner", "added"))

	var raw []byte
	var state, step string
	require.NoError(t, pool.QueryRow(ctx, `SELECT config, state, COALESCE(current_step, '') FROM stacks WHERE id = $1`, stackID).
		Scan(&raw, &state, &step))
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(raw, &cfg))
	assert.Equal(t, map[string]any{"installing_argocd": "new", "installing_gitlab": "keep", "runner": "added"}, cfg["yaml_overrides"])
	assert.Equal(t, "nullus.local", cfg["access_domain"])
	assert.NotNil(t, cfg["tools"], "StackConfig 에 없는 config.tools 도 남아야 한다")
	assert.Equal(t, string(domain.StateCompleted), state)
	assert.Equal(t, "done", step)

	// yaml_overrides 가 아직 없는 스택에도 쓴다.
	other := "stk-" + uuid.NewString()
	require.NoError(t, repo.Create(ctx, &domain.Stack{
		ID: other, Name: "override-" + other[4:12], TemplateID: "gitlab-argocd-v1", OrgID: orgID, ClusterID: clusterID,
		State: domain.StateCompleted, Config: domain.StackConfig{}, CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, repo.SetYAMLOverride(ctx, other, "argocd", "x"))
	got, err := repo.GetByID(ctx, other)
	require.NoError(t, err)
	assert.Equal(t, "x", got.Config.(domain.StackConfig).YAMLOverrides["argocd"])

	assert.Error(t, repo.SetYAMLOverride(ctx, "stk-missing", "argocd", "x"))
}
