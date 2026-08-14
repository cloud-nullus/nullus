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

type stubBuildReader struct{ builds []port.CIBuild }

func (s *stubBuildReader) ListBuilds(context.Context, string, string, int) ([]port.CIBuild, error) {
	return s.builds, nil
}

type memDeployments struct{ rows map[string]*domain.Deployment }

func newMemDeployments() *memDeployments {
	return &memDeployments{rows: map[string]*domain.Deployment{}}
}
func (m *memDeployments) Create(_ context.Context, d *domain.Deployment) error {
	m.rows[d.ID] = d
	return nil
}
func (m *memDeployments) GetByID(_ context.Context, id string) (*domain.Deployment, error) {
	return m.rows[id], nil
}
func (m *memDeployments) ListByPipelineID(context.Context, string) ([]*domain.Deployment, error) {
	return nil, nil
}
func (m *memDeployments) Update(_ context.Context, d *domain.Deployment) error {
	m.rows[d.ID] = d
	return nil
}

func started() time.Time { return time.UnixMilli(1786721616732) }

// GitOps 경로의 실행 기록은 CI 서버에만 있다. 들이지 않으면 빌드가 성공해도
// 화면의 실행 통계가 영원히 0 으로 남는다.
func TestSyncPipelineRuns_RecordsFinishedBuild(t *testing.T) {
	repo := newMemDeployments()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		{Number: 1, Result: "SUCCESS", StartedAt: started(), Duration: 26494 * time.Millisecond},
	}}, repo)

	n, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app", Branch: "main"})
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	d := repo.rows[runDeploymentID("pip_1", 1)]
	require.NotNil(t, d)
	assert.Equal(t, domain.DeploymentStatusSuccess, d.Status)
	assert.Equal(t, "#1", d.Version)
	require.NotNil(t, d.CompletedAt)
	assert.Equal(t, started().Add(26494*time.Millisecond), *d.CompletedAt)
}

// 같은 빌드를 다시 동기화해도 기록이 늘면 안 된다 — 통계가 부풀려진다.
func TestSyncPipelineRuns_IsIdempotent(t *testing.T) {
	repo := newMemDeployments()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		{Number: 1, Result: "SUCCESS", StartedAt: started(), Duration: time.Second},
	}}, repo)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	n, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)

	assert.Zero(t, n, "이미 같은 상태면 다시 쓰지 않는다")
	assert.Len(t, repo.rows, 1)
}

// 실행 중이던 빌드가 끝나면 상태가 갱신돼야 한다.
func TestSyncPipelineRuns_UpdatesRunningBuildOnCompletion(t *testing.T) {
	repo := newMemDeployments()
	reader := &stubBuildReader{builds: []port.CIBuild{
		{Number: 1, Building: true, StartedAt: started()},
	}}
	uc := NewSyncPipelineRuns(reader, repo)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.Equal(t, domain.DeploymentStatusRunning, repo.rows[runDeploymentID("pip_1", 1)].Status)

	reader.builds = []port.CIBuild{{Number: 1, Result: "SUCCESS", StartedAt: started(), Duration: time.Second}}
	_, err = uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)

	assert.Equal(t, domain.DeploymentStatusSuccess, repo.rows[runDeploymentID("pip_1", 1)].Status)
	assert.Len(t, repo.rows, 1)
}

// 실행 중인 빌드에 완료 시각을 넣으면 화면이 1970 년을 보여준다.
func TestSyncPipelineRuns_RunningBuildHasNoCompletedAt(t *testing.T) {
	repo := newMemDeployments()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		{Number: 3, Building: true, StartedAt: started()},
	}}, repo)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "pip_1", JobName: "app"})
	require.NoError(t, err)
	assert.Nil(t, repo.rows[runDeploymentID("pip_1", 3)].CompletedAt)
}

func TestSyncPipelineRuns_FailureStatuses(t *testing.T) {
	for _, result := range []string{"FAILURE", "ABORTED", "UNSTABLE"} {
		repo := newMemDeployments()
		uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
			{Number: 1, Result: result, StartedAt: started(), Duration: time.Second},
		}}, repo)
		_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "p", JobName: "app"})
		require.NoError(t, err)
		assert.Equal(t, domain.DeploymentStatusFailed, repo.rows[runDeploymentID("p", 1)].Status, result)
	}
}

// 어댑터가 정규화한 단계를 그대로 도메인 스텝으로 옮긴다.
// 이 함수는 CI 종류를 모른다 — OSS 를 하나 늘려도 바뀌지 않아야 한다.
func TestSyncPipelineRuns_MapsNormalizedStagesToSteps(t *testing.T) {
	repo := newMemDeployments()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{{
		Number: 1, Result: "SUCCESS", StartedAt: started(), Duration: time.Second,
		Stages: []port.CIStage{
			{Name: "Build", Status: port.CIStageSuccess, StartedAt: started()},
			{Name: "Deploy", Status: port.CIStageSkipped},
		},
	}}}, repo)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "p", JobName: "app"})
	require.NoError(t, err)

	steps := repo.rows[runDeploymentID("p", 1)].Steps
	require.Len(t, steps, 2)
	assert.Equal(t, "Build", steps[0].Name)
	assert.Equal(t, "success", steps[0].Status)
	assert.Equal(t, "ci_stage", steps[0].Kind)
	assert.Equal(t, "skipped", steps[1].Status, "건너뛴 단계를 실패로 옮기면 안 된다")
}

// 단계 정보가 없는 CI 도 있다. 빈 목록과 "모두 성공" 은 다르다.
func TestSyncPipelineRuns_NoStagesLeavesStepsEmpty(t *testing.T) {
	repo := newMemDeployments()
	uc := NewSyncPipelineRuns(&stubBuildReader{builds: []port.CIBuild{
		{Number: 1, Result: "SUCCESS", StartedAt: started(), Duration: time.Second},
	}}, repo)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "p", JobName: "app"})
	require.NoError(t, err)
	assert.Empty(t, repo.rows[runDeploymentID("p", 1)].Steps)
}

// CI 에 단계 플러그인을 나중에 깔면 이미 끝난 실행에 단계가 붙는다.
// 상태만 보고 건너뛰면 그 실행은 영원히 "실행 정보 없음" 으로 남는다.
func TestSyncPipelineRuns_FillsStagesThatAppearLater(t *testing.T) {
	repo := newMemDeployments()
	reader := &stubBuildReader{builds: []port.CIBuild{
		{Number: 1, Result: "SUCCESS", StartedAt: started(), Duration: time.Second},
	}}
	uc := NewSyncPipelineRuns(reader, repo)

	_, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "p", JobName: "app"})
	require.NoError(t, err)
	require.Empty(t, repo.rows[runDeploymentID("p", 1)].Steps)

	// 플러그인 설치 후: 같은 빌드에 단계가 생긴다.
	reader.builds[0].Stages = []port.CIStage{
		{Name: "Build", Status: port.CIStageSuccess},
		{Name: "Deploy", Status: port.CIStageSuccess},
	}
	n, err := uc.Execute(context.Background(), SyncPipelineRunsInput{PipelineID: "p", JobName: "app"})
	require.NoError(t, err)

	assert.Equal(t, 1, n)
	assert.Len(t, repo.rows[runDeploymentID("p", 1)].Steps, 2)
}
