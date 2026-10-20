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

// depReport 는 trivy fs 리포트다 — 락파일 둘(package-lock.json · go.sum), 수정본 있는 CRITICAL 하나와
// 수정본 없는 CRITICAL 하나, 수정본 있는 HIGH 하나.
const depReport = `{"SchemaVersion":2,"ArtifactName":".","ArtifactType":"filesystem",
"Trivy":{"Version":"0.58.1","Server":{"VulnerabilityDB":{"UpdatedAt":"2026-10-19T00:00:00Z"}}},
"Results":[
 {"Target":"package-lock.json","Class":"lang-pkgs","Type":"npm","Vulnerabilities":[
   {"Severity":"CRITICAL","FixedVersion":"4.17.21"},{"Severity":"CRITICAL","FixedVersion":""}]},
 {"Target":"go.sum","Class":"lang-pkgs","Type":"gomod","Vulnerabilities":[{"Severity":"HIGH","FixedVersion":"0.1.2"}]}]}`

// depNoLockfile 은 락파일이 없는 저장소의 리포트다. 검사한 것이 없다.
const depNoLockfile = `{"SchemaVersion":2,"ArtifactName":".","ArtifactType":"filesystem","Trivy":{"Version":"0.58.1"}}`

// memDependencyScans 는 port.DependencyScanResultRepository 의 테스트 대역이다.
type memDependencyScans struct {
	mu   sync.Mutex
	rows map[string]*domain.DependencyScanResult
}

func newMemDependencyScans() *memDependencyScans {
	return &memDependencyScans{rows: map[string]*domain.DependencyScanResult{}}
}

func (m *memDependencyScans) Upsert(_ context.Context, r *domain.DependencyScanResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.rows[r.ID] = &cp
	return nil
}

func (m *memDependencyScans) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, id)
	return nil
}

func (m *memDependencyScans) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.DependencyScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*domain.DependencyScanResult{}
	for _, r := range m.rows {
		if r.PipelineID == pipelineID {
			cp := *r
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScannedAt.After(out[j].ScannedAt) })
	return out, nil
}

func depStarted() time.Time { return started().Add(3 * time.Second) }

func buildWithDependencyStage(number int, status port.CIStageStatus) port.CIBuild {
	return port.CIBuild{
		Number: number, Result: "FAILURE", StartedAt: started(), Duration: time.Minute,
		Stages: []port.CIStage{
			// GitLab·GitHub 은 잡 키 dependency-scan, Jenkins 는 DependencyScan 이다.
			{ID: "39", Name: "dependency-scan", Status: status, StartedAt: depStarted()},
			{Name: "Build", Status: port.CIStageSuccess},
		},
	}
}

func syncDependency(t *testing.T, builds []port.CIBuild, arts port.CIArtifactReader, repo port.DependencyScanResultRepository, policy *domain.ScanPolicy) {
	t.Helper()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: builds}, newMemDeployments()).WithDependencyScans(repo)
	if arts != nil {
		uc = uc.WithArtifacts(arts)
	}
	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main", Policy: policy})
	require.NoError(t, err)
}

func onlyDependencyScan(t *testing.T, repo *memDependencyScans) *domain.DependencyScanResult {
	t.Helper()
	list, err := repo.ListByPipelineID(context.Background(), "pip_1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	return list[0]
}

func (p *pathArtifacts) readsOf(path string) int {
	n := 0
	for _, r := range p.reads {
		if r.Path == path {
			n++
		}
	}
	return n
}

// 의존성 스캔이 CI 산출물로만 남으면 대시보드의 의존성 열이 빈다(G1). 리포트로 건수 · 검사 대상 수 ·
// 판정을 남긴다. 건수는 정책의 ignore_unfixed 를 따른다 — 판정이 본 숫자와 같아야 한다.
func TestSyncPipelineRuns_RecordsDependencyScanFromReport(t *testing.T) {
	arts := &linkingArtifacts{stubArtifacts: &stubArtifacts{data: []byte(depReport), found: true}, base: "https://ci"}
	repo := newMemDependencyScans()

	syncDependency(t, []port.CIBuild{buildWithDependencyStage(7, port.CIStageFailed)}, arts, repo, nil)

	got := onlyDependencyScan(t, repo)
	assert.Equal(t, domain.DependencyScanResultID("dep_ci_pip_1_7"), got.ID)
	assert.Equal(t, "dep_ci_pip_1_7", got.DeploymentID)
	assert.Equal(t, "trivy", got.Scanner)
	assert.Equal(t, "0.58.1", got.ScannerVersion)
	assert.Equal(t, domain.GateResultBlock, got.GateResult)
	require.NotNil(t, got.Counts)
	assert.Equal(t, domain.SeverityCounts{Critical: 1, High: 1}, *got.Counts, "기본 정책은 수정본 없는 것을 뺀다")
	require.NotNil(t, got.Targets)
	assert.Equal(t, 2, *got.Targets)
	require.NotNil(t, got.DBUpdatedAt)
	assert.True(t, depStarted().Equal(got.ScannedAt))
	assert.Equal(t, "https://ci/app/7", got.ReportURI)
	require.NotNil(t, got.ReportRef)
	assert.Equal(t, port.DependencyScanReportArtifact, got.ReportRef.Artifact)
	assert.Equal(t, port.DependencyScanReportFile, got.ReportRef.Path)
	assert.Equal(t, "39", got.ReportRef.StageID)
}

// 수정본 없는 것도 세는 정책이면 건수에도 넣는다.
func TestSyncPipelineRuns_DependencyCountsFollowPolicy(t *testing.T) {
	repo := newMemDependencyScans()
	policy := domain.DefaultScanPolicy()
	policy.IgnoreUnfixed = false
	syncDependency(t, []port.CIBuild{buildWithDependencyStage(7, port.CIStageFailed)},
		&stubArtifacts{data: []byte(depReport), found: true}, repo, &policy)

	got := onlyDependencyScan(t, repo)
	assert.Equal(t, domain.SeverityCounts{Critical: 2, High: 1}, *got.Counts)
	assert.Equal(t, domain.GateResultBlock, got.GateResult)
}

// 락파일이 없으면 검사 대상 0 으로 통과한다. 0 을 남겨야 대시보드가 "대상 없음" 을 통과와 가른다.
func TestSyncPipelineRuns_DependencyScanWithoutLockfileRecordsZeroTargets(t *testing.T) {
	repo := newMemDependencyScans()
	syncDependency(t, []port.CIBuild{buildWithDependencyStage(7, port.CIStageSuccess)},
		&stubArtifacts{data: []byte(depNoLockfile), found: true}, repo, nil)

	got := onlyDependencyScan(t, repo)
	assert.Equal(t, domain.GateResultPass, got.GateResult)
	require.NotNil(t, got.Targets)
	assert.Zero(t, *got.Targets)
}

// 리포트가 확실히 없으면 스캔했다는 근거가 없다 — 성공이어도 실패여도 error 다. 건수는 모른다.
func TestSyncPipelineRuns_DependencyScanWithoutReportIsError(t *testing.T) {
	for _, status := range []port.CIStageStatus{port.CIStageSuccess, port.CIStageFailed} {
		t.Run(string(status), func(t *testing.T) {
			repo := newMemDependencyScans()
			syncDependency(t, []port.CIBuild{buildWithDependencyStage(7, status)}, &stubArtifacts{found: false}, repo, nil)

			got := onlyDependencyScan(t, repo)
			assert.Equal(t, domain.GateResultError, got.GateResult)
			assert.Nil(t, got.Counts, "모르는 건수를 0 으로 채우지 않는다")
			assert.Nil(t, got.Targets)
			assert.Empty(t, got.ReportURI, "없는 파일로 링크를 걸지 않는다")
			assert.Nil(t, got.ReportRef)
		})
	}
}

// 리포트를 읽지 못한 것은 모르는 것이다. 단계 결과로 남기고 다음 동기화 때 다시 읽는다.
func TestSyncPipelineRuns_DependencyScanReadErrorFallsBackToStage(t *testing.T) {
	repo := newMemDependencyScans()
	builds := []port.CIBuild{buildWithDependencyStage(7, port.CIStageFailed)}
	syncDependency(t, builds, &stubArtifacts{err: errors.New("gitlab 503")}, repo, nil)

	got := onlyDependencyScan(t, repo)
	assert.Equal(t, domain.GateResultBlock, got.GateResult)
	assert.Nil(t, got.Counts)

	syncDependency(t, builds, &stubArtifacts{data: []byte(depReport), found: true}, repo, nil)
	got = onlyDependencyScan(t, repo)
	require.NotNil(t, got.Counts, "다음 동기화가 다시 읽어 채운다")
}

// 화면을 열 때마다 동기화가 돈다. 리포트까지 읽은 끝난 실행은 다시 내려받지 않는다 — 단, 잡을 다시
// 돌렸으면(시작 시각이 바뀌었으면) 다시 읽는다.
func TestSyncPipelineRuns_DoesNotRereadRecordedDependencyReport(t *testing.T) {
	repo := newMemDependencyScans()
	arts := &pathArtifacts{files: map[string][]byte{port.DependencyScanReportFile: []byte(depReport)}}
	builds := []port.CIBuild{buildWithDependencyStage(7, port.CIStageFailed)}

	syncDependency(t, builds, arts, repo, nil)
	syncDependency(t, builds, arts, repo, nil)
	assert.Equal(t, 1, arts.readsOf(port.DependencyScanReportFile))

	retried := buildWithDependencyStage(7, port.CIStageSuccess)
	retried.Stages[0].StartedAt = depStarted().Add(10 * time.Minute)
	arts.files[port.DependencyScanReportFile] = []byte(depNoLockfile)
	syncDependency(t, []port.CIBuild{retried}, arts, repo, nil)
	assert.Equal(t, 2, arts.readsOf(port.DependencyScanReportFile))
	assert.Equal(t, domain.GateResultPass, onlyDependencyScan(t, repo).GateResult)
}

// 취소는 스캔 실패가 아니다 — 판정을 남기지 않고, 남아 있던 기록은 걷어낸다. 도는 중인 것은 판정하지 않는다.
func TestSyncPipelineRuns_DependencyScanCanceledOrRunning(t *testing.T) {
	repo := newMemDependencyScans()
	require.NoError(t, repo.Upsert(context.Background(), &domain.DependencyScanResult{
		ID: domain.DependencyScanResultID("dep_ci_pip_1_7"), PipelineID: "pip_1", GateResult: domain.GateResultError, ScannedAt: started(),
	}))
	arts := &pathArtifacts{files: map[string][]byte{port.DependencyScanReportFile: []byte(depReport)}}

	syncDependency(t, []port.CIBuild{buildWithDependencyStage(7, port.CIStageCanceled)}, arts, repo, nil)
	list, err := repo.ListByPipelineID(context.Background(), "pip_1")
	require.NoError(t, err)
	assert.Empty(t, list)

	for _, status := range []port.CIStageStatus{port.CIStageRunning, port.CIStageQueued, port.CIStageSkipped} {
		syncDependency(t, []port.CIBuild{buildWithDependencyStage(8, status)}, arts, repo, nil)
	}
	list, err = repo.ListByPipelineID(context.Background(), "pip_1")
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.Zero(t, arts.readsOf(port.DependencyScanReportFile))
}

// 산출물 조회가 배선되지 않았으면 단계 결과로만 판정한다(이미지 스캔과 같다). 건수는 모른다.
func TestSyncPipelineRuns_DependencyScanWithoutArtifactReader(t *testing.T) {
	repo := newMemDependencyScans()
	syncDependency(t, []port.CIBuild{buildWithDependencyStage(7, port.CIStageSuccess)}, nil, repo, nil)

	got := onlyDependencyScan(t, repo)
	assert.Equal(t, domain.GateResultPass, got.GateResult)
	assert.Nil(t, got.Counts)
}

type failingDependencyRepo struct{ *memDependencyScans }

func (failingDependencyRepo) Upsert(context.Context, *domain.DependencyScanResult) error {
	return errors.New("db down")
}

// 기록 실패로 실행 기록 동기화를 실패시키지 않는다.
func TestSyncPipelineRuns_DependencyRecordFailureDoesNotFailSync(t *testing.T) {
	deployments := newMemDeployments()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{buildWithDependencyStage(7, port.CIStageFailed)}}, deployments).
		WithDependencyScans(failingDependencyRepo{newMemDependencyScans()}).
		WithArtifacts(&stubArtifacts{data: []byte(depReport), found: true})
	n, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"})
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

// 주기·화면 동기화는 스택 번들로 다시 조립한다. 거기서 빠지면 의존성 결과가 쌓이지 않는다.
func TestSyncPipelineRuns_SyncAll_RecordsDependencyAndSecretThroughBundle(t *testing.T) {
	deps := newMemDependencyScans()
	secrets := newMemSecretScans()
	build := buildWithDependencyStage(7, port.CIStageFailed)
	build.Stages = append(build.Stages, port.CIStage{ID: "38", Name: "secret-scan", Status: port.CIStageFailed, StartedAt: started()})
	factory := &countingBundleFactory{calls: map[string]int{}, bundles: map[string]*port.SCMBundle{
		"stk_a": {
			CIBuilds: &stubBuildReader{builds: []port.CIBuild{build}},
			CIArtifacts: &pathArtifacts{files: map[string][]byte{
				port.DependencyScanReportFile: []byte(depReport),
				port.SecretScanReportFile:     []byte(gitleaksOneFinding),
			}},
		},
	}}
	uc := NewSyncPipelineRuns(nil, newMemDeployments()).
		WithDependencyScans(deps).
		WithSecretScans(secrets).
		WithBundleFactory(factory, nil).
		WithSyncablePipelines(staticSyncablePipelines{{ID: "pip_1", Name: "app", StackID: "stk_a"}})

	_, err := uc.SyncAll(context.Background())
	require.NoError(t, err)

	dl, err := deps.ListByPipelineID(context.Background(), "pip_1")
	require.NoError(t, err)
	require.Len(t, dl, 1)
	sl, err := secrets.ListByPipelineID(context.Background(), "pip_1")
	require.NoError(t, err)
	require.Len(t, sl, 1)
	assert.Equal(t, domain.GateResultBlock, sl[0].GateResult)
}
