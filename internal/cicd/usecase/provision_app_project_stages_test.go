package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 스캐폴딩이 실제로 만든 단계를 출력에 싣는다.
//
// 스캔 단계는 파이프라인 단위 선택이라 템플릿이 알 수 없다. 여기서 싣지 않으면
// 화면은 템플릿의 기본 단계만 그리고, 돌고 있는 스캔 단계를 보여주지 못한다.
func TestProvisionAppProject_ReportsRenderedStages(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	in := appInput()
	in.ImageScannerEndpoint = "http://trivy.acme-prod.svc.cluster.local:4954"
	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Equal(t, []string{"Build", "ImageScan", "Deploy"}, out.Stages)
}

func TestProvisionAppProject_ReportsBaseStagesWithoutScanner(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	uc := NewProvisionAppProject(scm, pipe, res)

	out, err := uc.Execute(context.Background(), appInput())
	require.NoError(t, err)

	assert.Equal(t, []string{"Build", "Deploy"}, out.Stages)
}

// 이미 있던 저장소는 스캐폴딩을 쓰지 않는다 — 그 안의 파이프라인 파일은
// 우리가 만든 것이 아니므로 단계를 안다고 말하면 안 된다. 비워 두면 화면이
// 템플릿으로 떨어진다.
func TestProvisionAppProject_NoStagesWhenScaffoldSkipped(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), harborResolver()
	scm.projectExists = true
	uc := NewProvisionAppProject(scm, pipe, res)

	in := appInput()
	in.ImageScannerEndpoint = "http://trivy.acme-prod.svc.cluster.local:4954"
	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	require.True(t, out.ScaffoldSkipped)
	assert.Empty(t, out.Stages)
}
