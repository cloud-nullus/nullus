package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// fakeCredentialPlane 은 Gitea 스택의 파이프라인 자격증명 평면 흉내다. events 에 호출 순서를 남긴다.
type fakeCredentialPlane struct {
	events    *[]string
	revokeErr error
}

func (f *fakeCredentialPlane) Provision(context.Context, string, []port.PipelineVariable) (string, error) {
	return "", nil
}

func (f *fakeCredentialPlane) CredentialObject(app string) (port.PipelineCredentialObject, error) {
	if app == ".." {
		return port.PipelineCredentialObject{}, errors.New("not one segment")
	}
	return port.PipelineCredentialObject{
		Namespace: "stack-ns", Name: "nullus-ci-" + app, RemoteKeyPrefix: "kv/x/cicd/pipelines/" + app + "/",
	}, nil
}

func (f *fakeCredentialPlane) Revoke(_ context.Context, app string) error {
	*f.events = append(*f.events, "revoke "+app)
	return f.revokeErr
}

type fakeCredentialObjectDeleter struct {
	events *[]string
	err    error
}

func (f *fakeCredentialObjectDeleter) DeleteCredentialObject(_ context.Context, _ []byte, obj port.PipelineCredentialObject) error {
	*f.events = append(*f.events, "delete "+obj.Namespace+"/"+obj.Name)
	return f.err
}

func credentialDeleteFixture(t *testing.T, planeErr, deleterErr error) (*DeletePipeline, *fakePipelineRepo, *[]string) {
	t.Helper()
	events := &[]string{}
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), harborResolver())
	bundle.Credentials = &fakeCredentialPlane{events: events, revokeErr: planeErr}
	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{}).
		WithCredentialObjectDeleter(&fakeCredentialObjectDeleter{events: events, err: deleterErr})
	return uc, repo, events
}

// Gitea · Jenkins 스택은 파이프라인 자격증명을 OpenBao → ESO → Secret 으로 나른다. 그 자격증명은
// CI job 만 쓰고 CI job 은 플래그 없이 지우므로, 자격증명도 플래그 없이 함께 회수한다.
// ExternalSecret 을 먼저 지운다 — 금고 값을 먼저 지우면 ESO 가 남은 객체를 동기화하다 실패한다.
func TestDeletePipeline_RevokesCICredentialsWithoutAnyFlag(t *testing.T) {
	uc, repo, events := credentialDeleteFixture(t, nil, nil)

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"delete stack-ns/nullus-ci-myapp", "revoke myapp"}, *events)
	assert.Equal(t, []string{"pip_1"}, repo.deleted)
	assert.Empty(t, out.Warnings)
}

// 회수에 실패해도 삭제는 막지 않는다(CI job 과 같다). 대신 무엇이 남았는지 알린다.
func TestDeletePipeline_WarnsWhenCredentialObjectDeleteFails(t *testing.T) {
	uc, repo, events := credentialDeleteFixture(t, nil, errors.New("forbidden"))

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"delete stack-ns/nullus-ci-myapp", "revoke myapp"}, *events,
		"클러스터 객체를 못 지워도 금고 값은 회수한다 — 자격증명이 남는 편이 더 나쁘다")
	assert.Equal(t, []string{"pip_1"}, repo.deleted)
	joined := strings.Join(out.Warnings, "\n")
	assert.Contains(t, joined, "nullus-ci-myapp")
}

func TestDeletePipeline_WarnsWhenCredentialRevokeFails(t *testing.T) {
	uc, repo, _ := credentialDeleteFixture(t, errors.New("sealed"), nil)

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"pip_1"}, repo.deleted)
	assert.Contains(t, strings.Join(out.Warnings, "\n"), "kv/x/cicd/pipelines/myapp/")
}

// CI 변수 저장소가 있는 스택(GitLab · GitHub)에는 평면이 없다 — 할 일이 없다.
func TestDeletePipeline_SkipsCredentialsWhenPlatformHasNone(t *testing.T) {
	events := &[]string{}
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), harborResolver())
	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{}).
		WithCredentialObjectDeleter(&fakeCredentialObjectDeleter{events: events})

	_, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)
	assert.Empty(t, *events)
}

// 삭제기가 배선되지 않으면 종전대로 둔다 — 금고 값만 지우면 남은 ExternalSecret 이 실패한다.
func TestDeletePipeline_CredentialCleanupIsOptional(t *testing.T) {
	events := &[]string{}
	repo := &fakePipelineRepo{pipeline: deletablePipeline()}
	bundle := newBundle(newFakeSCM(), newFakePipelineConfig(), harborResolver())
	bundle.Credentials = &fakeCredentialPlane{events: events}
	uc := NewDeletePipeline(repo, &fakeBundleFactory{bundle: bundle}, &fakeKubeconfigProvider{})

	_, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)
	assert.Empty(t, *events)
}

// 같은 스택에 같은 이름의 다른 파이프라인이 있으면 자격증명을 함께 쓴다(이름으로 찾는다).
// 하나를 지운다고 회수하면 남은 파이프라인의 CI 가 멈추고 사용자가 넣은 값은 되살릴 수 없다.
func TestDeletePipeline_KeepsCredentialsSharedWithSameNamedPipeline(t *testing.T) {
	uc, repo, events := credentialDeleteFixture(t, nil, nil)
	repo.siblings = []*domain.Pipeline{
		deletablePipeline(),
		{ID: "pip_2", Name: " myapp ", StackID: "stk_1", Namespace: "other"},
	}

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Empty(t, *events)
	assert.Equal(t, []string{"pip_1"}, repo.deleted)
	assert.Contains(t, strings.Join(out.Warnings, "\n"), "pip_2")
}

// 같은 이름이 있는지 확인하지 못하면 지우지 않는다 — 모르는 채 되돌릴 수 없는 일을 하지 않는다.
func TestDeletePipeline_KeepsCredentialsWhenSiblingLookupFails(t *testing.T) {
	uc, repo, events := credentialDeleteFixture(t, nil, nil)
	repo.listErr = errors.New("db down")

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Empty(t, *events)
	assert.NotEmpty(t, out.Warnings)
}

// 경로로 쓸 수 없는 이름이면 아무것도 지우지 않고 알린다.
func TestDeletePipeline_SkipsCredentialsForUnsafeName(t *testing.T) {
	uc, repo, events := credentialDeleteFixture(t, nil, nil)
	repo.pipeline.Name = ".."

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Empty(t, *events)
	assert.NotEmpty(t, out.Warnings)
}

type fakeNameLookup struct {
	others []string
	err    error
	calls  []string
}

func (f *fakeNameLookup) OtherPipelinesNamed(_ context.Context, stackID, name, excludeID string) ([]string, error) {
	f.calls = append(f.calls, stackID+"/"+name+"/"+excludeID)
	return f.others, f.err
}

// 목록 조회는 최근 100개만 돌려준다. 이름 조회가 배선되면 그것으로 빠짐없이 찾는다.
func TestDeletePipeline_UsesNameLookupForSharedCredentials(t *testing.T) {
	uc, _, events := credentialDeleteFixture(t, nil, nil)
	lookup := &fakeNameLookup{others: []string{"pip_old"}}
	uc.WithPipelineNameLookup(lookup)

	out, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"stk_1/myapp/pip_1"}, lookup.calls)
	assert.Empty(t, *events)
	assert.Contains(t, strings.Join(out.Warnings, "\n"), "pip_old")
}

func TestDeletePipeline_RevokesWhenNameLookupFindsNoOne(t *testing.T) {
	uc, _, events := credentialDeleteFixture(t, nil, nil)
	uc.WithPipelineNameLookup(&fakeNameLookup{})

	_, err := uc.Execute(context.Background(), DeletePipelineInput{PipelineID: "pip_1"})
	require.NoError(t, err)

	assert.Equal(t, []string{"delete stack-ns/nullus-ci-myapp", "revoke myapp"}, *events)
}
