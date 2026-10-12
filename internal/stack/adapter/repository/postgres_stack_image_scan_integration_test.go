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

func TestPostgresStackImageScanRepository_ReplaceAndList(t *testing.T) {
	t.Parallel()

	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	stacks := NewPostgresStackRepository(pool)
	stackID := "stack-" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, stacks.Create(ctx, &domain.Stack{
		ID: stackID, Name: "Image Scan Stack", TemplateID: "gitlab-allinone-v1", OrgID: orgID,
		ClusterID: clusterID, Namespace: "image-scan", State: domain.StateCompleted,
		Config: sampleStackConfig("GitLab CE"), CreatedAt: now, UpdatedAt: now,
	}))

	repo := NewPostgresStackImageScanRepository(pool)
	counts := shareddomain.SeverityCounts{Critical: 1, High: 2}
	fixable := shareddomain.SeverityCounts{High: 1}
	dbAt := now.Add(-time.Hour)
	first := []domain.StackImageScan{
		{Image: "docker.io/library/redis:7", ImageDigest: "sha256:r", Release: "argo-cd",
			Workloads: []string{"argo-cd-argocd-redis"}, Status: domain.ImageScanStatusScanned,
			Counts: &counts, FixableCounts: &fixable, ScannerVersion: "0.74.0", DBUpdatedAt: &dbAt, ScannedAt: now},
		{Image: "quay.io/argoproj/argocd:v2.13.3", ImageDigest: "sha256:a",
			Status: domain.ImageScanStatusFailed, Error: "manifest unknown", ScannedAt: now},
	}
	require.NoError(t, repo.ReplaceForStack(ctx, stackID, first))

	got, err := repo.ListByStack(ctx, stackID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	byDigest := map[string]domain.StackImageScan{}
	for _, s := range got {
		byDigest[s.ImageDigest] = s
	}
	redis := byDigest["sha256:r"]
	assert.Equal(t, stackID, redis.StackID)
	assert.Equal(t, &counts, redis.Counts)
	assert.Equal(t, &fixable, redis.FixableCounts)
	assert.Equal(t, []string{"argo-cd-argocd-redis"}, redis.Workloads)
	require.NotNil(t, redis.DBUpdatedAt)
	assert.WithinDuration(t, dbAt, *redis.DBUpdatedAt, time.Second)
	// 스캔하지 못한 이미지의 건수는 모른다 — 0 으로 읽히면 안 된다.
	assert.Nil(t, byDigest["sha256:a"].Counts)
	assert.Equal(t, "manifest unknown", byDigest["sha256:a"].Error)

	// 다시 스캔하면 더는 돌지 않는 이미지의 결과가 남지 않는다.
	require.NoError(t, repo.ReplaceForStack(ctx, stackID, first[:1]))
	got, err = repo.ListByStack(ctx, stackID)
	require.NoError(t, err)
	require.Len(t, got, 1)

	// 취약점 목록을 함께 저장하고 이미지별로 읽는다.
	vulns := []shareddomain.ImageVulnerability{
		{ID: "CVE-1", PkgName: "openssl", InstalledVersion: "3.0.1", FixedVersion: "3.0.2", Severity: "critical",
			Class: shareddomain.VulnerabilityClassOS, Target: "debian 12", PrimaryURL: "https://avd.aquasec.com/nvd/cve-1"},
		{ID: "GHSA-2", PkgName: "x/net", InstalledVersion: "0.1", Severity: "medium",
			Class: shareddomain.VulnerabilityClassLibrary, Target: "app"},
	}
	withList := first[0]
	withList.VulnerabilitiesRecorded = true
	withList.Vulnerabilities = vulns
	require.NoError(t, repo.ReplaceForStack(ctx, stackID, []domain.StackImageScan{withList, first[1]}))

	record, err := repo.ListVulnerabilities(ctx, stackID, "sha256:r")
	require.NoError(t, err)
	assert.True(t, record.Found)
	assert.True(t, record.Recorded)
	assert.ElementsMatch(t, vulns, record.Items)

	failedRecord, err := repo.ListVulnerabilities(ctx, stackID, "sha256:a")
	require.NoError(t, err)
	assert.True(t, failedRecord.Found)
	assert.False(t, failedRecord.Recorded)
	assert.Empty(t, failedRecord.Items)

	unknown, err := repo.ListVulnerabilities(ctx, stackID, "sha256:nope")
	require.NoError(t, err)
	assert.False(t, unknown.Found)

	// 다시 스캔하면 목록도 이번 스캔으로 바뀐다.
	require.NoError(t, repo.ReplaceForStack(ctx, stackID, first[:1]))
	record, err = repo.ListVulnerabilities(ctx, stackID, "sha256:r")
	require.NoError(t, err)
	assert.False(t, record.Recorded)
	assert.Empty(t, record.Items)

	completed, err := stacks.ListCompleted(ctx)
	require.NoError(t, err)
	ids := make([]string, 0, len(completed))
	for _, s := range completed {
		ids = append(ids, s.ID)
	}
	assert.Contains(t, ids, stackID)
}
