package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// highOnlyReport 는 수정본 있는 HIGH 하나만 담은 리포트다.
const highOnlyReport = `{"SchemaVersion":2,"ArtifactName":"registry.local/shop/api:abc",
"Results":[{"Vulnerabilities":[{"Severity":"HIGH","FixedVersion":"1.2.3"}]}]}`

var highBlockPolicy = domain.ScanPolicy{
	BlockSeverity: domain.SeverityHigh, IgnoreUnfixed: true, OnScannerUnreachable: domain.UnreachableBlock,
}

// 스택 정책이 HIGH 차단이면, CI 는 HIGH 로 막는다. 동기화가 기본 정책(CRITICAL)으로
// 판정하면 그 실행을 "차단 사유 없는 실패 = 스캔 오류" 로 잘못 센다.
func TestSyncPipelineRuns_EvaluatesWithStackPolicy(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{buildWithScanStage(7, port.CIStageFailed)}},
		newMemDeployments()).
		WithImageScans(scans).
		WithArtifacts(&stubArtifacts{data: []byte(highOnlyReport), found: true})

	policy := highBlockPolicy
	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Policy: &policy})
	require.NoError(t, err)

	got := onlyScan(t, scans)
	assert.Equal(t, domain.GateResultBlock, got.GateResult)
	require.NotNil(t, got.Counts)
	assert.Equal(t, 1, got.Counts.High)
}

func TestSyncPipelineRuns_WithoutPolicyUsesDefault(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{buildWithScanStage(7, port.CIStageFailed)}},
		newMemDeployments()).
		WithImageScans(scans).
		WithArtifacts(&stubArtifacts{data: []byte(highOnlyReport), found: true})

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.Equal(t, domain.GateResultError, onlyScan(t, scans).GateResult,
		"기본 정책에서 HIGH 는 차단 사유가 아니다")
}

// 화면이 부르는 경로(ForPipeline)가 파이프라인의 스택 정책을 찾아 넘긴다.
func TestSyncPipelineRuns_ForPipelineLoadsStackPolicy(t *testing.T) {
	policies := newMemPolicies()
	policies.rows["stk_1"] = highBlockPolicy
	scans := newMemScanResults()
	factory := &fakeBundleFactory{bundle: &port.SCMBundle{
		CIBuilds:    &stubBuildReader{builds: []port.CIBuild{buildWithScanStage(7, port.CIStageFailed)}},
		CIArtifacts: &stubArtifacts{data: []byte(highOnlyReport), found: true},
	}}
	pipelines := &fakePipelineRepo{pipeline: &domain.Pipeline{ID: "pip_1", Name: "app", StackID: "stk_1"}}

	uc := NewSyncPipelineRuns(nil, newMemDeployments()).
		WithImageScans(scans).
		WithBundleFactory(factory, pipelines).
		WithScanPolicies(policies)

	_, err := uc.ForPipeline(context.Background(), "pip_1")
	require.NoError(t, err)
	assert.Equal(t, domain.GateResultBlock, onlyScan(t, scans).GateResult)
}
