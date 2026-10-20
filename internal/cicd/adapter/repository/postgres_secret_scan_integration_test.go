//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func TestPostgresSecretScanResultRepository_RoundTrip(t *testing.T) {
	t.Parallel()
	pool, cleanup := setupPostgres(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	orgID, clusterID := createTestOrgAndCluster(t, ctx, pool)
	pipelineID := createScanTestPipeline(t, ctx, NewPostgresPipelineRepository(pool), orgID, clusterID, "", "secret-scan", nil)
	repo := NewPostgresSecretScanResultRepository(pool)
	at := time.Now().UTC().Truncate(time.Microsecond)
	two := 2

	found := &domain.SecretScanResult{
		ID: domain.SecretScanResultID("dep_1"), PipelineID: pipelineID, DeploymentID: "dep_1",
		Scanner: "gitleaks", FindingsCount: &two,
		Rules: []domain.SecretRuleCount{{RuleID: "github-pat", Count: 1}, {RuleID: "aws-access-token", Count: 1}},
		Findings: []domain.SecretFinding{
			{RuleID: "github-pat", File: "config/app.env", StartLine: 3, Commit: "9f1c2ab", Fingerprint: "9f1c2ab:config/app.env:github-pat:3"},
			{RuleID: "aws-access-token", File: "deploy/aws.sh", StartLine: 7, Fingerprint: "deploy/aws.sh:aws-access-token:7"},
		},
		GateResult: domain.GateResultBlock, ReportURI: "https://gitlab.example.com/-/jobs/7/artifacts/raw/gitleaks-report.json",
		ScannedAt: at,
		ReportRef: &domain.ScanReportRef{JobName: "app", BuildNumber: 7, StageID: "40", Artifact: "secret-scan-report", Path: "gitleaks-report.json"},
	}
	require.NoError(t, repo.Upsert(ctx, found))

	// 리포트가 없는 실행 — 탐지 건수를 모른다. 0 으로 되읽히면 "탐지 없음" 으로 보인다.
	missing := &domain.SecretScanResult{
		ID: domain.SecretScanResultID("dep_2"), PipelineID: pipelineID, DeploymentID: "dep_2",
		Scanner: "gitleaks", GateResult: domain.GateResultError, ScannedAt: at.Add(time.Minute),
	}
	require.NoError(t, repo.Upsert(ctx, missing))

	list, err := repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Nil(t, list[0].FindingsCount)
	assert.Equal(t, []domain.SecretRuleCount{}, list[0].Rules, "빈 목록은 null 이 아니라 []")
	assert.Equal(t, []domain.SecretFinding{}, list[0].Findings)
	assert.Equal(t, found, list[1])

	// 위치는 상한까지만 둔다. 건수는 정확히 남는다.
	many := make([]domain.SecretFinding, 0, domain.MaxStoredSecretFindings+10)
	for i := 0; i < domain.MaxStoredSecretFindings+10; i++ {
		many = append(many, domain.SecretFinding{RuleID: "generic-api-key", File: "f.txt", StartLine: i + 1,
			Fingerprint: fmt.Sprintf("f.txt:generic-api-key:%d", i+1)})
	}
	total := len(many)
	require.NoError(t, repo.Upsert(ctx, &domain.SecretScanResult{
		ID: domain.SecretScanResultID("dep_3"), PipelineID: pipelineID, DeploymentID: "dep_3",
		Scanner: "gitleaks", FindingsCount: &total, Findings: many,
		Rules:      []domain.SecretRuleCount{{RuleID: "generic-api-key", Count: total}},
		GateResult: domain.GateResultWarn, ScannedAt: at.Add(2 * time.Minute),
	}))
	list, err = repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Len(t, list[0].Findings, domain.MaxStoredSecretFindings)
	assert.Equal(t, total, *list[0].FindingsCount)

	// 비밀값을 담을 열이 없다 — 스키마가 그 자리를 주지 않는다.
	var cols []string
	rows, err := pool.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_name = 'secret_scan_results'`)
	require.NoError(t, err)
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		cols = append(cols, c)
	}
	rows.Close()
	assert.NotContains(t, cols, "secret")
	assert.NotContains(t, cols, "match")

	// 취소된 실행의 기록은 지운다.
	require.NoError(t, repo.Delete(ctx, missing.ID))
	list, err = repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	_, err = pool.Exec(ctx, `UPDATE secret_scan_results SET gate_result = 'maybe' WHERE id = $1`, found.ID)
	assert.Error(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM pipelines WHERE id = $1`, pipelineID)
	require.NoError(t, err)
	list, err = repo.ListByPipelineID(ctx, pipelineID)
	require.NoError(t, err)
	assert.Empty(t, list)
}
