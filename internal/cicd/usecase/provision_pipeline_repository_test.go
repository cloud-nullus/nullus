package usecase

import (
	"context"
	"errors"
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

type fakeKubeconfigProvider struct{ err error }

func (f *fakeKubeconfigProvider) GetKubeconfig(_ context.Context, _ string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte("apiVersion: v1\nkind: Config\n"), nil
}

func newBundle(scm *fakeSCM, pipe *fakePipelineConfig, res *fakeResolver) *port.SCMBundle {
	return &port.SCMBundle{
		Provisioner: scm, Pipeline: pipe, Registry: res,
		GroupPath: "acme", ArgoNamespace: "devsecops", ClusterID: "c1",
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
