package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
)

type refreshPipelineRepo struct {
	pipeline *domain.Pipeline
	getErr   error
	updated  []*domain.Pipeline
}

func (r *refreshPipelineRepo) Create(context.Context, *domain.Pipeline) error { return nil }
func (r *refreshPipelineRepo) GetByID(_ context.Context, id string) (*domain.Pipeline, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.pipeline == nil || r.pipeline.ID != id {
		return nil, errors.New("pipeline not found")
	}
	copied := *r.pipeline
	return &copied, nil
}
func (r *refreshPipelineRepo) List(context.Context, string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (r *refreshPipelineRepo) ListByStackID(context.Context, string) ([]*domain.Pipeline, error) {
	return nil, nil
}
func (r *refreshPipelineRepo) Update(_ context.Context, p *domain.Pipeline) error {
	copied := *p
	r.updated = append(r.updated, &copied)
	return nil
}
func (r *refreshPipelineRepo) Delete(context.Context, string) error { return nil }

func legacyPipeline() *domain.Pipeline {
	return &domain.Pipeline{
		ID: "pip_1", Name: "orders", StackID: "stk_1", TemplateID: "tmpl-1",
		Namespace: "apps", AppType: domain.AppTypeBackend, OrgID: "org-1", ClusterID: "c1",
		ExecutionMode: domain.ExecutionModeStackIntegrated,
		Stages:        []string{"Build", "Deploy"},
	}
}

// SAST 리포트 전에 만든 파이프라인은 정의를 다시 써야 결과가 붙는다. 파이프라인 레코드만으로
// 프로비저닝 입력을 다시 만들고, 돌아온 단계로 레코드를 맞춘다.
func TestRefreshPipelineDefinition_정의를_다시_쓰고_단계를_맞춘다(t *testing.T) {
	repo := &refreshPipelineRepo{pipeline: legacyPipeline()}
	prov := &fakeRepoProvisioner{out: &ProvisionPipelineRepositoryOutput{
		Stages:              []string{"Build", "SAST", "Deploy"},
		Warnings:            []string{"변수 X 는 마스킹 없이 등록합니다"},
		MissingVariables:    []string{"SONAR_TOKEN"},
		DefinitionRefreshed: true,
	}}
	uc := NewRefreshPipelineDefinition(repo, prov)

	out, err := uc.Execute(context.Background(), "pip_1")
	require.NoError(t, err)

	require.Len(t, prov.calls, 1)
	in := prov.calls[0]
	assert.True(t, in.RefreshDefinitionOnly, "정의 파일만 다시 쓴다")
	assert.Equal(t, "orders", in.AppName)
	assert.Equal(t, "stk_1", in.StackID)
	assert.Equal(t, "apps", in.Namespace)
	assert.Equal(t, domain.AppTypeBackend, in.AppType)
	assert.Equal(t, "tmpl-1", in.TemplateID)

	assert.Equal(t, []string{"Build", "SAST", "Deploy"}, out.Stages)
	assert.Equal(t, []string{"SONAR_TOKEN"}, out.MissingVariables)
	assert.Len(t, out.Warnings, 1)
	require.Len(t, repo.updated, 1, "단계가 바뀌었으니 레코드를 갱신한다")
	assert.Equal(t, []string{"Build", "SAST", "Deploy"}, repo.updated[0].Stages)
	assert.Equal(t, []string{"Build", "SAST", "Deploy"}, out.Pipeline.Stages)
}

// 스택에 묶이지 않은 파이프라인은 다시 쓸 정의가 없다 — 저장소도 CI 도 플랫폼 것이 아니다.
func TestRefreshPipelineDefinition_스택_없는_파이프라인은_거절한다(t *testing.T) {
	p := legacyPipeline()
	p.StackID = ""
	repo := &refreshPipelineRepo{pipeline: p}
	prov := &fakeRepoProvisioner{out: provisionedResult()}
	uc := NewRefreshPipelineDefinition(repo, prov)

	_, err := uc.Execute(context.Background(), "pip_1")
	require.ErrorIs(t, err, ErrPipelineNotStackBound)
	assert.Empty(t, prov.calls)
	assert.Empty(t, repo.updated)
}

func TestRefreshPipelineDefinition_프로비저닝_실패는_레코드를_건드리지_않는다(t *testing.T) {
	repo := &refreshPipelineRepo{pipeline: legacyPipeline()}
	prov := &fakeRepoProvisioner{err: errors.New("gitlab down")}
	uc := NewRefreshPipelineDefinition(repo, prov)

	_, err := uc.Execute(context.Background(), "pip_1")
	require.Error(t, err)
	assert.Empty(t, repo.updated)
}

// 단계가 돌아오지 않으면(옛 번들) 레코드의 단계는 그대로 둔다 — 비워 두면 화면이 템플릿으로 떨어진다.
func TestRefreshPipelineDefinition_단계가_없으면_레코드를_비우지_않는다(t *testing.T) {
	repo := &refreshPipelineRepo{pipeline: legacyPipeline()}
	prov := &fakeRepoProvisioner{out: &ProvisionPipelineRepositoryOutput{DefinitionRefreshed: true}}
	uc := NewRefreshPipelineDefinition(repo, prov)

	out, err := uc.Execute(context.Background(), "pip_1")
	require.NoError(t, err)
	assert.Equal(t, []string{"Build", "Deploy"}, out.Pipeline.Stages)
	assert.Empty(t, repo.updated)
}

func TestRefreshPipelineDefinition_없는_파이프라인(t *testing.T) {
	uc := NewRefreshPipelineDefinition(&refreshPipelineRepo{}, &fakeRepoProvisioner{out: provisionedResult()})
	_, err := uc.Execute(context.Background(), "nope")
	require.Error(t, err)
}

// 원본을 가져와 만든 파이프라인은 정의 갱신에도 원본을 넘긴다. 저장소가 없어 다시 만들면 처음처럼
// 원본을 다시 가져와야 한다 — 이미 있는 저장소에는 쓰지 않으므로 평소 갱신에는 영향이 없다.
func TestRefreshPipelineDefinition_원본을_가져온_파이프라인은_원본을_넘긴다(t *testing.T) {
	p := legacyPipeline()
	p.Source = egovBootSource()
	repo := &refreshPipelineRepo{pipeline: p}
	prov := &fakeRepoProvisioner{out: &ProvisionPipelineRepositoryOutput{DefinitionRefreshed: true}}

	_, err := NewRefreshPipelineDefinition(repo, prov).Execute(context.Background(), "pip_1")
	require.NoError(t, err)

	require.Len(t, prov.calls, 1)
	require.NotNil(t, prov.calls[0].Source)
	assert.Equal(t, egovBootSource().Commit, prov.calls[0].Source.Commit)
}
