//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

// 대시보드는 조직의 앱마다 단계별 최신 결과를 한 번에 읽는다. 다른 조직의 파이프라인은 섞이지 않고,
// 결과가 없는 칸은 nil 이다(0 이나 빈 결과로 채우지 않는다).
func TestPostgresPipelineSecurityReader_LatestPerStageScopedToOrg(t *testing.T) {
	t.Parallel()
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	otherOrg, otherCluster := createTestOrgAndCluster(t, ctx, pool)
	newStack := func(org, cluster string) string {
		id := "stack-" + uuid.NewString()
		_, err := pool.Exec(ctx,
			`INSERT INTO stacks (id, name, template_id, org_id, cluster_id) VALUES ($1, $2, $3, $4, $5)`,
			id, "Security Stack", "gitlab-argocd-sonarqube-v1", org, cluster)
		require.NoError(t, err)
		return id
	}
	stackOff := newStack(orgID, clusterID)
	stackDefault := newStack(orgID, clusterID)
	policy := domain.DefaultScanPolicy()
	policy.SecretDetection = domain.SecretDetectionOff
	require.NoError(t, NewPostgresScanPolicyRepository(pool).Upsert(ctx, stackOff, policy, "alice"))

	pipelines := NewPostgresPipelineRepository(pool)
	stages := []string{"SecretScan", "DependencyScan", "Build", "SAST", "ImageScan", "Deploy"}
	orders := createScanTestPipeline(t, ctx, pipelines, orgID, clusterID, stackOff, "orders", stages)
	time.Sleep(5 * time.Millisecond)
	_ = createScanTestPipeline(t, ctx, pipelines, orgID, clusterID, stackDefault, "billing", nil)
	time.Sleep(5 * time.Millisecond)
	_ = createScanTestPipeline(t, ctx, pipelines, orgID, clusterID, "", "direct", []string{"Build", "Deploy"})
	foreign := createScanTestPipeline(t, ctx, pipelines, otherOrg, otherCluster, "", "foreign", stages)

	at := time.Now().UTC().Truncate(time.Microsecond)
	deployments := NewPostgresDeploymentRepository(pool)
	for _, d := range []*domain.Deployment{
		{ID: "dep_ci_" + orders + "_41", PipelineID: orders, Version: "#41", Status: domain.DeploymentStatusSuccess, StartedAt: at.Add(-time.Hour), DeployedBy: "ci"},
		{ID: "dep_ci_" + orders + "_42", PipelineID: orders, Version: "#42", Status: domain.DeploymentStatusFailed, StartedAt: at, DeployedBy: "ci"},
		{ID: "dep_ci_" + foreign + "_1", PipelineID: foreign, Version: "#1", Status: domain.DeploymentStatusSuccess, StartedAt: at, DeployedBy: "ci"},
	} {
		require.NoError(t, deployments.Create(ctx, d))
	}

	images := NewPostgresImageScanResultRepository(pool)
	require.NoError(t, images.Upsert(ctx, &domain.ImageScanResult{ID: "scan_old", PipelineID: orders, DeploymentID: "dep_ci_" + orders + "_40",
		ScanSource: "central", Scanner: "trivy", GateResult: domain.GateResultPass, ScannedAt: at.Add(-2 * time.Hour)}))
	require.NoError(t, images.Upsert(ctx, &domain.ImageScanResult{ID: "scan_new", PipelineID: orders, DeploymentID: "dep_ci_" + orders + "_41",
		ScanSource: "central", Scanner: "trivy", GateResult: domain.GateResultBlock, Counts: &domain.SeverityCounts{Critical: 2, High: 11},
		ScannedAt: at.Add(-time.Hour)}))
	require.NoError(t, images.Upsert(ctx, &domain.ImageScanResult{ID: "scan_foreign", PipelineID: foreign, DeploymentID: "dep_ci_" + foreign + "_1",
		ScanSource: "central", Scanner: "trivy", GateResult: domain.GateResultBlock, ScannedAt: at}))

	vulns := 4
	require.NoError(t, NewPostgresSASTResultRepository(pool).Upsert(ctx, &domain.SASTResult{ID: "sast_1", PipelineID: orders,
		DeploymentID: "dep_ci_" + orders + "_41", GateResult: domain.GateResultWarn,
		Metrics: &domain.SASTMetrics{Vulnerabilities: &vulns}, AnalyzedAt: at.Add(-time.Hour)}))

	targets := 2
	deps := NewPostgresDependencyScanResultRepository(pool)
	require.NoError(t, deps.Upsert(ctx, &domain.DependencyScanResult{ID: "depscan_41", PipelineID: orders, DeploymentID: "dep_ci_" + orders + "_41",
		Scanner: "trivy", GateResult: domain.GateResultPass, Counts: &domain.SeverityCounts{}, Targets: &targets, ScannedAt: at.Add(-time.Hour)}))
	require.NoError(t, deps.Upsert(ctx, &domain.DependencyScanResult{ID: "depscan_42", PipelineID: orders, DeploymentID: "dep_ci_" + orders + "_42",
		Scanner: "trivy", GateResult: domain.GateResultBlock, Counts: &domain.SeverityCounts{Critical: 3, High: 5}, Targets: &targets, ScannedAt: at}))

	two := 2
	require.NoError(t, NewPostgresSecretScanResultRepository(pool).Upsert(ctx, &domain.SecretScanResult{ID: "secretscan_42", PipelineID: orders,
		DeploymentID: "dep_ci_" + orders + "_42", Scanner: "gitleaks", FindingsCount: &two,
		Rules: []domain.SecretRuleCount{{RuleID: "github-pat", Count: 2}}, GateResult: domain.GateResultBlock, ScannedAt: at}))

	reader := NewPostgresPipelineSecurityReader(pool)
	got, err := reader.ListPipelineSecurity(ctx, orgID, 1000)
	require.NoError(t, err)
	require.Len(t, got, 3, "다른 조직의 파이프라인은 섞이지 않는다")
	byName := map[string]*domain.PipelineSecuritySnapshot{}
	for _, s := range got {
		byName[s.Name] = s
	}
	assert.Equal(t, []string{"direct", "billing", "orders"}, []string{got[0].Name, got[1].Name, got[2].Name}, "최근 만든 것부터")

	o := byName["orders"]
	require.NotNil(t, o)
	assert.Equal(t, stackOff, o.StackID)
	assert.Equal(t, clusterID, o.ClusterID)
	assert.Equal(t, stages, o.Stages)
	require.NotNil(t, o.LatestRun)
	assert.Equal(t, "dep_ci_"+orders+"_42", o.LatestRun.DeploymentID)
	assert.Equal(t, "#42", o.LatestRun.Version)
	assert.Equal(t, domain.DeploymentStatusFailed, o.LatestRun.Status)
	require.NotNil(t, o.ImageScan)
	assert.Equal(t, "scan_new", o.ImageScan.ID, "단계별 최신")
	require.NotNil(t, o.SAST)
	assert.Equal(t, domain.GateResultWarn, o.SAST.GateResult)
	require.NotNil(t, o.DependencyScan)
	assert.Equal(t, "depscan_42", o.DependencyScan.ID)
	require.NotNil(t, o.SecretScan)
	assert.Equal(t, 2, *o.SecretScan.FindingsCount)
	assert.Equal(t, domain.SecretDetectionOff, o.SecretDetection)

	b := byName["billing"]
	require.NotNil(t, b)
	assert.Nil(t, b.Stages, "단계 기록 전의 파이프라인은 모른다")
	assert.Nil(t, b.LatestRun)
	assert.Nil(t, b.ImageScan)
	assert.Nil(t, b.SAST)
	assert.Nil(t, b.DependencyScan)
	assert.Nil(t, b.SecretScan)
	assert.Equal(t, domain.SecretDetectionBlock, b.SecretDetection, "정책 행이 없는 스택은 기본값(차단)이다")

	d := byName["direct"]
	require.NotNil(t, d)
	assert.Empty(t, d.StackID)
	assert.Empty(t, d.SecretDetection, "스택이 없으면 정책도 없다")

	// 상한 — 최근 만든 것부터 자른다.
	limited, err := reader.ListPipelineSecurity(ctx, orgID, 2)
	require.NoError(t, err)
	require.Len(t, limited, 2)
	assert.Equal(t, "direct", limited[0].Name)

	empty, err := reader.ListPipelineSecurity(ctx, uuid.NewString(), 1000)
	require.NoError(t, err)
	assert.Empty(t, empty)
}
