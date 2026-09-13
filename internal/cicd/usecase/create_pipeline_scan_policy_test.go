package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

type recordingPolicyPipelinePublisher struct {
	pipelines []*domain.Pipeline
	push      *ScanPolicyPush
}

func (r *recordingPolicyPipelinePublisher) PublishToPipeline(_ context.Context, p *domain.Pipeline) *ScanPolicyPush {
	r.pipelines = append(r.pipelines, p)
	return r.push
}

func scanProvisionedResult() *ProvisionPipelineRepositoryOutput {
	out := provisionedResult()
	out.Stages = []string{"Build", "ImageScan", "Deploy"}
	return out
}

// 새 스캔 파이프라인에는 스택에 저장된 정책을 싣는다. 싣지 않으면 스크립트 기본값으로
// 돌아, 운영자가 HIGH 를 막아 둔 스택에서 새 파이프라인만 HIGH 가 통과한다.
func TestCreatePipeline_PublishesStackScanPolicyToNewPipeline(t *testing.T) {
	uc, _ := newCreateWithProvisioner(&fakeRepoProvisioner{out: scanProvisionedResult()})
	pub := &recordingPolicyPipelinePublisher{push: &ScanPolicyPush{Status: ScanPolicyPushApplied}}

	out, err := uc.WithScanPolicyPublisher(pub).Execute(context.Background(), provisionInput())
	require.NoError(t, err)

	require.Len(t, pub.pipelines, 1)
	assert.Equal(t, out.Pipeline.ID, pub.pipelines[0].ID)
	assert.Equal(t, []string{"Build", "ImageScan", "Deploy"}, pub.pipelines[0].Stages,
		"단계가 기록된 뒤에 실어야 스캔 파이프라인인지 알 수 있다")
	assert.Empty(t, out.Warnings)
}

// 싣지 못해도 파이프라인 생성은 실패시키지 않는다. 대신 알린다 — 조용히 넘기면
// 운영자는 새 파이프라인도 스택 정책으로 막히는 줄 안다.
func TestCreatePipeline_WarnsWhenScanPolicyPushFails(t *testing.T) {
	uc, repo := newCreateWithProvisioner(&fakeRepoProvisioner{out: scanProvisionedResult()})
	pub := &recordingPolicyPipelinePublisher{push: &ScanPolicyPush{Status: ScanPolicyPushFailed, Error: "403 forbidden"}}

	out, err := uc.WithScanPolicyPublisher(pub).Execute(context.Background(), provisionInput())
	require.NoError(t, err)
	require.Len(t, repo.created, 1)

	require.NotEmpty(t, out.Warnings)
	assert.Contains(t, out.Warnings[len(out.Warnings)-1], "403 forbidden")
}
