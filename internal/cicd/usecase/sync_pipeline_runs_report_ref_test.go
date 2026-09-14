package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// 취약점 목록은 볼 때 CI 리포트를 다시 읽어 만든다(원본은 DB 에 넣지 않는다). 그러려면
// 리포트가 어디 있는지 남겨야 한다 — GitLab 은 잡 id, GitHub 은 실행 id, Jenkins 는 빌드 번호.
func TestSyncPipelineRuns_RecordsReportRef(t *testing.T) {
	raw, _ := nodeReport(t)
	scans := newMemScanResults()
	build := buildWithScanStage(7, port.CIStageSuccess)
	build.ID = "run-77"
	build.Stages[1].ID = "11"
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{build}}, newMemDeployments()).
		WithImageScans(scans).WithArtifacts(&stubArtifacts{data: raw, found: true})

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"})
	require.NoError(t, err)

	assert.Equal(t, &domain.ScanReportRef{
		JobName: "app", Branch: "main", BuildID: "run-77", BuildNumber: 7,
		StageID: "11", StageName: "ImageScan",
		Artifact: port.ImageScanReportArtifact, Path: port.ImageScanReportFile,
	}, onlyScan(t, scans).ReportRef)
}

// 리포트 위치 기능 전에 건수까지 기록한 실행도 위치를 받는다. 리포트는 다시 내려받지 않는다.
func TestSyncPipelineRuns_BackfillsReportRef(t *testing.T) {
	scans := newMemScanResults()
	counts := domain.SeverityCounts{High: 1}
	require.NoError(t, scans.Upsert(context.Background(), &domain.ImageScanResult{
		ID: "scan_dep_ci_pip_1_7", PipelineID: "pip_1", DeploymentID: "dep_ci_pip_1_7",
		Counts: &counts, GateResult: domain.GateResultWarn, ScannedAt: time.Now(),
	}))
	arts := &stubArtifacts{found: true}
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{buildWithScanStage(7, port.CIStageSuccess)}},
		newMemDeployments()).WithImageScans(scans).WithArtifacts(arts)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"})
	require.NoError(t, err)

	got := onlyScan(t, scans)
	require.NotNil(t, got.ReportRef)
	assert.Equal(t, 7, got.ReportRef.BuildNumber)
	assert.Empty(t, arts.refs, "이미 건수를 채운 리포트를 다시 내려받았다")
}

func TestArtifactRefRoundTrip(t *testing.T) {
	ref := port.CIArtifactRef{
		JobName: "app", Branch: "main",
		Build: port.CIBuild{ID: "run-1", Number: 3},
		Stage: port.CIStage{ID: "9", Name: "image-scan"},
		Name:  port.ImageScanReportArtifact, Path: port.ImageScanReportFile,
	}
	back := artifactRefFrom(reportRefFrom(ref))
	assert.Equal(t, ref.JobName, back.JobName)
	assert.Equal(t, ref.Branch, back.Branch)
	assert.Equal(t, ref.Build.ID, back.Build.ID)
	assert.Equal(t, ref.Build.Number, back.Build.Number)
	assert.Equal(t, ref.Stage.ID, back.Stage.ID)
	assert.Equal(t, ref.Stage.Name, back.Stage.Name)
	assert.Equal(t, ref.Name, back.Name)
	assert.Equal(t, ref.Path, back.Path)
}
