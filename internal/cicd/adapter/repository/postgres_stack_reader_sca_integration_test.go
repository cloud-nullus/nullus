//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 의존성 스캔(SCA)을 고른 스택에만 주소를 준다 — 이미지 스캐너를 고르지 않아도 서버는 선다(nullus-plan#79).
// 고르지 않은 스택(또는 외부 선택)에 주소를 주면 단계가 렌더링되고 CI 잡은 없는 서버에 붙어 실패한다.
func TestPostgresStackReader_DependencyScanEndpoint(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)

	insert := func(security string) string {
		id := "stack-" + uuid.NewString()
		_, err := pool.Exec(ctx,
			`INSERT INTO stacks (id, name, template_id, org_id, cluster_id, namespace, config)
			 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
			id, "SCA Stack", "gitlab-argocd-v1", orgID, clusterID, "sca-ns",
			`{"security":`+security+`}`)
		require.NoError(t, err)
		return id
	}
	reader := NewPostgresStackReader(pool)
	trivy := "http://trivy.sca-ns.svc.cluster.local:4954"

	cases := []struct {
		name      string
		security  string
		wantSCA   string
		wantImage string
	}{
		{"의존성 스캔만", `{"sca":{"name":"trivy","version":"0.69.3","enabled":true}}`, trivy, ""},
		{"둘 다 — 같은 서버", `{"sca":{"name":"trivy","enabled":true},"image_scanner":{"name":"trivy","enabled":true}}`, trivy, trivy},
		{"고르지 않음", `{"sca":{"name":"","version":"","enabled":false}}`, "", ""},
		{"외부", `{"sca":{"name":"trivy","version":"external","enabled":true}}`, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := reader.GetStackSummary(ctx, insert(tc.security))
			require.NoError(t, err)
			require.NotNil(t, s)
			assert.Equal(t, tc.wantSCA, s.DependencyScanEndpoint)
			assert.Equal(t, tc.wantImage, s.ImageScannerEndpoint)
		})
	}
}
