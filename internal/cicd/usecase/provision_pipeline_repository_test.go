package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type fakeBundleFactory struct {
	bundle *port.SCMBundle
	err    error
	asked  []string
}

func (f *fakeBundleFactory) For(_ context.Context, stackID string) (*port.SCMBundle, error) {
	f.asked = append(f.asked, stackID)
	if f.err != nil {
		return nil, f.err
	}
	return f.bundle, nil
}

type fakeApplier struct {
	applied []string
	err     error
}

func (f *fakeApplier) Apply(_ context.Context, _ []byte, manifests []string) error {
	if f.err != nil {
		return f.err
	}
	f.applied = append(f.applied, manifests...)
	return nil
}

func (f *fakeApplier) ApplyWithTracking(ctx context.Context, kc []byte, manifests []string, _ string, _ ...int) error {
	return f.Apply(ctx, kc, manifests)
}

type fakeKubeconfigProvider struct {
	err error
	// asked 는 어느 클러스터의 kubeconfig 를 요청했는지다. 클러스터를 잘못 고르면
	// 호출은 성공하고 결과만 비어 보이므로, 어디를 봤는지를 직접 확인해야 한다.
	asked []string
}

func (f *fakeKubeconfigProvider) GetKubeconfig(_ context.Context, clusterID string) ([]byte, error) {
	f.asked = append(f.asked, clusterID)
	if f.err != nil {
		return nil, f.err
	}
	return []byte("apiVersion: v1\nkind: Config\n"), nil
}

func newBundle(scm *fakeSCM, pipe *fakePipelineConfig, res *fakeResolver) *port.SCMBundle {
	return &port.SCMBundle{
		Provisioner: scm, Pipeline: pipe, Registry: res,
		GroupPath: "acme", CDNamespace: "devsecops", ClusterID: "c1",
	}
}

func repoInput() ProvisionPipelineRepositoryInput {
	return ProvisionPipelineRepositoryInput{
		AppName:   "myapp",
		StackID:   "stk_1",
		Namespace: "acme-prod",
		Port:      8080,
		Replicas:  2,
	}
}

func newRepoUC(scm *fakeSCM, pipe *fakePipelineConfig, res *fakeResolver, applier *fakeApplier) *ProvisionPipelineRepository {
	return NewProvisionPipelineRepository(
		&fakeBundleFactory{bundle: newBundle(scm, pipe, res)},
		applier,
		&fakeKubeconfigProvider{},
	)
}

func TestProvisionPipelineRepository_CreatesCommonThenAppProject(t *testing.T) {
	scm, pipe, res, applier := newFakeSCM(), newFakePipelineConfig(), gitlabResolver(), &fakeApplier{}
	uc := newRepoUC(scm, pipe, res, applier)

	out, err := uc.Execute(context.Background(), repoInput())
	require.NoError(t, err)

	// common 이 먼저, 그다음 앱 프로젝트.
	require.Len(t, scm.projects, 2)
	assert.Equal(t, DefaultCommonProjectPath, scm.projects[0].Path)
	assert.Equal(t, "myapp", scm.projects[1].Path)

	assert.Equal(t, "acme/myapp", out.Project.FullPath)
	assert.NotEmpty(t, out.CommonProject.FullPath)
}

// Argo CD Application 이 없으면 CI 가 태그를 갱신해도 배포가 일어나지 않는다.
func TestProvisionPipelineRepository_AppliesArgoCDApplication(t *testing.T) {
	scm, pipe, res, applier := newFakeSCM(), newFakePipelineConfig(), gitlabResolver(), &fakeApplier{}
	uc := newRepoUC(scm, pipe, res, applier)

	out, err := uc.Execute(context.Background(), repoInput())
	require.NoError(t, err)

	// 저장소 자격증명 Secret 과 Application 을 함께 적용한다 —
	// 자격증명이 없으면 private 저장소 동기화가 실패한다.
	require.Len(t, applier.applied, 4)
	joined := strings.Join(applier.applied, "\n---\n")
	assert.Contains(t, joined, "argocd.argoproj.io/secret-type: repository")
	assert.Contains(t, joined, "kubernetes.io/dockerconfigjson")
	assert.Contains(t, joined, "name: nullus-registry-myapp", "pull 자격증명은 앱마다 따로다")
	assert.Contains(t, joined, "kind: Namespace")

	manifest := applier.applied[3]
	assert.Contains(t, manifest, "kind: Application")
	assert.Contains(t, manifest, "name: myapp")
	assert.Contains(t, manifest, "namespace: devsecops", "Argo CD 네임스페이스에 만들어야 인식된다")
	assert.Contains(t, manifest, "path: deploy")
	assert.Contains(t, manifest, "namespace: acme-prod")
	assert.True(t, out.ArgoApplicationCreated)
}

// Application 의 repoURL 은 앱 저장소를 가리켜야 한다.
func TestProvisionPipelineRepository_ArgoApplicationPointsAtAppRepository(t *testing.T) {
	scm, pipe, res, applier := newFakeSCM(), newFakePipelineConfig(), gitlabResolver(), &fakeApplier{}
	uc := newRepoUC(scm, pipe, res, applier)

	out, err := uc.Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.Contains(t, strings.Join(applier.applied, "\n"), out.Project.HTTPCloneURL)
}

func TestProvisionPipelineRepository_ReturnsImageRepositoryForPipelineRecord(t *testing.T) {
	scm, pipe, res, applier := newFakeSCM(), newFakePipelineConfig(), harborResolver(), &fakeApplier{}
	uc := newRepoUC(scm, pipe, res, applier)

	out, err := uc.Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.Equal(t, "harbor.test/acme/myapp", out.ImageRepository)
	assert.Equal(t, out.Project.HTTPCloneURL, out.RepoURL)
}

// 사람이 채워야 할 변수는 위로 전달되어야 UI 가 보여줄 수 있다.
func TestProvisionPipelineRepository_SurfacesMissingVariables(t *testing.T) {
	scm, pipe, res, applier := newFakeSCM(), newFakePipelineConfig(), harborResolver(), &fakeApplier{}
	uc := newRepoUC(scm, pipe, res, applier)

	out, err := uc.Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"HARBOR_USERNAME", "HARBOR_PASSWORD"}, out.MissingVariables)
}

// Application 적용 실패로 저장소 프로비저닝을 되돌리지 않는다.
// 저장소는 이미 만들어졌고, 되돌리면 재실행 시 상태를 알 수 없다.
func TestProvisionPipelineRepository_ContinuesWhenArgoApplyFails(t *testing.T) {
	scm, pipe, res := newFakeSCM(), newFakePipelineConfig(), gitlabResolver()
	applier := &fakeApplier{err: errors.New("argocd crd missing")}
	uc := newRepoUC(scm, pipe, res, applier)

	out, err := uc.Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.False(t, out.ArgoApplicationCreated)
	require.NotEmpty(t, out.Warnings)
	assert.True(t, strings.Contains(strings.Join(out.Warnings, " "), "argocd crd missing"))
	assert.NotNil(t, out.Project, "저장소는 그대로 남아야 한다")
}

func TestProvisionPipelineRepository_FailsWhenBundleUnavailable(t *testing.T) {
	uc := NewProvisionPipelineRepository(
		&fakeBundleFactory{err: errors.New("gitlab not installed")},
		&fakeApplier{},
		&fakeKubeconfigProvider{},
	)

	_, err := uc.Execute(context.Background(), repoInput())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gitlab not installed")
}

func TestProvisionPipelineRepository_RequiresAppNameAndStack(t *testing.T) {
	scm, pipe, res, applier := newFakeSCM(), newFakePipelineConfig(), gitlabResolver(), &fakeApplier{}
	uc := newRepoUC(scm, pipe, res, applier)

	in := repoInput()
	in.AppName = ""
	_, err := uc.Execute(context.Background(), in)
	require.Error(t, err)

	in = repoInput()
	in.StackID = ""
	_, err = uc.Execute(context.Background(), in)
	require.Error(t, err)
}

func TestProvisionPipelineRepository_AsksFactoryForRequestedStack(t *testing.T) {
	factory := &fakeBundleFactory{bundle: newBundle(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())}
	uc := NewProvisionPipelineRepository(factory, &fakeApplier{}, &fakeKubeconfigProvider{})

	_, err := uc.Execute(context.Background(), repoInput())
	require.NoError(t, err)
	assert.Equal(t, []string{"stk_1"}, factory.asked)
}

type fakeOrgMembers struct {
	org   string
	email string
	err   error
}

func (f *fakeOrgMembers) EnsureOrgMember(_ context.Context, org, email string) error {
	f.org, f.email = org, email
	return f.err
}

// 아직 로그인하지 않아 SCM 계정이 없는 것은 실패가 아니라 "아직" 이다.
// 프로비저닝을 멈추지 않고, 무엇을 하면 되는지 경고로 남긴다.
func TestEnsureRequesterIsOrgMember_ExplainsMissingAccount(t *testing.T) {
	members := &fakeOrgMembers{err: fmt.Errorf("%w: dev@acme.io", port.ErrSCMUserNotFound)}
	bundle := &port.SCMBundle{OrgMembers: members}

	warning := ensureRequesterIsOrgMember(context.Background(), bundle, "nullus", "dev@acme.io")

	assert.Contains(t, warning, "dev@acme.io")
	assert.Contains(t, warning, "로그인")
}

func TestEnsureRequesterIsOrgMember_PassesOrgAndEmail(t *testing.T) {
	members := &fakeOrgMembers{}
	bundle := &port.SCMBundle{OrgMembers: members}

	warning := ensureRequesterIsOrgMember(context.Background(), bundle, "nullus", "dev@acme.io")

	assert.Equal(t, "", warning)
	assert.Equal(t, "nullus", members.org)
	assert.Equal(t, "dev@acme.io", members.email)
}

// 이메일을 모르는 경로(자동화 호출 등)까지 경고를 남길 이유는 없다.
func TestEnsureRequesterIsOrgMember_SilentWithoutEmail(t *testing.T) {
	members := &fakeOrgMembers{}
	bundle := &port.SCMBundle{OrgMembers: members}

	assert.Equal(t, "", ensureRequesterIsOrgMember(context.Background(), bundle, "nullus", ""))
	assert.Equal(t, "", members.org)
}

// 정의 갱신은 Argo CD Application 을 다시 적용하지 않는다 — 사용자가 손본 Application
// 을 기본값으로 되돌릴 이유가 없고, 갱신의 목적은 CI 정의와 변수다.
func TestProvisionPipelineRepository_RefreshDefinitionOnly_는_Argo_Application_을_건드리지_않는다(t *testing.T) {
	scm := newFakeSCM()
	scm.projectExists = true
	applier := &fakeApplier{}
	uc := NewProvisionPipelineRepository(
		&fakeBundleFactory{bundle: newBundle(scm, newFakePipelineConfig(), harborResolver())},
		applier,
		&fakeKubeconfigProvider{},
	)
	in := repoInput()
	in.RefreshDefinitionOnly = true

	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.True(t, out.DefinitionRefreshed)
	assert.False(t, out.ArgoApplicationCreated)
	assert.Empty(t, applier.applied, "Application·자격증명 매니페스트를 다시 적용하지 않는다")
	assert.NotEmpty(t, out.Stages)
}

func TestProvisionPipelineRepository_ImportsTemplateSource(t *testing.T) {
	scm, pipe, res, applier := newFakeSCM(), newFakePipelineConfig(), gitlabResolver(), &fakeApplier{}
	archive := egovBootArchive()
	uc := newRepoUC(scm, pipe, res, applier).WithSourceArchive(archive)

	in := repoInput()
	in.Source = egovBootSource()
	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Len(t, archive.calls, 1)
	require.NotNil(t, out.ImportedSource)
	assert.Equal(t, egovBootSource().Commit, out.ImportedSource.Commit)
}

type fakeCDRefresher struct {
	calls []string
	err   error
}

func (f *fakeCDRefresher) EnsureFirstComparison(_ context.Context, _ []byte, namespace, name string) error {
	f.calls = append(f.calls, namespace+"/"+name)
	return f.err
}

type fakeCDWebhooks struct {
	calls []string
	err   error
	// errs 가 있으면 호출마다 차례로 돌려준다(다 쓰면 err).
	errs []error
}

func (f *fakeCDWebhooks) EnsureWebhook(_ context.Context, projectID, targetURL, _ string) error {
	f.calls = append(f.calls, projectID+" -> "+targetURL)
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		return err
	}
	return f.err
}

// repoUCWithBundle 은 뒤에서 하는 재시도를 기다리지 않고 바로 돌리는 유스케이스를 만든다.
func repoUCWithBundle(bundle *port.SCMBundle, applier *fakeApplier) *ProvisionPipelineRepository {
	uc := NewProvisionPipelineRepository(&fakeBundleFactory{bundle: bundle}, applier, &fakeKubeconfigProvider{})
	uc.runAsync = func(f func()) { f() }
	uc.cdWebhookRetryInterval = 0
	return uc
}

// 자격증명 Secret 과 Application 을 한 번에 적용하면 Argo CD 가 Secret 을 보기 전에
// 첫 비교를 돌려 "authentication required" 로 끝날 수 있다(2026-10-19 실측). 그대로
// 두면 다음 정기 확인(3분)까지 동기화되지 않으므로, 만든 직후 첫 비교를 확인한다.
func TestProvisionPipelineRepository_EnsuresFirstArgoComparison(t *testing.T) {
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())
	refresher := &fakeCDRefresher{}
	bundle.CDRefresher = refresher

	out, err := repoUCWithBundle(bundle, &fakeApplier{}).Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.Equal(t, []string{"devsecops/myapp"}, refresher.calls, "Application 은 CD 네임스페이스에 있다")
	assert.True(t, out.ArgoApplicationCreated)
	for _, w := range out.Warnings {
		assert.NotContains(t, w, "Argo CD 가 저장소")
	}
}

// 다시 읽게 해도 못 읽으면 진짜 문제다(자격증명 · 경로). 생성은 마치되 이유를 알린다.
func TestProvisionPipelineRepository_WarnsWhenArgoCannotReadRepository(t *testing.T) {
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())
	bundle.CDRefresher = &fakeCDRefresher{err: errors.New("authentication required")}

	out, err := repoUCWithBundle(bundle, &fakeApplier{}).Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.True(t, out.ArgoApplicationCreated)
	joined := strings.Join(out.Warnings, "\n")
	assert.Contains(t, joined, "Argo CD 가 저장소를 아직 읽지 못했습니다")
	assert.Contains(t, joined, "authentication required")
}

// CD 도구는 폴링(Argo CD 기본 3분)으로만 새 커밋을 안다. CI 가 배포 태그를 되커밋해도
// 최대 3분 늦게 배포된다 — 저장소가 커밋을 바로 알리게 webhook 을 건다.
func TestProvisionPipelineRepository_RegistersCDWebhook(t *testing.T) {
	scm := newFakeSCM()
	bundle := newBundle(scm, newFakePipelineConfig(), gitlabResolver())
	hooks := &fakeCDWebhooks{}
	bundle.CDWebhooks = hooks
	bundle.CDWebhookURL = "http://argo-cd-argocd-server.devsecops.svc.cluster.local/api/webhook"

	out, err := repoUCWithBundle(bundle, &fakeApplier{}).Execute(context.Background(), repoInput())
	require.NoError(t, err)

	require.Len(t, hooks.calls, 1)
	assert.Equal(t, out.Project.ID+" -> http://argo-cd-argocd-server.devsecops.svc.cluster.local/api/webhook", hooks.calls[0])
}

// webhook 을 못 걸어도 배포는 된다 — 늦을 뿐이다. 생성 경고로 돌려주면 화면이 자동 실행을
// 멈추고, 권한처럼 되풀이되는 실패면 그 스택의 모든 생성이 멈춘다. 서버 로그에만 남긴다.
func TestProvisionPipelineRepository_CDWebhookFailureDoesNotBlockAutoRun(t *testing.T) {
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())
	bundle.CDWebhooks = &fakeCDWebhooks{err: errors.New("403 forbidden")}
	bundle.CDWebhookURL = "http://argo/api/webhook"

	out, err := repoUCWithBundle(bundle, &fakeApplier{}).Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.True(t, out.ArgoApplicationCreated)
	assert.NotContains(t, strings.Join(out.Warnings, "\n"), "webhook")
	assert.NotContains(t, strings.Join(out.Warnings, "\n"), "정기 확인")
}

// 정의 갱신은 Application 은 건드리지 않지만 webhook 은 다시 보장한다 — 이 기능 전에 만든
// 파이프라인이 정의 갱신으로 webhook 을 얻는다.
func TestProvisionPipelineRepository_RefreshDefinitionEnsuresCDWebhook(t *testing.T) {
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())
	hooks := &fakeCDWebhooks{}
	bundle.CDWebhooks, bundle.CDWebhookURL = hooks, "http://argo/api/webhook"
	applier := &fakeApplier{}
	input := repoInput()
	input.RefreshDefinitionOnly = true

	_, err := repoUCWithBundle(bundle, applier).Execute(context.Background(), input)
	require.NoError(t, err)

	assert.Len(t, hooks.calls, 1)
	assert.Empty(t, applier.applied, "Application 은 다시 적용하지 않는다")
}

// Application 을 만들지 못했으면 확인할 것도, 알릴 곳도 없다.
func TestProvisionPipelineRepository_SkipsCDFollowUpsWhenArgoApplyFails(t *testing.T) {
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())
	refresher, hooks := &fakeCDRefresher{}, &fakeCDWebhooks{}
	bundle.CDRefresher, bundle.CDWebhooks, bundle.CDWebhookURL = refresher, hooks, "http://argo/api/webhook"

	out, err := repoUCWithBundle(bundle, &fakeApplier{err: errors.New("argocd crd missing")}).Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.False(t, out.ArgoApplicationCreated)
	assert.Empty(t, refresher.calls)
	assert.Empty(t, hooks.calls)
}

// GitLab 은 애플리케이션 설정을 프로세스마다 최대 1분 캐시한다. 허용 목록을 막 바꾼 직후의
// 생성은 옛 설정으로 검사돼 거부된다(2026-10-19 실측 "Invalid url given"). 생성 요청을
// 붙잡지 않고 뒤에서 다시 건다 — CI 의 배포 되커밋은 몇 분 뒤라 그 전에 걸린다.
func TestProvisionPipelineRepository_RetriesCDWebhookInBackgroundWhilePending(t *testing.T) {
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())
	hooks := &fakeCDWebhooks{errs: []error{port.ErrWebhookTargetPending, port.ErrWebhookTargetPending}}
	bundle.CDWebhooks, bundle.CDWebhookURL = hooks, "http://argo/api/webhook"

	out, err := repoUCWithBundle(bundle, &fakeApplier{}).Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.Len(t, hooks.calls, 3, "두 번 거부된 뒤 세 번째에 걸린다")
	assert.NotContains(t, strings.Join(out.Warnings, "\n"), "정기 확인", "곧 걸릴 것이므로 경고하지 않는다")
}

// 뒤에서 하는 재시도에도 끝이 있다.
func TestProvisionPipelineRepository_CDWebhookRetryIsBounded(t *testing.T) {
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), gitlabResolver())
	hooks := &fakeCDWebhooks{err: port.ErrWebhookTargetPending}
	bundle.CDWebhooks, bundle.CDWebhookURL = hooks, "http://argo/api/webhook"

	_, err := repoUCWithBundle(bundle, &fakeApplier{}).Execute(context.Background(), repoInput())
	require.NoError(t, err)

	assert.Len(t, hooks.calls, 1+cdWebhookRetryAttempts)
}
