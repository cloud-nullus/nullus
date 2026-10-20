package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// gitleaksOneFinding 은 --redact 리포트다. Secret · Match 는 가려져 있어도 남기지 않는다.
const gitleaksOneFinding = `[{"Description":"GitHub Personal Access Token","StartLine":3,"Match":"token = REDACTED",
"Secret":"REDACTED","File":"config/app.env","Commit":"9f1c2ab","RuleID":"github-pat",
"Fingerprint":"9f1c2ab:config/app.env:github-pat:3"}]`

// memSecretScans 는 port.SecretScanResultRepository 의 테스트 대역이다.
type memSecretScans struct {
	mu   sync.Mutex
	rows map[string]*domain.SecretScanResult
}

func newMemSecretScans() *memSecretScans {
	return &memSecretScans{rows: map[string]*domain.SecretScanResult{}}
}

func (m *memSecretScans) Upsert(_ context.Context, r *domain.SecretScanResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.rows[r.ID] = &cp
	return nil
}

func (m *memSecretScans) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, id)
	return nil
}

func (m *memSecretScans) ListByPipelineID(_ context.Context, pipelineID string) ([]*domain.SecretScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*domain.SecretScanResult{}
	for _, r := range m.rows {
		if r.PipelineID == pipelineID {
			cp := *r
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScannedAt.After(out[j].ScannedAt) })
	return out, nil
}

func secretStarted() time.Time { return started().Add(time.Second) }

func buildWithSecretStage(number int, status port.CIStageStatus) port.CIBuild {
	return port.CIBuild{
		Number: number, Result: "FAILURE", StartedAt: started(), Duration: time.Minute,
		Stages: []port.CIStage{
			// GitLab·GitHub 은 잡 키 secret-scan, Jenkins 는 SecretScan 이다.
			{ID: "38", Name: "SecretScan", Status: status, StartedAt: secretStarted()},
			{Name: "Build", Status: port.CIStageSkipped},
		},
	}
}

func syncSecret(t *testing.T, builds []port.CIBuild, arts port.CIArtifactReader, repo port.SecretScanResultRepository, policy *domain.ScanPolicy) {
	t.Helper()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: builds}, newMemDeployments()).WithSecretScans(repo)
	if arts != nil {
		uc = uc.WithArtifacts(arts)
	}
	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main", Policy: policy})
	require.NoError(t, err)
}

func secretRows(t *testing.T, repo *memSecretScans) []*domain.SecretScanResult {
	t.Helper()
	list, err := repo.ListByPipelineID(context.Background(), "pip_1")
	require.NoError(t, err)
	return list
}

func onlySecretScan(t *testing.T, repo *memSecretScans) *domain.SecretScanResult {
	t.Helper()
	list := secretRows(t, repo)
	require.Len(t, list, 1)
	return list[0]
}

// 시크릿 탐지가 CI 산출물로만 남으면 대시보드가 경고를 통과와 가르지 못한다(G2). 탐지 위치와 규칙별 건수를
// 남기고, 비밀값은 남기지 않는다.
func TestSyncPipelineRuns_RecordsSecretScanFromReport(t *testing.T) {
	raw := strings.ReplaceAll(gitleaksOneFinding, "REDACTED", "ghp_live_value")
	arts := &linkingArtifacts{stubArtifacts: &stubArtifacts{data: []byte(raw), found: true}, base: "https://ci"}
	repo := newMemSecretScans()

	syncSecret(t, []port.CIBuild{buildWithSecretStage(7, port.CIStageFailed)}, arts, repo, nil)

	got := onlySecretScan(t, repo)
	assert.Equal(t, domain.SecretScanResultID("dep_ci_pip_1_7"), got.ID)
	assert.Equal(t, "dep_ci_pip_1_7", got.DeploymentID)
	assert.Equal(t, "gitleaks", got.Scanner)
	assert.Equal(t, domain.GateResultBlock, got.GateResult)
	require.NotNil(t, got.FindingsCount)
	assert.Equal(t, 1, *got.FindingsCount)
	assert.Equal(t, []domain.SecretRuleCount{{RuleID: "github-pat", Count: 1}}, got.Rules)
	require.Len(t, got.Findings, 1)
	assert.Equal(t, domain.SecretFinding{RuleID: "github-pat", File: "config/app.env", StartLine: 3, Commit: "9f1c2ab",
		Fingerprint: "9f1c2ab:config/app.env:github-pat:3"}, got.Findings[0])
	assert.True(t, secretStarted().Equal(got.ScannedAt))
	assert.Equal(t, "https://ci/app/7", got.ReportURI)
	require.NotNil(t, got.ReportRef)
	assert.Equal(t, port.SecretScanReportArtifact, got.ReportRef.Artifact)
	assert.Equal(t, port.SecretScanReportFile, got.ReportRef.Path)

	require.NotEmpty(t, arts.refs)
	ref := arts.refs[len(arts.refs)-1]
	assert.Equal(t, port.SecretScanReportFile, ref.Path)
	assert.Equal(t, "38", ref.Stage.ID, "GitLab 은 잡 ID 로 산출물을 찾는다")
}

// 설계 §8 G2 의 표 — 단계 결과와 탐지 건수로 판정을 추론한다.
func TestSyncPipelineRuns_SecretScanVerdicts(t *testing.T) {
	warn := domain.DefaultScanPolicy()
	warn.SecretDetection = domain.SecretDetectionWarn
	cases := []struct {
		name   string
		status port.CIStageStatus
		report string
		found  bool
		policy *domain.ScanPolicy
		want   domain.GateResult
	}{
		{"탐지 · 실패 = 차단", port.CIStageFailed, gitleaksOneFinding, true, nil, domain.GateResultBlock},
		{"탐지 · 성공 = 경고 정책이 통과시켰다", port.CIStageSuccess, gitleaksOneFinding, true, &warn, domain.GateResultWarn},
		{"탐지 0 · 성공 = 통과", port.CIStageSuccess, "[]", true, nil, domain.GateResultPass},
		{"탐지 0 · 실패 = 스캔 실패", port.CIStageFailed, "[]", true, nil, domain.GateResultError},
		{"리포트 없음 · 성공 = 판정 불가", port.CIStageSuccess, "", false, nil, domain.GateResultError},
		{"리포트 없음 · 실패 = 판정 불가", port.CIStageFailed, "", false, nil, domain.GateResultError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemSecretScans()
			syncSecret(t, []port.CIBuild{buildWithSecretStage(7, tc.status)},
				&stubArtifacts{data: []byte(tc.report), found: tc.found}, repo, tc.policy)
			got := onlySecretScan(t, repo)
			assert.Equal(t, tc.want, got.GateResult)
			if !tc.found {
				assert.Nil(t, got.FindingsCount, "리포트가 없으면 탐지 건수를 모른다 — 0 이 아니다")
				assert.Empty(t, got.ReportURI)
				assert.Nil(t, got.ReportRef)
			}
		})
	}
}

// 스택 정책이 off 면 단계는 돌되 바로 통과한다. 그것을 "통과" 로 남기면 대시보드가 탐지하지 않은 앱을
// 초록으로 보인다 — 남기지 않는다(대시보드의 "미적용").
func TestSyncPipelineRuns_SecretScanPolicyOffRecordsNothing(t *testing.T) {
	off := domain.DefaultScanPolicy()
	off.SecretDetection = domain.SecretDetectionOff
	repo := newMemSecretScans()
	arts := &pathArtifacts{files: map[string][]byte{port.SecretScanReportFile: []byte("[]")}}

	syncSecret(t, []port.CIBuild{buildWithSecretStage(7, port.CIStageSuccess)}, arts, repo, &off)

	assert.Empty(t, secretRows(t, repo))
	assert.Zero(t, arts.readsOf(port.SecretScanReportFile), "남기지 않을 리포트는 내려받지도 않는다")
}

// 취소는 스캔 실패가 아니다 — 남아 있던 기록을 걷어낸다. 도는 중인 것은 판정하지 않는다.
func TestSyncPipelineRuns_SecretScanCanceledOrRunning(t *testing.T) {
	repo := newMemSecretScans()
	require.NoError(t, repo.Upsert(context.Background(), &domain.SecretScanResult{
		ID: domain.SecretScanResultID("dep_ci_pip_1_7"), PipelineID: "pip_1", GateResult: domain.GateResultError, ScannedAt: started(),
	}))
	arts := &pathArtifacts{files: map[string][]byte{port.SecretScanReportFile: []byte(gitleaksOneFinding)}}

	syncSecret(t, []port.CIBuild{buildWithSecretStage(7, port.CIStageCanceled)}, arts, repo, nil)
	assert.Empty(t, secretRows(t, repo))

	for _, status := range []port.CIStageStatus{port.CIStageRunning, port.CIStageQueued, port.CIStageSkipped} {
		syncSecret(t, []port.CIBuild{buildWithSecretStage(8, status)}, arts, repo, nil)
	}
	assert.Empty(t, secretRows(t, repo))
	assert.Zero(t, arts.readsOf(port.SecretScanReportFile))
}

// 읽지 못한 리포트로는 경고와 통과를 가를 수 없다. 남기지 않고 다음 동기화 때 다시 읽는다.
func TestSyncPipelineRuns_SecretScanReadErrorRetriesLater(t *testing.T) {
	repo := newMemSecretScans()
	builds := []port.CIBuild{buildWithSecretStage(7, port.CIStageFailed)}

	syncSecret(t, builds, &stubArtifacts{err: errors.New("gitlab 503")}, repo, nil)
	assert.Empty(t, secretRows(t, repo))

	// 형식이 깨진 리포트도 판정의 근거가 아니다.
	syncSecret(t, builds, &stubArtifacts{data: []byte(`{"broken":`), found: true}, repo, nil)
	assert.Empty(t, secretRows(t, repo))

	syncSecret(t, builds, &stubArtifacts{data: []byte(gitleaksOneFinding), found: true}, repo, nil)
	assert.Len(t, secretRows(t, repo), 1)
}

// 화면을 열 때마다 동기화가 돈다. 리포트까지 읽은 끝난 실행은 다시 내려받지 않는다 — 잡을 다시 돌렸으면 다시 읽는다.
func TestSyncPipelineRuns_DoesNotRereadRecordedSecretReport(t *testing.T) {
	repo := newMemSecretScans()
	arts := &pathArtifacts{files: map[string][]byte{port.SecretScanReportFile: []byte(gitleaksOneFinding)}}
	builds := []port.CIBuild{buildWithSecretStage(7, port.CIStageFailed)}

	syncSecret(t, builds, arts, repo, nil)
	syncSecret(t, builds, arts, repo, nil)
	assert.Equal(t, 1, arts.readsOf(port.SecretScanReportFile))

	retried := buildWithSecretStage(7, port.CIStageSuccess)
	retried.Stages[0].StartedAt = secretStarted().Add(10 * time.Minute)
	arts.files[port.SecretScanReportFile] = []byte("[]")
	syncSecret(t, []port.CIBuild{retried}, arts, repo, nil)
	assert.Equal(t, 2, arts.readsOf(port.SecretScanReportFile))
	assert.Equal(t, domain.GateResultPass, onlySecretScan(t, repo).GateResult)
}

// 산출물 조회가 없으면 탐지 건수를 모른다 — 경고와 통과를 가를 수 없어 남기지 않는다.
func TestSyncPipelineRuns_SecretScanNeedsArtifactReader(t *testing.T) {
	repo := newMemSecretScans()
	syncSecret(t, []port.CIBuild{buildWithSecretStage(7, port.CIStageSuccess)}, nil, repo, nil)
	assert.Empty(t, secretRows(t, repo))
}

type failingSecretRepo struct{ *memSecretScans }

func (failingSecretRepo) Upsert(context.Context, *domain.SecretScanResult) error {
	return errors.New("db down")
}

// 기록 실패로 실행 기록 동기화를 실패시키지 않는다.
func TestSyncPipelineRuns_SecretRecordFailureDoesNotFailSync(t *testing.T) {
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{buildWithSecretStage(7, port.CIStageFailed)}}, newMemDeployments()).
		WithSecretScans(failingSecretRepo{newMemSecretScans()}).
		WithArtifacts(&stubArtifacts{data: []byte(gitleaksOneFinding), found: true})
	n, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"})
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

// 주기 동기화는 스택 정책을 읽어 넘긴다. off 인 스택의 실행은 남기지 않는다.
func TestSyncPipelineRuns_SyncAll_SecretPolicyOffFromStack(t *testing.T) {
	secrets := newMemSecretScans()
	policies := newMemPolicies()
	off := domain.DefaultScanPolicy()
	off.SecretDetection = domain.SecretDetectionOff
	policies.rows["stk_a"] = off
	factory := &countingBundleFactory{calls: map[string]int{}, bundles: map[string]*port.SCMBundle{
		"stk_a": {
			CIBuilds:    &stubBuildReader{builds: []port.CIBuild{buildWithSecretStage(7, port.CIStageSuccess)}},
			CIArtifacts: &pathArtifacts{files: map[string][]byte{port.SecretScanReportFile: []byte("[]")}},
		},
	}}
	uc := NewSyncPipelineRuns(nil, newMemDeployments()).
		WithSecretScans(secrets).
		WithScanPolicies(policies).
		WithBundleFactory(factory, nil).
		WithSyncablePipelines(staticSyncablePipelines{{ID: "pip_1", Name: "app", StackID: "stk_a"}})

	_, err := uc.SyncAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, secretRows(t, secrets))
}
