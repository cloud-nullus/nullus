package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type fakePipelineRepo struct {
	pipeline  *domain.Pipeline
	getErr    error
	deleteErr error
	deleted   []string
}

func (f *fakePipelineRepo) Create(context.Context, *domain.Pipeline) error { return nil }

func (f *fakePipelineRepo) GetByID(_ context.Context, _ string) (*domain.Pipeline, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.pipeline, nil
}

func (f *fakePipelineRepo) List(context.Context, string) ([]*domain.Pipeline, error) { return nil, nil }

func (f *fakePipelineRepo) ListByStackID(context.Context, string) ([]*domain.Pipeline, error) {
	return nil, nil
}

func (f *fakePipelineRepo) Update(context.Context, *domain.Pipeline) error { return nil }

func (f *fakePipelineRepo) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeArgoAppDeleter struct {
	calls []string
	err   error
}

func (f *fakeArgoAppDeleter) DeleteApplication(_ context.Context, _ []byte, namespace, name string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, namespace+"/"+name)
	return nil
}

type fakeImageDeleter struct {
	deleted []string
	err     error
}

func (f *fakeImageDeleter) DeleteImageRepository(_ context.Context, target *port.ImageTarget) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, target.Repository)
	return nil
}

func deletablePipeline() *domain.Pipeline {
	return &domain.Pipeline{
		ID: "pip_1", Name: "myapp", ClusterID: "c1",
		StackID: "stk_1", Namespace: "apps",
	}
}

// deleteFixture 는 유스케이스와 대역들을 함께 만든다.
func deleteFixture(t *testing.T) (
	*DeletePipeline, *fakePipelineRepo, *fakeSCM, *fakeArgoAppDeleter, *fakeImageDeleter,
) {
	t.Helper()
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	scm := newFakeSCM()
	images := &fakeImageDeleter{}
	argo := &fakeArgoAppDeleter{}

	bundle := newBundle(scm, newFakePipelineConfig(), harborResolver())
	bundle.Images = images
	// CD 도구의 삭제기는 번들이 공급한다 — 스택마다 다른 도구를 쓸 수 있다.
	bundle.CDApplications = argo

	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{})
	return uc, repo, scm, argo, images
}

// 아무것도 고르지 않으면 레코드만 지운다 — 종전 동작이다.
func TestDeletePipeline_RecordOnlyByDefault(t *testing.T) {
	uc, repo, scm, argo, images := deleteFixture(t)

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"pip_1"}, repo.deleted)
	assert.Empty(t, scm.deleted, "고르지 않은 저장소는 건드리지 않는다")
	assert.Empty(t, argo.calls)
	assert.Empty(t, images.deleted)
	assert.False(t, out.RepositoryDeleted)
}

func TestDeletePipeline_DeletesClusterResourcesWhenRequested(t *testing.T) {
	uc, repo, _, argo, _ := deleteFixture(t)

	out, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteClusterResources: true,
	})
	require.NoError(t, err)

	// 애플리케이션은 **CD 도구의 네임스페이스**(devsecops)에 산다. 예전에는
	// 배포 대상 네임스페이스(apps)를 넘겼는데, 구현체가 "이미 없음" 을 성공으로
	// 보므로 애플리케이션이 조용히 남았다.
	assert.Equal(t, []string{"devsecops/myapp"}, argo.calls)
	assert.True(t, out.ClusterResourcesDeleted)
	assert.Equal(t, []string{"pip_1"}, repo.deleted)
}

// 저장소 경로는 그룹 경로와 앱 이름으로 조합한다 — git_repo_url 은 스킴과
// .git 접미사가 붙어 API 경로로 바로 쓸 수 없다.
func TestDeletePipeline_DeletesRepositoryWithGroupPath(t *testing.T) {
	uc, _, scm, _, _ := deleteFixture(t)

	out, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteRepository: true,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"acme/myapp"}, scm.deleted)
	assert.True(t, out.RepositoryDeleted)
}

func TestDeletePipeline_DeletesImagesWhenRequested(t *testing.T) {
	uc, _, _, _, images := deleteFixture(t)

	out, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteImages: true,
	})
	require.NoError(t, err)

	require.Len(t, images.deleted, 1)
	assert.True(t, out.ImagesDeleted)
}

// 지원하지 않는 레지스트리에서 조용히 넘어가면 사용자는 이미지가 지워진 줄 안다.
// 레지스트리가 이미지 삭제를 지원하지 않는 것은 삭제의 실패가 아니라 이 플랫폼이
// 할 수 없는 일이다. 그것으로 삭제 전체를 막으면 파이프라인을 영영 못 지운다 —
// 2026-08-21 운영에서 그랬다. 클러스터 리소스는 이미 지워진 뒤인데 레코드는 남고,
// images=true 인 한 몇 번을 눌러도 400 이었다.
//
// 할 수 있는 것은 다 하고, 하지 못한 것을 정확히 말한다.
func TestDeletePipeline_ContinuesWhenImageDeletionUnsupported(t *testing.T) {
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	scm := newFakeSCM()
	bundle := newBundle(scm, newFakePipelineConfig(), harborResolver())
	bundle.Images = nil // Harbor·Nexus 처럼 삭제 수단이 없는 구성

	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{})

	out, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteImages: true, DeleteRepository: true,
	})

	require.NoError(t, err)
	assert.False(t, out.ImagesDeleted, "지우지 못한 것을 지웠다고 하면 안 된다")
	assert.True(t, out.RepositoryDeleted, "할 수 있는 것은 해야 한다")
	assert.NotEmpty(t, repo.deleted, "레코드는 지워져야 한다 — 남기면 다시 눌러도 같은 자리에서 막힌다")

	require.Len(t, out.Warnings, 1)
	assert.Contains(t, out.Warnings[0], "이미지")
}

// 지원하지 않는 것과 진짜 실패는 다르다. 진짜 실패는 레코드를 남겨 다시 시도할
// 수 있게 한다.
func TestDeletePipeline_KeepsRecordWhenImageDeleteFails(t *testing.T) {
	uc, repo, _, _, images := deleteFixture(t)
	images.err = errors.New("harbor unreachable")

	_, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteImages: true,
	})

	require.Error(t, err)
	assert.Empty(t, repo.deleted)
}

// 요청한 삭제가 실패하면 레코드를 남긴다. 레코드가 사라지면 목록에서 보이지
// 않는데 리소스는 남아, 사용자가 다시 시도할 방법조차 없어진다.
func TestDeletePipeline_KeepsRecordWhenRepositoryDeleteFails(t *testing.T) {
	uc, repo, scm, _, _ := deleteFixture(t)
	scm.deleteErr = errors.New("403 forbidden")

	_, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteRepository: true,
	})
	require.Error(t, err)
	assert.Empty(t, repo.deleted)
}

func TestDeletePipeline_KeepsRecordWhenClusterCleanupFails(t *testing.T) {
	uc, repo, _, argo, _ := deleteFixture(t)
	argo.err = errors.New("connection refused")

	_, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteClusterResources: true,
	})
	require.Error(t, err)
	assert.Empty(t, repo.deleted)
}

func TestDeletePipeline_RequiresPipelineID(t *testing.T) {
	uc, _, _, _, _ := deleteFixture(t)

	_, err := uc.Execute(context.Background(), DeletePipelineInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline id")
}

type fakeWorkloadDeleter struct {
	calls []string
	err   error
}

func (f *fakeWorkloadDeleter) DeleteByLabel(_ context.Context, _ []byte, namespace, selector string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, namespace+"/"+selector)
	return nil
}

// Argo CD Application 이 없는 파이프라인(매니페스트 직접 적용)은 워크로드를
// 지워 줄 주체가 없다. 그대로 두면 목록에서는 사라졌는데 클러스터에는 앱이
// 계속 도는 상태가 남는다.
func TestDeletePipeline_DeletesDirectlyAppliedWorkloads(t *testing.T) {
	uc, _, _, _, _ := deleteFixture(t)
	workloads := &fakeWorkloadDeleter{}
	uc.WithWorkloadDeleter(workloads)

	_, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteClusterResources: true,
	})
	require.NoError(t, err)

	// 매니페스트 생성기가 모든 리소스에 붙이는 라벨이다.
	assert.Equal(t, []string{"apps/app=myapp"}, workloads.calls)
}

// 워크로드 정리에 실패하면 레코드를 남긴다. 레코드가 사라지면 남은 리소스를
// 찾을 방법도, 다시 시도할 방법도 없어진다.
func TestDeletePipeline_KeepsRecordWhenWorkloadCleanupFails(t *testing.T) {
	uc, repo, _, _, _ := deleteFixture(t)
	uc.WithWorkloadDeleter(&fakeWorkloadDeleter{err: assert.AnError})

	_, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteClusterResources: true,
	})
	require.Error(t, err)
	assert.Empty(t, repo.deleted)
}

// 배선되지 않으면 종전대로 Argo CD 경로만 정리한다.
func TestDeletePipeline_WorkloadCleanupIsOptional(t *testing.T) {
	uc, repo, _, argo, _ := deleteFixture(t)

	_, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteClusterResources: true,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"devsecops/myapp"}, argo.calls)
	assert.Equal(t, []string{"pip_1"}, repo.deleted)
}

// CD 도구를 지원하지 않는 스택에서도 삭제는 진행돼야 한다. 애플리케이션을
// 지울 수단이 없는 것이 파이프라인을 못 지울 이유는 아니다.
func TestDeletePipeline_ContinuesWhenCDToolHasNoDeleter(t *testing.T) {
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), harborResolver())
	bundle.CDApplications = nil

	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{})

	out, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteClusterResources: true,
	})

	require.NoError(t, err)
	assert.True(t, out.ClusterResourcesDeleted)
	assert.NotEmpty(t, repo.deleted)
}

// CD 네임스페이스를 모르면 지우지 않는다. 배포 대상 네임스페이스로 대신
// 시도하면 없는 곳을 뒤지고 성공으로 끝나, 애플리케이션이 조용히 남는다.
func TestDeletePipeline_RefusesWhenCDNamespaceUnknown(t *testing.T) {
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), harborResolver())
	bundle.CDApplications = &fakeArgoAppDeleter{}
	bundle.CDNamespace = ""

	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{})

	_, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteClusterResources: true,
	})

	require.Error(t, err)
	assert.Empty(t, repo.deleted, "지우지 못했으면 레코드를 남겨 다시 시도할 수 있어야 한다")
}

type recordingCIJobs struct {
	deleted []string
	err     error
}

func (r *recordingCIJobs) EnsureJob(context.Context, port.CIJobSpec) (*port.CIJob, error) {
	return &port.CIJob{}, nil
}

func (r *recordingCIJobs) DeleteJob(_ context.Context, name string) error {
	if r.err != nil {
		return r.err
	}
	r.deleted = append(r.deleted, name)
	return nil
}

// CI job 은 플래그 없이도 지운다. 파이프라인이 사라진 뒤에 job 이 남으면
// 없어진 리포를 계속 스캔하며 실패한다.
func TestDeletePipeline_DeletesCIJobWithoutAnyFlag(t *testing.T) {
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	jobs := &recordingCIJobs{}
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), harborResolver())
	bundle.CIJobs = jobs

	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{})

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})

	require.NoError(t, err)
	assert.Equal(t, []string{"myapp"}, jobs.deleted)
	assert.Empty(t, out.Warnings)
}

// CI 서버가 잠깐 응답하지 않는다고 파이프라인을 못 지우면, 클러스터 리소스는
// 이미 지워진 뒤라 되돌리기 어려운 상태에 갇힌다. 경고로 남기고 진행한다.
func TestDeletePipeline_ContinuesWhenCIJobDeleteFails(t *testing.T) {
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), harborResolver())
	bundle.CIJobs = &recordingCIJobs{err: errors.New("jenkins unreachable")}

	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{})

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})

	require.NoError(t, err)
	assert.NotEmpty(t, repo.deleted)
	require.Len(t, out.Warnings, 1)
	assert.Contains(t, out.Warnings[0], "CI job")
}

// GitLab CI·GitHub Actions 는 파이프라인 정의를 리포에서 읽으므로 지울 job 이
// 없다. CIJobs 가 nil 인 그 구성에서 경고를 남기면 매번 거짓 경고가 뜬다.
func TestDeletePipeline_SilentWhenCIHasNoJobs(t *testing.T) {
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), harborResolver())
	bundle.CIJobs = nil

	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{})

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})

	require.NoError(t, err)
	assert.Empty(t, out.Warnings)
}

type unavailableStackFactory struct{}

func (unavailableStackFactory) For(context.Context, string) (*port.SCMBundle, error) {
	return nil, fmt.Errorf("%w: stack stk_1 (state=%q)", port.ErrStackToolsUnavailable, "cancelled")
}

// 스택을 먼저 지운 파이프라인도 지워져야 한다.
//
// 스택이 사라지면 그 도구들도 함께 사라진다. 그것을 삭제의 실패로 다루면
// 파이프라인이 영영 지워지지 않고, 목록에는 보이는데 손댈 수 없는 좀비가 남는다 —
// 2026-08-21 운영에서 그랬다.
func TestDeletePipeline_DeletesRecordWhenStackIsGone(t *testing.T) {
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	uc := NewDeletePipeline(repo, unavailableStackFactory{}, &fakeKubeconfigProvider{})

	out, err := uc.Execute(context.Background(), DeletePipelineInput{
		PipelineID: "pip_1", DeleteRepository: true, DeleteImages: true,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, repo.deleted, "레코드는 지워져야 한다 — 남기면 손댈 수 없는 좀비가 된다")
	assert.False(t, out.RepositoryDeleted)
	assert.False(t, out.ImagesDeleted)

	// 지우지 못한 것은 정확히 말한다. 조용히 성공으로 넘기면 사용자는
	// 저장소와 이미지가 사라진 줄 안다.
	require.NotEmpty(t, out.Warnings)
	assert.Contains(t, out.Warnings[0], "직접 지워야")
}
