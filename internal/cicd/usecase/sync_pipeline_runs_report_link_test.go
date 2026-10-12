package usecase

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type linkingArtifacts struct {
	*stubArtifacts
	base string
}

func (l *linkingArtifacts) ArtifactWebURL(ref port.CIArtifactRef) string {
	return l.base + "/" + ref.JobName + "/" + strconv.Itoa(ref.Build.Number)
}

// 건수만으로는 무엇에 걸렸는지 모른다. 리포트를 읽은 실행에는 CI 리포트로 가는 링크를 남긴다.
func TestSyncPipelineRuns_FillsReportLink(t *testing.T) {
	raw, _ := nodeReport(t)
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{buildWithScanStage(7, port.CIStageSuccess)}},
		newMemDeployments()).WithImageScans(scans).
		WithArtifacts(&linkingArtifacts{stubArtifacts: &stubArtifacts{data: raw, found: true}, base: "https://ci"})

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"})
	require.NoError(t, err)

	assert.Equal(t, "https://ci/app/7", onlyScan(t, scans).ReportURI)
}

// 리포트가 없는 실행에는 링크를 걸지 않는다 — 열어도 없는 파일이다.
func TestSyncPipelineRuns_NoReportLinkWithoutReport(t *testing.T) {
	scans := newMemScanResults()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{buildWithScanStage(7, port.CIStageFailed)}},
		newMemDeployments()).WithImageScans(scans).
		WithArtifacts(&linkingArtifacts{stubArtifacts: &stubArtifacts{found: false}, base: "https://ci"})

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)

	assert.Empty(t, onlyScan(t, scans).ReportURI)
}

// 링크 기능 전에 기록한 실행도 링크를 받는다. 리포트는 다시 내려받지 않는다.
func TestSyncPipelineRuns_BackfillsReportLinkForRecordedScan(t *testing.T) {
	raw, _ := nodeReport(t)
	scans := newMemScanResults()
	builds := &stubBuildReader{builds: []port.CIBuild{buildWithScanStage(7, port.CIStageSuccess)}}
	input := SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"}

	_, err := NewSyncPipelineRuns(builds, newMemDeployments()).WithImageScans(scans).
		WithArtifacts(&stubArtifacts{data: raw, found: true}).Execute(context.Background(), input)
	require.NoError(t, err)
	require.Empty(t, onlyScan(t, scans).ReportURI)

	second := &linkingArtifacts{stubArtifacts: &stubArtifacts{data: raw, found: true}, base: "https://ci"}
	_, err = NewSyncPipelineRuns(builds, newMemDeployments()).WithImageScans(scans).
		WithArtifacts(second).Execute(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, "https://ci/app/7", onlyScan(t, scans).ReportURI)
	assert.Empty(t, second.refs, "이미 건수를 채운 리포트를 다시 내려받았다")
}
