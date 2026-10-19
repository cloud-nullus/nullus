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

// 외부 노출을 함께 저장하고, 새 CVE 고지가 알린 CVE 를 선점 · 기록한다.
func TestPostgresStackImageScanRepository_ExposureAndCVENotices(t *testing.T) {
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

	// 처음 고지하는 스택은 알린 것이 없다.
	ids, err := repo.NotifiedCVEIDs(ctx, stackID)
	require.NoError(t, err)
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
		{Image: "redis:7", ImageDigest: "sha256:r", Status: domain.ImageScanStatusScanned, ScannedAt: now,
			Namespace: "cve"},
		{Image: "cert-manager-controller:v1.16.3", ImageDigest: "sha256:cm", Status: domain.ImageScanStatusScanned,
			ScannedAt: now, Release: "cert-manager", Namespace: "cert-manager", Shared: true},
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
	// 공용 구성요소는 어디 있는지와 함께 남는다.
	assert.Equal(t, "cve", byDigest["sha256:r"].Namespace)
	assert.False(t, byDigest["sha256:r"].Shared)
	assert.Equal(t, "cert-manager", byDigest["sha256:cm"].Namespace)
	assert.True(t, byDigest["sha256:cm"].Shared)

	// 알릴 것을 선점한다. 이미 남은 것은 돌려주지 않고(다른 레플리카가 보냈다) 시각도 바꾸지 않는다.
	first := now.Add(-time.Hour)
	claimed, err := repo.ClaimCVENotices(ctx, stackID, []string{"CVE-FIX"}, first)
	require.NoError(t, err)
	assert.Equal(t, []string{"CVE-FIX"}, claimed)
	claimed, err = repo.ClaimCVENotices(ctx, stackID, []string{"CVE-FIX", "CVE-LATER"}, now)
	require.NoError(t, err)
	assert.Equal(t, []string{"CVE-LATER"}, claimed)
	claimed, err = repo.ClaimCVENotices(ctx, stackID, nil, now)
	require.NoError(t, err)
	assert.Empty(t, claimed)

	// 그 이미지의 스캔이 실패해 행이 사라져도 알린 것은 남는다.
	require.NoError(t, repo.ReplaceForStack(ctx, stackID, []domain.StackImageScan{
		{Image: "gitlab-webservice:17.7", ImageDigest: "sha256:w", Status: domain.ImageScanStatusFailed,
			Error: "unexpected EOF", ScannedAt: now},
	}))
	ids, err = repo.NotifiedCVEIDs(ctx, stackID)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"CVE-FIX": true, "CVE-LATER": true}, ids)
	var notifiedAt time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT first_notified_at FROM stack_cve_notices WHERE stack_id = $1 AND vulnerability_id = 'CVE-FIX'`,
		stackID).Scan(&notifiedAt))
	assert.True(t, notifiedAt.Equal(first), "처음 알린 시각은 대응 기한의 기준이라 바꾸지 않는다")

	// 보내지 못한 선점은 되돌린다.
	require.NoError(t, repo.ReleaseCVENotices(ctx, stackID, []string{"CVE-LATER"}))
	ids, err = repo.NotifiedCVEIDs(ctx, stackID)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"CVE-FIX": true}, ids)
}

// 업그레이드 후보가 "이 업그레이드가 지금 있는 CVE 를 고치는가" 를 보이려고 릴리스별 CVE 를 읽는다.
// 공용 구성요소는 스택 업그레이드 대상이 아니므로 같은 이름의 릴리스라도 빼고, 심각도는 가리지 않는다.
func TestPostgresStackImageScanRepository_VulnerabilityIDsByRelease(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	stackID := "stack-" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, NewPostgresStackRepository(pool).Create(ctx, &domain.Stack{
		ID: stackID, Name: "Release CVE Stack", TemplateID: "gitlab-allinone-v1", OrgID: orgID,
		ClusterID: clusterID, Namespace: "rel", State: domain.StateCompleted,
		Config: sampleStackConfig("GitLab CE"), CreatedAt: now, UpdatedAt: now,
	}))
	repo := NewPostgresStackImageScanRepository(pool)

	vuln := func(id, severity string) shareddomain.ImageVulnerability {
		return shareddomain.ImageVulnerability{ID: id, PkgName: "pkg", InstalledVersion: "1", Severity: severity,
			Class: shareddomain.VulnerabilityClassOS}
	}
	require.NoError(t, repo.ReplaceForStack(ctx, stackID, []domain.StackImageScan{
		{Image: "argocd:v2.13.3", ImageDigest: "sha256:a1", Release: "argo-cd", Status: domain.ImageScanStatusScanned,
			ScannedAt: now, VulnerabilitiesRecorded: true,
			Vulnerabilities: []shareddomain.ImageVulnerability{vuln("CVE-A", "critical"), vuln("CVE-B", "low")}},
		{Image: "redis:7", ImageDigest: "sha256:a2", Release: "argo-cd", Status: domain.ImageScanStatusScanned,
			ScannedAt: now, VulnerabilitiesRecorded: true,
			Vulnerabilities: []shareddomain.ImageVulnerability{vuln("CVE-A", "critical"), vuln("CVE-C", "high")}},
		{Image: "gitlab:17", ImageDigest: "sha256:g", Release: "gitlab", Status: domain.ImageScanStatusScanned,
			ScannedAt: now, VulnerabilitiesRecorded: true,
			Vulnerabilities: []shareddomain.ImageVulnerability{vuln("CVE-G", "critical")}},
		{Image: "argocd-shared:v1", ImageDigest: "sha256:s", Release: "argo-cd", Namespace: "argocd", Shared: true,
			Status: domain.ImageScanStatusScanned, ScannedAt: now, VulnerabilitiesRecorded: true,
			Vulnerabilities: []shareddomain.ImageVulnerability{vuln("CVE-SHARED", "critical")}},
	}))

	ids, err := repo.VulnerabilityIDsByRelease(ctx, stackID, "argo-cd")
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"CVE-A": true, "CVE-B": true, "CVE-C": true}, ids)

	ids, err = repo.VulnerabilityIDsByRelease(ctx, stackID, "keycloak")
	require.NoError(t, err)
	assert.Empty(t, ids, "스캔에 없는 릴리스")
}
