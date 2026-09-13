package usecase

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type memScanResults struct {
	rows map[string]*domain.ImageScanResult
}

func newMemScanResults() *memScanResults {
	return &memScanResults{rows: map[string]*domain.ImageScanResult{}}
}

func (m *memScanResults) Upsert(_ context.Context, r *domain.ImageScanResult) error {
	cp := *r
	m.rows[r.ID] = &cp
	return nil
}

func (m *memScanResults) ListByPipelineID(_ context.Context, id string) ([]*domain.ImageScanResult, error) {
	var out []*domain.ImageScanResult
	for _, r := range m.rows {
		if r.PipelineID == id {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScannedAt.After(out[j].ScannedAt) })
	return out, nil
}

func buildWithScanStage(number int, scanStatus port.CIStageStatus) port.CIBuild {
	return port.CIBuild{
		Number: number, Result: "FAILURE", StartedAt: started(), Duration: time.Minute,
		Stages: []port.CIStage{
			{Name: "Build", Status: port.CIStageSuccess},
			{Name: "ImageScan", Status: scanStatus, StartedAt: started().Add(time.Second)},
		},
	}
}

// 스캔 단계가 끝난 실행은 게이트 판정을 기록한다. 기록이 없으면 대시보드(#65)가
// "차단된 배포" 를 셀 방법이 없다.
func TestSyncPipelineRuns_RecordsImageScanGate(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageFailed),
	}}, newMemDeployments()).WithImageScans(scans)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)

	got, err := scans.ListByPipelineID(context.Background(), "pip_1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, domain.GateResultBlock, got[0].GateResult)
	assert.Equal(t, runDeploymentID("pip_1", 7), got[0].DeploymentID)
	assert.Equal(t, "trivy", got[0].Scanner)
	// 리포트를 읽지 않았으므로 건수를 지어내지 않는다.
	assert.Nil(t, got[0].Counts, "건수를 0 으로 채우면 취약점 0건으로 읽힌다")
}

// 같은 실행을 다시 동기화해도 기록이 늘지 않는다.
func TestSyncPipelineRuns_ImageScanIsIdempotent(t *testing.T) {
	scans := newMemScanResults()
	reader := &stubBuildReader{builds: []port.CIBuild{buildWithScanStage(7, port.CIStageSuccess)}}
	uc := NewSyncPipelineRuns(reader, newMemDeployments()).WithImageScans(scans)

	for i := 0; i < 2; i++ {
		_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
		require.NoError(t, err)
	}
	assert.Len(t, scans.rows, 1)
}

// 도는 중인 스캔은 판정하지 않는다 — 통과로 적으면 안 된다.
func TestSyncPipelineRuns_SkipsUnfinishedImageScan(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageRunning),
	}}, newMemDeployments()).WithImageScans(scans)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.Empty(t, scans.rows)
}

// GitLab·GitHub 은 스캔 단계를 잡 키 image-scan 으로 보고한다. Jenkins 의
// ImageScan 과 같은 단계다 — 대소문자만 맞추면 판정이 기록되지 않는다.
func TestSyncPipelineRuns_RecordsImageScanGateFromJobKeyName(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		{Number: 3, Result: "FAILURE", StartedAt: started(), Duration: time.Minute,
			Stages: []port.CIStage{
				{Name: "build", Status: port.CIStageSuccess},
				{Name: "image-scan", Status: port.CIStageFailed},
			}},
	}}, newMemDeployments()).WithImageScans(scans)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	require.Len(t, scans.rows, 1)
	for _, r := range scans.rows {
		assert.Equal(t, domain.GateResultBlock, r.GateResult)
	}
}

// 스캔 단계가 없는 파이프라인에는 아무것도 남기지 않는다.
func TestSyncPipelineRuns_NoScanStageNoRecord(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		{Number: 1, Result: "SUCCESS", StartedAt: started(), Duration: time.Second,
			Stages: []port.CIStage{{Name: "Build", Status: port.CIStageSuccess}}},
	}}, newMemDeployments()).WithImageScans(scans)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.Empty(t, scans.rows)
}
