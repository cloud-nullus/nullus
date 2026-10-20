package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

type fakeSecurityReader struct {
	snapshots []*domain.PipelineSecuritySnapshot
	err       error
	gotOrg    string
	gotLimit  int
}

func (f *fakeSecurityReader) ListPipelineSecurity(_ context.Context, orgID string, limit int) ([]*domain.PipelineSecuritySnapshot, error) {
	f.gotOrg, f.gotLimit = orgID, limit
	if f.err != nil {
		return nil, f.err
	}
	if len(f.snapshots) > limit {
		return f.snapshots[:limit], nil
	}
	return f.snapshots, nil
}

// 결과 행은 실행 ID 만 갖는다. 셀에 "#42" 를 달려면 동기화가 만든 실행 ID 규칙을 거꾸로 읽어야 한다 —
// 그 규칙은 이 패키지(runDeploymentID)가 안다.
func TestListPipelineSecurity_FillsRunVersions(t *testing.T) {
	reader := &fakeSecurityReader{snapshots: []*domain.PipelineSecuritySnapshot{{
		PipelineID:     "pip_1",
		ImageScan:      &domain.ImageScanResult{DeploymentID: runDeploymentID("pip_1", 41)},
		SAST:           &domain.SASTResult{DeploymentID: runDeploymentID("pip_1", 40)},
		DependencyScan: &domain.DependencyScanResult{DeploymentID: runDeploymentID("pip_1", 42)},
		SecretScan:     &domain.SecretScanResult{DeploymentID: runDeploymentID("pip_1", 42)},
	}, {
		PipelineID: "pip_2",
		// 플랫폼이 직접 실행한 배포 ID 는 실행 번호 규칙을 따르지 않는다 — 지어내지 않는다.
		ImageScan: &domain.ImageScanResult{DeploymentID: "dep_manual"},
	}}}

	out, err := NewListPipelineSecurity(reader).Execute(context.Background(), " org-1 ")
	require.NoError(t, err)

	assert.Equal(t, "org-1", reader.gotOrg)
	assert.False(t, out.Truncated)
	require.Len(t, out.Pipelines, 2)
	assert.Equal(t, map[string]string{
		runDeploymentID("pip_1", 41): "#41",
		runDeploymentID("pip_1", 40): "#40",
		runDeploymentID("pip_1", 42): "#42",
	}, out.Pipelines[0].RunVersions)
	assert.Empty(t, out.Pipelines[1].RunVersions)
}

// 동기화 대상과 같은 상한(1000)을 두고, 넘으면 잘랐다고 알린다 — 화면이 "전부" 로 읽지 않게.
func TestListPipelineSecurity_TruncatesAtLimit(t *testing.T) {
	snaps := make([]*domain.PipelineSecuritySnapshot, 0, MaxSecurityPipelines+5)
	for i := 0; i < MaxSecurityPipelines+5; i++ {
		snaps = append(snaps, &domain.PipelineSecuritySnapshot{PipelineID: fmt.Sprintf("pip_%d", i)})
	}
	reader := &fakeSecurityReader{snapshots: snaps}

	out, err := NewListPipelineSecurity(reader).Execute(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, MaxSecurityPipelines+1, reader.gotLimit, "하나 더 읽어 넘쳤는지 안다")
	assert.True(t, out.Truncated)
	assert.Len(t, out.Pipelines, MaxSecurityPipelines)

	reader.snapshots = snaps[:MaxSecurityPipelines]
	out, err = NewListPipelineSecurity(reader).Execute(context.Background(), "org-1")
	require.NoError(t, err)
	assert.False(t, out.Truncated, "딱 상한이면 자른 것이 아니다")
}

func TestListPipelineSecurity_RequiresOrg(t *testing.T) {
	_, err := NewListPipelineSecurity(&fakeSecurityReader{}).Execute(context.Background(), "  ")
	assert.Error(t, err)
}

func TestListPipelineSecurity_PropagatesReaderError(t *testing.T) {
	_, err := NewListPipelineSecurity(&fakeSecurityReader{err: errors.New("db down")}).Execute(context.Background(), "org-1")
	assert.Error(t, err)

	var nilUC *ListPipelineSecurity
	_, err = nilUC.Execute(context.Background(), "org-1")
	assert.Error(t, err, "배선되지 않았으면 빈 목록이 아니라 오류다 — 빈 목록은 앱이 없다로 읽힌다")
}

// 단계 이름은 CI 마다 표기가 다르다. 대시보드의 "미적용" 판정은 동기화와 같은 단계 키로 본다.
func TestListPipelineSecurity_MarksSecurityStages(t *testing.T) {
	reader := &fakeSecurityReader{snapshots: []*domain.PipelineSecuritySnapshot{
		{PipelineID: "jenkins", Stages: []string{"SecretScan", "DependencyScan", "Build", "SAST", "ImageScan", "Deploy"}},
		{PipelineID: "gitlab", Stages: []string{"secret-scan", "build", "image-scan", "deploy"}},
		{PipelineID: "legacy"},
	}}
	out, err := NewListPipelineSecurity(reader).Execute(context.Background(), "org-1")
	require.NoError(t, err)

	j, g, l := out.Pipelines[0], out.Pipelines[1], out.Pipelines[2]
	assert.True(t, j.HasSecretScan && j.HasDependencyScan && j.HasSAST && j.HasImageScan)
	assert.True(t, g.HasSecretScan)
	assert.False(t, g.HasDependencyScan)
	assert.False(t, g.HasSAST)
	assert.True(t, g.HasImageScan)
	assert.False(t, l.HasSecretScan || l.HasDependencyScan || l.HasSAST || l.HasImageScan, "단계를 모르면 아무것도 없다고 표시한다 — 판정은 대시보드가 한다")
}

func TestRunVersionOf(t *testing.T) {
	assert.Equal(t, "#7", runVersionOf("pip_1", runDeploymentID("pip_1", 7)))
	assert.Equal(t, "", runVersionOf("pip_1", runDeploymentID("pip_2", 7)), "다른 파이프라인의 실행")
	assert.Equal(t, "", runVersionOf("pip_1", "dep_ci_pip_1_x"))
	assert.Equal(t, "", runVersionOf("pip_1", ""))
}
