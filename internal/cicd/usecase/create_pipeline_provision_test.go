package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type fakeRepoProvisioner struct {
	out   *ProvisionPipelineRepositoryOutput
	err   error
	calls []ProvisionPipelineRepositoryInput
}

func (f *fakeRepoProvisioner) Execute(
	_ context.Context,
	in ProvisionPipelineRepositoryInput,
) (*ProvisionPipelineRepositoryOutput, error) {
	f.calls = append(f.calls, in)
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func provisionedResult() *ProvisionPipelineRepositoryOutput {
	return &ProvisionPipelineRepositoryOutput{
		Project:                &port.SCMProject{FullPath: "acme/myapp", HTTPCloneURL: "http://gl/acme/myapp.git"},
		RepoURL:                "http://gl/acme/myapp.git",
		ImageRepository:        "registry.test/acme/myapp",
		ArgoApplicationCreated: true,
	}
}

func newCreateWithProvisioner(p *fakeRepoProvisioner) (*CreatePipeline, *mockCreatePipelineRepo) {
	pipelineRepo := &mockCreatePipelineRepo{}
	uc := NewCreatePipeline(pipelineRepo, newMockCreateTemplateRepo())
	return uc.WithRepositoryProvisioner(p), pipelineRepo
}

func provisionInput() CreatePipelineInput {
	return CreatePipelineInput{
		Name:                "myapp",
		OrgID:               "org-1",
		ClusterID:           "c1",
		StackID:             "stk_1",
		Namespace:           "acme-prod",
		AppType:             domain.AppTypeBackend,
		ProvisionRepository: true,
	}
}

// 저장소를 만들었으면 그 주소가 파이프라인에 기록되어야 한다.
// 사용자가 입력한 Git URL 이 없어도 파이프라인이 자기 저장소를 안다.
func TestCreatePipeline_RecordsProvisionedRepository(t *testing.T) {
	p := &fakeRepoProvisioner{out: provisionedResult()}
	uc, repo := newCreateWithProvisioner(p)

	out, err := uc.Execute(context.Background(), provisionInput())
	require.NoError(t, err)

	require.Len(t, p.calls, 1)
	assert.Equal(t, "myapp", p.calls[0].AppName)
	assert.Equal(t, "stk_1", p.calls[0].StackID)
	assert.Equal(t, "acme-prod", p.calls[0].Namespace)

	assert.Equal(t, "http://gl/acme/myapp.git", out.Pipeline.GitRepoURL)
	require.Len(t, repo.created, 1)
	assert.Equal(t, "http://gl/acme/myapp.git", repo.created[0].GitRepoURL)
	assert.Equal(t, "registry.test/acme/myapp", out.Pipeline.EnvVars[envRegistryURL])
}

// 템플릿 id 는 스캐폴딩까지 내려가야 한다. 배포 매니페스트에 라벨로 실려야
// 클러스터에서 "이 템플릿으로 만든 앱들" 을 골라낼 수 있다.
func TestCreatePipeline_PassesTemplateIDToProvisioner(t *testing.T) {
	p := &fakeRepoProvisioner{out: provisionedResult()}
	templates := newMockCreateTemplateRepo(&domain.PipelineTemplate{ID: "web-backend-v1", Name: "User Custom Pipeline"})
	uc := NewCreatePipeline(&mockCreatePipelineRepo{}, templates).WithRepositoryProvisioner(p)

	in := provisionInput()
	in.TemplateID = "web-backend-v1"

	_, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	require.Len(t, p.calls, 1)
	assert.Equal(t, "web-backend-v1", p.calls[0].TemplateID)
}

// 스택에 묶인 파이프라인은 플래그가 없어도 준비한다.
//
// 이 테스트는 원래 "요청하지 않으면 만들지 않는다" 를 고정하고 있었다. 그런데
// 요청하는 쪽이 없었다 — 프론트는 provision_repository 를 한 번도 보내지 않아
// UI 로 만든 파이프라인에는 저장소도, Jenkinsfile 도, Jenkins job 도, Argo CD
// Application 도 없었다. 통합모드는 그것들이 있다는 전제 위에 서 있으므로,
// 준비 없이 만든 파이프라인은 배포를 눌러도 넘길 job 이 없다.
func TestCreatePipeline_ProvisionsStackPipelineWithoutExplicitRequest(t *testing.T) {
	p := &fakeRepoProvisioner{out: provisionedResult()}
	uc, _ := newCreateWithProvisioner(p)

	in := provisionInput()
	in.ProvisionRepository = false

	_, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, p.calls, 1, "스택에 묶인 파이프라인은 저장소·CI job 을 준비한다")
}

// 스택이 없으면 준비할 대상 자체가 없다. 기존 경로 그대로 둔다.
func TestCreatePipeline_SkipsProvisioningWithoutStack(t *testing.T) {
	p := &fakeRepoProvisioner{out: provisionedResult()}
	uc, _ := newCreateWithProvisioner(p)

	in := provisionInput()
	in.ProvisionRepository = false
	in.StackID = ""

	_, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Empty(t, p.calls)
}

// 긴급 직접 배포는 스택 컴포넌트를 쓸 수 없을 때의 경로다. 그 상황에서 저장소
// 생성을 시도하면 쓰지 못하는 도구에 붙으려다 파이프라인 생성 자체가 막힌다.
func TestCreatePipeline_EmergencyDirectSkipsProvisioning(t *testing.T) {
	p := &fakeRepoProvisioner{out: provisionedResult()}
	uc, _ := newCreateWithProvisioner(p)

	in := provisionInput()
	in.ProvisionRepository = false
	in.ExecutionMode = domain.ExecutionModeEmergencyDirect

	_, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Empty(t, p.calls)
}

// 저장소 생성은 스택이 있어야 가능하다.
func TestCreatePipeline_RequiresStackForProvisioning(t *testing.T) {
	p := &fakeRepoProvisioner{out: provisionedResult()}
	uc, _ := newCreateWithProvisioner(p)

	in := provisionInput()
	in.StackID = ""

	_, err := uc.Execute(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stack")
	assert.Empty(t, p.calls)
}

// 프로비저닝이 실패하면 파이프라인을 만들지 않는다 — 저장소 없는 파이프라인은
// 배포할 것이 없어 의미가 없고, 그 상태로 남으면 사용자가 원인을 알기 어렵다.
func TestCreatePipeline_FailsWhenProvisioningFails(t *testing.T) {
	p := &fakeRepoProvisioner{err: errors.New("gitlab not ready")}
	uc, repo := newCreateWithProvisioner(p)

	_, err := uc.Execute(context.Background(), provisionInput())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gitlab not ready")
	assert.Empty(t, repo.created, "실패 시 파이프라인 레코드를 남기지 않는다")
}

// 사람이 채워야 할 변수와 경고는 응답으로 올라가야 UI 가 보여줄 수 있다.
func TestCreatePipeline_SurfacesProvisioningFollowUps(t *testing.T) {
	result := provisionedResult()
	result.MissingVariables = []string{"HARBOR_USERNAME"}
	result.Warnings = []string{"Argo CD Application 적용 실패"}
	result.ArgoApplicationCreated = false

	p := &fakeRepoProvisioner{out: result}
	uc, _ := newCreateWithProvisioner(p)

	out, err := uc.Execute(context.Background(), provisionInput())
	require.NoError(t, err)

	assert.Equal(t, []string{"HARBOR_USERNAME"}, out.MissingVariables)
	assert.NotEmpty(t, out.Warnings)
	assert.False(t, out.ArgoApplicationCreated)
	assert.Equal(t, "acme/myapp", out.RepositoryPath)
}

// 프로비저너가 배선되지 않았는데 요청이 오면 조용히 넘어가지 않는다.
func TestCreatePipeline_FailsWhenProvisioningRequestedButUnwired(t *testing.T) {
	uc := NewCreatePipeline(&mockCreatePipelineRepo{}, newMockCreateTemplateRepo())

	_, err := uc.Execute(context.Background(), provisionInput())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "프로비저닝")
}
