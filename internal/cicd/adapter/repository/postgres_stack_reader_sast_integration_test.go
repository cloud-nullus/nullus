//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SonarQube 를 고른 스택에만 주소를 준다. 고르지 않은 스택(또는 외부 SonarQube)에 주소를
// 주면 분석 단계가 렌더링되고, CI 잡은 없는 서버에 붙어 전부 실패한다.
func TestPostgresStackReader_SASTServerEndpoint(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)

	insert := func(sast string) string {
		id := "stack-" + uuid.NewString()
		_, err := pool.Exec(ctx,
			`INSERT INTO stacks (id, name, template_id, org_id, cluster_id, namespace, config)
			 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
			id, "SAST Stack", "gitlab-argocd-sonarqube-v1", orgID, clusterID, "sast-ns",
			`{"security":{"sast":`+sast+`}}`)
		require.NoError(t, err)
		return id
	}
	reader := NewPostgresStackReader(pool)

	cases := []struct {
		name string
		sast string
		want string
	}{
		{"SonarQube 를 골랐다", `{"name":"sonarqube","version":"26.9.0.129388","enabled":true}`, "http://sonarqube.sast-ns.svc.cluster.local:9000"},
		{"고르지 않았다", `{"name":"","version":"","enabled":false}`, ""},
		{"외부 SonarQube 다", `{"name":"sonarqube","version":"external","enabled":true}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary, err := reader.GetStackSummary(ctx, insert(tc.sast))
			require.NoError(t, err)
			require.NotNil(t, summary)
			assert.Equal(t, tc.want, summary.SASTServerEndpoint)
		})
	}
}
