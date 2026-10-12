package usecase

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type memSASTResults struct {
	mu   sync.Mutex
	rows map[string]*domain.SASTResult
}

func newMemSASTResults() *memSASTResults {
	return &memSASTResults{rows: map[string]*domain.SASTResult{}}
}

func (m *memSASTResults) Upsert(_ context.Context, r *domain.SASTResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.rows[r.ID] = &cp
	return nil
}

func (m *memSASTResults) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.SASTResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.SASTResult
	for _, r := range m.rows {
		if r.PipelineID == pipelineID {
			cp := *r
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AnalyzedAt.After(out[j].AnalyzedAt) })
	return out, nil
}

// pathArtifacts 는 파일 경로마다 다른 산출물을 돌려준다 — 한 실행에 이미지 스캔과 SAST
// 리포트가 함께 있다.
type pathArtifacts struct {
	files map[string][]byte
	err   error
	reads []port.CIArtifactRef
}

func (p *pathArtifacts) ReadArtifact(_ context.Context, ref port.CIArtifactRef) ([]byte, bool, error) {
	p.reads = append(p.reads, ref)
	if p.err != nil {
		return nil, false, p.err
	}
	raw, ok := p.files[ref.Path]
	return raw, ok, nil
}

func (p *pathArtifacts) sastReads() int {
	n := 0
	for _, r := range p.reads {
		if r.Path == port.SASTReportFile {
			n++
		}
	}
	return n
}

const sastGateFailedReport = `{"scanner_exit_code":3,"project_key":"app","ce_task_id":"ce-1","analysis_id":"an-1",
"dashboard_url":"https://sonarqube.example.com/dashboard?id=app",
"quality_gate":{"projectStatus":{"status":"ERROR","conditions":[{"status":"ERROR","metricKey":"new_violations","comparator":"GT","errorThreshold":"0","actualValue":"1"}]}},
"measures":{"component":{"measures":[{"metric":"vulnerabilities","value":"3"},{"metric":"bugs","value":"0"}]}}}`

func sastStarted() time.Time { return started().Add(2 * time.Second) }

func buildWithSASTStage(number int, status port.CIStageStatus) port.CIBuild {
	return port.CIBuild{
		Number: number, Result: "FAILURE", StartedAt: started(), Duration: time.Minute,
		Stages: []port.CIStage{
			{Name: "Build", Status: port.CIStageSuccess},
			// GitLab·GitHub 은 잡 키 sast, Jenkins 는 SAST 다.
			{ID: "41", Name: "sast", Status: status, StartedAt: sastStarted()},
		},
	}
}

func syncSAST(t *testing.T, builds []port.CIBuild, arts port.CIArtifactReader, results *memSASTResults) {
	t.Helper()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: builds}, newMemDeployments()).
		WithSASTResults(results).WithArtifacts(arts)
	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"})
	require.NoError(t, err)
}

// 분석 단계가 남긴 리포트로 Quality Gate 판정·조건·지표를 남긴다. 단계 결과만으로는
// 무엇에 걸렸는지, 경고 정책이 통과시킨 실패인지 알 수 없다.
func TestSyncPipelineRuns_RecordsSASTResultFromReport(t *testing.T) {
	arts := &pathArtifacts{files: map[string][]byte{port.SASTReportFile: []byte(sastGateFailedReport)}}
	results := newMemSASTResults()

	syncSAST(t, []port.CIBuild{buildWithSASTStage(7, port.CIStageFailed)}, arts, results)

	require.Len(t, results.rows, 1)
	got := results.rows[domain.SASTResultID("dep_ci_pip_1_7")]
	require.NotNil(t, got)
	assert.Equal(t, "pip_1", got.PipelineID)
	assert.Equal(t, "dep_ci_pip_1_7", got.DeploymentID)
	assert.Equal(t, domain.GateResultBlock, got.GateResult)
	assert.Equal(t, domain.QualityGateError, got.QualityGateStatus)
	assert.Equal(t, "app", got.ProjectKey)
	assert.Equal(t, "an-1", got.AnalysisID)
	assert.Equal(t, "https://sonarqube.example.com/dashboard?id=app", got.DashboardURL)
	require.Len(t, got.Conditions, 1)
	require.NotNil(t, got.Metrics)
	assert.Equal(t, 3, *got.Metrics.Vulnerabilities)
	assert.True(t, sastStarted().Equal(got.AnalyzedAt))

	require.Equal(t, 1, arts.sastReads())
	ref := arts.reads[len(arts.reads)-1]
	assert.Equal(t, "app", ref.JobName)
	assert.Equal(t, "main", ref.Branch)
	assert.Equal(t, 7, ref.Build.Number)
	assert.Equal(t, "41", ref.Stage.ID, "GitLab 은 잡 ID 로 산출물을 찾는다")
	assert.Equal(t, port.SASTReportArtifact, ref.Name)
	assert.Equal(t, port.SASTReportFile, ref.Path)
}

// 경고 정책은 게이트에 걸려도 단계를 통과시킨다. 배포는 나갔으므로 차단이 아니라 경고다.
func TestSyncPipelineRuns_SASTGateFailureUnderWarnPolicyIsWarn(t *testing.T) {
	arts := &pathArtifacts{files: map[string][]byte{port.SASTReportFile: []byte(sastGateFailedReport)}}
	results := newMemSASTResults()

	syncSAST(t, []port.CIBuild{buildWithSASTStage(7, port.CIStageSuccess)}, arts, results)

	got := results.rows[domain.SASTResultID("dep_ci_pip_1_7")]
	require.NotNil(t, got)
	assert.Equal(t, domain.GateResultWarn, got.GateResult)
}

// 리포트가 없으면 남기지 않는다 — 리포트를 남기기 전에 만든 파이프라인의 실패를 차단으로
// 적으면 분석기 장애가 "보안 문제로 막힌 배포" 로 보인다. 단계 상태는 실행 기록에 따로 있다.
func TestSyncPipelineRuns_SkipsSASTWithoutReport(t *testing.T) {
	results := newMemSASTResults()
	syncSAST(t, []port.CIBuild{buildWithSASTStage(7, port.CIStageFailed)}, &pathArtifacts{files: map[string][]byte{}}, results)
	assert.Empty(t, results.rows)

	// 형식이 깨진 리포트도 판정의 근거가 아니다.
	syncSAST(t, []port.CIBuild{buildWithSASTStage(7, port.CIStageFailed)},
		&pathArtifacts{files: map[string][]byte{port.SASTReportFile: []byte(`{"project_key":"app"}`)}}, results)
	assert.Empty(t, results.rows)
}

// 읽지 못한 것은 모르는 것이다. 다음 동기화 때 다시 읽는다.
func TestSyncPipelineRuns_RetriesSASTReportAfterReadError(t *testing.T) {
	results := newMemSASTResults()
	builds := []port.CIBuild{buildWithSASTStage(7, port.CIStageFailed)}

	syncSAST(t, builds, &pathArtifacts{err: errors.New("gitlab 503")}, results)
	assert.Empty(t, results.rows)

	syncSAST(t, builds, &pathArtifacts{files: map[string][]byte{port.SASTReportFile: []byte(sastGateFailedReport)}}, results)
	assert.Len(t, results.rows, 1)
}

// 끝나지 않은 단계와 취소된 단계는 판정하지 않고, 산출물도 내려받지 않는다.
func TestSyncPipelineRuns_SkipsUnfinishedSASTStage(t *testing.T) {
	for _, status := range []port.CIStageStatus{port.CIStageRunning, port.CIStageQueued, port.CIStageCanceled, port.CIStageSkipped} {
		t.Run(string(status), func(t *testing.T) {
			arts := &pathArtifacts{files: map[string][]byte{port.SASTReportFile: []byte(sastGateFailedReport)}}
			results := newMemSASTResults()
			syncSAST(t, []port.CIBuild{buildWithSASTStage(7, status)}, arts, results)
			assert.Empty(t, results.rows)
			assert.Zero(t, arts.sastReads())
		})
	}
}

// 화면을 열 때마다 동기화가 돈다. 이미 남긴 실행의 리포트를 다시 내려받지 않는다.
func TestSyncPipelineRuns_DoesNotRereadRecordedSAST(t *testing.T) {
	results := newMemSASTResults()
	builds := []port.CIBuild{buildWithSASTStage(7, port.CIStageFailed)}
	arts := &pathArtifacts{files: map[string][]byte{port.SASTReportFile: []byte(sastGateFailedReport)}}

	syncSAST(t, builds, arts, results)
	syncSAST(t, builds, arts, results)

	assert.Equal(t, 1, arts.sastReads())
}

// 주기 동기화·화면 동기화는 스택 번들로 다시 조립한다. 거기서 빠지면 SAST 결과가 쌓이지 않는다.
func TestSyncPipelineRuns_SyncAll_RecordsSASTThroughBundle(t *testing.T) {
	results := newMemSASTResults()
	factory := &countingBundleFactory{calls: map[string]int{}, bundles: map[string]*port.SCMBundle{
		"stk_a": {
			CIBuilds:    &stubBuildReader{builds: []port.CIBuild{buildWithSASTStage(7, port.CIStageFailed)}},
			CIArtifacts: &pathArtifacts{files: map[string][]byte{port.SASTReportFile: []byte(sastGateFailedReport)}},
		},
	}}
	uc := NewSyncPipelineRuns(nil, newMemDeployments()).
		WithSASTResults(results).
		WithBundleFactory(factory, nil).
		WithSyncablePipelines(staticSyncablePipelines{{ID: "pip_1", Name: "app", StackID: "stk_a"}})

	_, err := uc.SyncAll(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, results.rows[domain.SASTResultID("dep_ci_pip_1_7")])
}

// 리포트의 대시보드 주소는 스캐너가 붙은 클러스터 내 주소다. 스택의 공개 주소로 링크를 만든다.
func TestSyncPipelineRuns_SASTDashboardUsesStackWebURL(t *testing.T) {
	results := newMemSASTResults()
	report := []byte(`{"scanner_exit_code":0,"project_key":"app","analysis_id":"an-1",
"dashboard_url":"http://sonarqube.ns.svc.cluster.local:9000/dashboard?id=app","quality_gate":{"projectStatus":{"status":"OK"}},"measures":null}`)
	arts := &pathArtifacts{files: map[string][]byte{port.SASTReportFile: report}}

	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{buildWithSASTStage(7, port.CIStageSuccess)}}, newMemDeployments()).
		WithSASTResults(results).WithArtifacts(arts)
	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{
		PipelineID: "pip_1", JobName: "app", Branch: "main", SASTWebURL: "https://sonarqube.nullus.local",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://sonarqube.nullus.local/dashboard?id=app", results.rows[domain.SASTResultID("dep_ci_pip_1_7")].DashboardURL)

	// 공개 주소를 모르는 스택이면 열리지 않는 링크를 남기지 않는다.
	results = newMemSASTResults()
	syncSAST(t, []port.CIBuild{buildWithSASTStage(7, port.CIStageSuccess)}, arts, results)
	assert.Empty(t, results.rows[domain.SASTResultID("dep_ci_pip_1_7")].DashboardURL)
}

// 주기·화면 동기화는 번들의 공개 주소를 넘긴다.
func TestSyncPipelineRuns_SyncAll_PassesSASTWebURL(t *testing.T) {
	results := newMemSASTResults()
	factory := &countingBundleFactory{calls: map[string]int{}, bundles: map[string]*port.SCMBundle{
		"stk_a": {
			CIBuilds:    &stubBuildReader{builds: []port.CIBuild{buildWithSASTStage(7, port.CIStageFailed)}},
			CIArtifacts: &pathArtifacts{files: map[string][]byte{port.SASTReportFile: []byte(sastGateFailedReport)}},
			SASTWebURL:  "https://sonarqube.nullus.local",
		},
	}}
	uc := NewSyncPipelineRuns(nil, newMemDeployments()).
		WithSASTResults(results).
		WithBundleFactory(factory, nil).
		WithSyncablePipelines(staticSyncablePipelines{{ID: "pip_1", Name: "app", StackID: "stk_a"}})

	_, err := uc.SyncAll(context.Background())
	require.NoError(t, err)

	got := results.rows[domain.SASTResultID("dep_ci_pip_1_7")]
	require.NotNil(t, got)
	assert.Equal(t, "https://sonarqube.nullus.local/dashboard?id=app", got.DashboardURL)
}

// GitLab 의 Retry·GitHub 의 Re-run 은 같은 실행(빌드 번호)에서 잡만 다시 돌린다. 분석기 장애로
// error 가 남은 실행을 고쳐 다시 돌리면 판정이 바뀌어야 한다 — 예전 판정을 붙들고 있으면 배포는
// 나갔는데 배지는 "분석 실패" 로 남는다. 단계 시작 시각이 바뀌었으면 다시 읽는다.
func TestSyncPipelineRuns_RereadsRetriedSASTStage(t *testing.T) {
	results := newMemSASTResults()
	failed := []byte(`{"scanner_exit_code":1,"project_key":"app","quality_gate":null,"measures":null}`)
	syncSAST(t, []port.CIBuild{buildWithSASTStage(7, port.CIStageFailed)},
		&pathArtifacts{files: map[string][]byte{port.SASTReportFile: failed}}, results)
	require.Equal(t, domain.GateResultError, results.rows[domain.SASTResultID("dep_ci_pip_1_7")].GateResult)

	retried := buildWithSASTStage(7, port.CIStageSuccess)
	retried.Stages[1].ID = "57"
	retried.Stages[1].StartedAt = sastStarted().Add(10 * time.Minute)
	passed := []byte(`{"scanner_exit_code":0,"project_key":"app","quality_gate":{"projectStatus":{"status":"OK"}},"measures":null}`)
	arts := &pathArtifacts{files: map[string][]byte{port.SASTReportFile: passed}}
	syncSAST(t, []port.CIBuild{retried}, arts, results)

	got := results.rows[domain.SASTResultID("dep_ci_pip_1_7")]
	assert.Equal(t, domain.GateResultPass, got.GateResult)
	assert.True(t, retried.Stages[1].StartedAt.Equal(got.AnalyzedAt))
	assert.Equal(t, 1, arts.sastReads())
}
