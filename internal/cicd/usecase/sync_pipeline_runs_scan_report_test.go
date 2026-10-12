package usecase

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type stubArtifacts struct {
	data  []byte
	found bool
	err   error
	refs  []port.CIArtifactRef
}

func (s *stubArtifacts) ReadArtifact(_ context.Context, ref port.CIArtifactRef) ([]byte, bool, error) {
	s.refs = append(s.refs, ref)
	return s.data, s.found, s.err
}

func nodeReport(t *testing.T) ([]byte, *domain.TrivyReportSummary) {
	t.Helper()
	raw, err := os.ReadFile("../../shared/domain/testdata/trivy-report-node16.json")
	require.NoError(t, err)
	summary, err := domain.ParseTrivyReport(raw)
	require.NoError(t, err)
	return raw, summary
}

func onlyScan(t *testing.T, scans *memScanResults) *domain.ImageScanResult {
	t.Helper()
	require.Len(t, scans.rows, 1)
	for _, r := range scans.rows {
		return r
	}
	return nil
}

// 판정만으로는 대시보드가 "무엇이 몇 건" 인지 보여줄 수 없다. CI 가 남긴 리포트를
// 읽어 건수·다이제스트·DB 시각을 채운다.
func TestSyncPipelineRuns_FillsScanFromReport(t *testing.T) {
	raw, summary := nodeReport(t)
	arts := &stubArtifacts{data: raw, found: true}
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageSuccess),
	}}, newMemDeployments()).WithImageScans(scans).WithArtifacts(arts)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"})
	require.NoError(t, err)

	got := onlyScan(t, scans)
	require.NotNil(t, got.Counts, "리포트를 읽었는데 건수가 비었다")
	// 기본 정책은 수정본 없는 취약점을 판정에서 뺀다. 건수도 판정이 본 것과 같아야 한다.
	assert.Equal(t, summary.Fixable, *got.Counts)
	assert.Equal(t, summary.ImageDigest, got.ImageDigest)
	assert.Equal(t, summary.ScannerVersion, got.ScannerVersion)
	require.NotNil(t, got.DBUpdatedAt)
	assert.True(t, summary.DBUpdatedAt.Equal(*got.DBUpdatedAt))
	want, _ := domain.GateFromStageAndReport("success", summary, domain.DefaultScanPolicy())
	assert.Equal(t, want, got.GateResult)

	require.Len(t, arts.refs, 1)
	ref := arts.refs[0]
	assert.Equal(t, "app", ref.JobName)
	assert.Equal(t, "main", ref.Branch)
	assert.Equal(t, 7, ref.Build.Number)
	assert.Equal(t, "ImageScan", ref.Stage.Name)
	assert.Equal(t, port.ImageScanReportArtifact, ref.Name)
	assert.Equal(t, port.ImageScanReportFile, ref.Path)
}

// 스캔 명령은 리포트부터 쓴다. 실패했는데 리포트가 없으면 스캐너에 닿지 못한 것이다.
func TestSyncPipelineRuns_FailedScanWithoutReportIsError(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageFailed),
	}}, newMemDeployments()).WithImageScans(scans).WithArtifacts(&stubArtifacts{found: false})

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)

	got := onlyScan(t, scans)
	assert.Equal(t, domain.GateResultError, got.GateResult)
	assert.Nil(t, got.Counts)
}

// 산출물 조회가 일시적으로 실패하면 모르는 것이다. 단계 결과로 남기고 동기화는 계속한다.
func TestSyncPipelineRuns_ReportReadErrorKeepsStageVerdict(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageFailed),
	}}, newMemDeployments()).WithImageScans(scans).WithArtifacts(&stubArtifacts{err: errors.New("timeout")})

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)

	got := onlyScan(t, scans)
	assert.Equal(t, domain.GateResultBlock, got.GateResult)
	assert.Nil(t, got.Counts)
}

// 화면을 열 때마다 동기화가 돈다. 이미 읽은 리포트를 30개씩 다시 내려받지 않는다.
func TestSyncPipelineRuns_DoesNotRereadRecordedReport(t *testing.T) {
	raw, _ := nodeReport(t)
	arts := &stubArtifacts{data: raw, found: true}
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageSuccess),
	}}, newMemDeployments()).WithImageScans(scans).WithArtifacts(arts)

	for i := 0; i < 3; i++ {
		_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
		require.NoError(t, err)
	}
	assert.Len(t, arts.refs, 1)
	assert.NotNil(t, onlyScan(t, scans).Counts)
}

// 리포트를 읽지 못한 기록은 다음 동기화 때 다시 시도한다 — 한 번 실패로 영원히 비지 않게.
func TestSyncPipelineRuns_RetriesReportAfterReadError(t *testing.T) {
	raw, _ := nodeReport(t)
	arts := &stubArtifacts{err: errors.New("timeout")}
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageSuccess),
	}}, newMemDeployments()).WithImageScans(scans).WithArtifacts(arts)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.Nil(t, onlyScan(t, scans).Counts)

	arts.err, arts.data, arts.found = nil, raw, true
	_, err = uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.NotNil(t, onlyScan(t, scans).Counts)
	_ = time.Second
}
