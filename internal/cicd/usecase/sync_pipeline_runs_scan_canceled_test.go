package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 새 커밋이 들어오면 CI 는 앞선 실행을 취소한다. 취소는 스캔 실패가 아니다 —
// error 로 남기면 대시보드가 "스캐너 장애" 로 센다(kind 실측에서 자동 취소된
// 실행이 error 로 기록됐다).
func TestSyncPipelineRuns_CanceledScanLeavesNoRecord(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageCanceled),
	}}, newMemDeployments()).
		WithImageScans(scans).
		WithArtifacts(&stubArtifacts{found: false})

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.Empty(t, scans.rows)
}

// 수정 전에 동기화된 취소 실행은 error 로 남아 있다. 다시 동기화하면 걷어낸다.
func TestSyncPipelineRuns_RemovesErrorRecordedForCanceledRun(t *testing.T) {
	scans := newMemScanResults()
	id := "scan_" + runDeploymentID("pip_1", 7)
	scans.rows[id] = &domain.ImageScanResult{
		ID: id, PipelineID: "pip_1", GateResult: domain.GateResultError, ScannedAt: started(),
	}
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		buildWithScanStage(7, port.CIStageCanceled),
	}}, newMemDeployments()).WithImageScans(scans)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.NotContains(t, scans.rows, id)
}
