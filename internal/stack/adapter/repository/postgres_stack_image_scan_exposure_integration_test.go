//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shareddomain "github.com/cloud-nullus/draft/internal/shared/domain"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 외부 노출을 함께 저장하고, 새 CVE 고지가 지난 스캔의 "수정판 있는 CRITICAL" 을 읽는다.
func TestPostgresStackImageScanRepository_ExposureAndKnownCriticals(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	stacks := NewPostgresStackRepository(pool)
	stackID := "stack-" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, stacks.Create(ctx, &domain.Stack{
		ID: stackID, Name: "CVE Stack", TemplateID: "gitlab-allinone-v1", OrgID: orgID,
		ClusterID: clusterID, Namespace: "cve", State: domain.StateCompleted,
		Config: sampleStackConfig("GitLab CE"), CreatedAt: now, UpdatedAt: now,
	}))
	repo := NewPostgresStackImageScanRepository(pool)

	// 처음 스캔하는 스택은 알던 것이 없다.
	ids, recorded, err := repo.KnownFixableCriticalIDs(ctx, stackID)
	require.NoError(t, err)
	assert.False(t, recorded)
	assert.Empty(t, ids)

	counts := shareddomain.SeverityCounts{Critical: 2}
	scans := []domain.StackImageScan{
		{Image: "gitlab-webservice:17.7", ImageDigest: "sha256:w", Status: domain.ImageScanStatusScanned,
			Counts: &counts, ScannedAt: now, Exposed: true, ExposedHosts: []string{"gitlab.nullus.local"},
			VulnerabilitiesRecorded: true, Vulnerabilities: []shareddomain.ImageVulnerability{
				{ID: "CVE-FIX", PkgName: "openssl", InstalledVersion: "3.0.1", FixedVersion: "3.0.9", Severity: "critical",
					Class: shareddomain.VulnerabilityClassOS},
				{ID: "CVE-NOFIX", PkgName: "zlib", InstalledVersion: "1.2", Severity: "CRITICAL", Class: shareddomain.VulnerabilityClassOS},
				{ID: "CVE-HIGH", PkgName: "curl", InstalledVersion: "8.0", FixedVersion: "8.1", Severity: "high", Class: shareddomain.VulnerabilityClassOS},
			}},
		{Image: "redis:7", ImageDigest: "sha256:r", Status: domain.ImageScanStatusScanned, ScannedAt: now},
	}
	require.NoError(t, repo.ReplaceForStack(ctx, stackID, scans))

	got, err := repo.ListByStack(ctx, stackID)
	require.NoError(t, err)
	byDigest := map[string]domain.StackImageScan{}
	for _, s := range got {
		byDigest[s.ImageDigest] = s
	}
	assert.True(t, byDigest["sha256:w"].Exposed)
	assert.Equal(t, []string{"gitlab.nullus.local"}, byDigest["sha256:w"].ExposedHosts)
	assert.False(t, byDigest["sha256:r"].Exposed)
	assert.Empty(t, byDigest["sha256:r"].ExposedHosts)

	ids, recorded, err = repo.KnownFixableCriticalIDs(ctx, stackID)
	require.NoError(t, err)
	assert.True(t, recorded)
	assert.Equal(t, map[string]bool{"CVE-FIX": true}, ids, "대소문자와 상관없이 CRITICAL · 수정판이 있는 것만")

	// 알린 CVE 는 그 이미지의 스캔이 실패해 행이 사라져도 아는 것으로 남는다. 처음 알린 시각은 그대로다.
	first := now.Add(-time.Hour)
	require.NoError(t, repo.RecordCVENotices(ctx, stackID, []string{"CVE-FIX"}, first))
	require.NoError(t, repo.RecordCVENotices(ctx, stackID, []string{"CVE-FIX", "CVE-LATER"}, now))
	require.NoError(t, repo.RecordCVENotices(ctx, stackID, nil, now))
	require.NoError(t, repo.ReplaceForStack(ctx, stackID, []domain.StackImageScan{
		{Image: "gitlab-webservice:17.7", ImageDigest: "sha256:w", Status: domain.ImageScanStatusFailed,
			Error: "unexpected EOF", ScannedAt: now},
	}))
	ids, recorded, err = repo.KnownFixableCriticalIDs(ctx, stackID)
	require.NoError(t, err)
	assert.True(t, recorded, "목록이 하나도 없어도 알린 적이 있으면 첫 스캔이 아니다")
	assert.Equal(t, map[string]bool{"CVE-FIX": true, "CVE-LATER": true}, ids)
	var notifiedAt time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT first_notified_at FROM stack_cve_notices WHERE stack_id = $1 AND vulnerability_id = 'CVE-FIX'`,
		stackID).Scan(&notifiedAt))
	assert.True(t, notifiedAt.Equal(first), "처음 알린 시각은 대응 기한의 기준이라 바꾸지 않는다")
}
