package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

func egovTemplate() *domain.PipelineTemplate {
	return &domain.PipelineTemplate{
		ID:      "egovframe-boot-sample-v1",
		Name:    "eGovFrame Boot Sample",
		AppType: domain.AppTypeBackend,
		Stages:  []string{"Build", "Deploy"},
		Source:  egovBootSource(),
	}
}

// 템플릿의 원본은 저장소 프로비저닝까지 내려가야 한다.
func TestCreatePipeline_PassesTemplateSourceToProvisioner(t *testing.T) {
	result := provisionedResult()
	result.ImportedSource = egovBootSource()
	p := &fakeRepoProvisioner{out: result}
	uc := NewCreatePipeline(&mockCreatePipelineRepo{}, newMockCreateTemplateRepo(egovTemplate())).WithRepositoryProvisioner(p)

	in := provisionInput()
	in.TemplateID = "egovframe-boot-sample-v1"
	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	require.Len(t, p.calls, 1)
	require.NotNil(t, p.calls[0].Source)
	assert.Equal(t, *egovBootSource(), *p.calls[0].Source)
	require.NotNil(t, out.Pipeline.Source, "저장소가 어느 원본에서 시작했는지는 파이프라인에 남는다")
	assert.Equal(t, "c44e4bc8ea1e1b1e83e0aaab1037fbf04c761feb", out.Pipeline.Source.Commit)
}

// 이미 있던 저장소라 원본을 가져오지 않았으면 파이프라인에 원본을 기록하지 않는다.
func TestCreatePipeline_DoesNotRecordSourceThatWasNotImported(t *testing.T) {
	result := provisionedResult()
	result.ScaffoldSkipped = true
	p := &fakeRepoProvisioner{out: result}
	uc := NewCreatePipeline(&mockCreatePipelineRepo{}, newMockCreateTemplateRepo(egovTemplate())).WithRepositoryProvisioner(p)

	in := provisionInput()
	in.TemplateID = "egovframe-boot-sample-v1"
	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Nil(t, out.Pipeline.Source)
}

// 원본을 가져오는 템플릿은 가져올 저장소가 있어야 한다. 스택 없이 만들면 원본 없는 파이프라인이 생긴다.
func TestCreatePipeline_SourceTemplateRequiresStack(t *testing.T) {
	p := &fakeRepoProvisioner{out: provisionedResult()}
	repo := &mockCreatePipelineRepo{}
	uc := NewCreatePipeline(repo, newMockCreateTemplateRepo(egovTemplate())).WithRepositoryProvisioner(p)

	in := provisionInput()
	in.TemplateID = "egovframe-boot-sample-v1"
	in.StackID = ""
	in.ProvisionRepository = false
	_, err := uc.Execute(context.Background(), in)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTemplateSourceRequiresStack), "got %v", err)
	assert.Empty(t, p.calls)
	assert.Empty(t, repo.created)
}

func TestCreatePipeline_SourceTemplateRejectsEmergencyDirect(t *testing.T) {
	p := &fakeRepoProvisioner{out: provisionedResult()}
	uc := NewCreatePipeline(&mockCreatePipelineRepo{}, newMockCreateTemplateRepo(egovTemplate())).WithRepositoryProvisioner(p)

	in := provisionInput()
	in.TemplateID = "egovframe-boot-sample-v1"
	in.ProvisionRepository = false
	in.ExecutionMode = domain.ExecutionModeEmergencyDirect
	_, err := uc.Execute(context.Background(), in)
	assert.True(t, errors.Is(err, ErrTemplateSourceRequiresStack), "got %v", err)
}

// 자동 준비는 배선이 없으면 조용히 건너뛴다. 원본을 가져오는 템플릿은 건너뛰면 안 된다.
func TestCreatePipeline_SourceTemplateFailsWithoutProvisioner(t *testing.T) {
	repo := &mockCreatePipelineRepo{}
	uc := NewCreatePipeline(repo, newMockCreateTemplateRepo(egovTemplate()))

	in := provisionInput()
	in.TemplateID = "egovframe-boot-sample-v1"
	in.ProvisionRepository = false
	_, err := uc.Execute(context.Background(), in)
	require.Error(t, err)
	assert.Empty(t, repo.created)
}
