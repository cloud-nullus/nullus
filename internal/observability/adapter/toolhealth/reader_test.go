package toolhealth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	obskube "github.com/cloud-nullus/draft/internal/observability/adapter/kube"
	obsdomain "github.com/cloud-nullus/draft/internal/observability/domain"
	stackdomain "github.com/cloud-nullus/draft/internal/stack/domain"
)

type fakeStackLister struct {
	stacks []*stackdomain.Stack
	err    error
}

func (f *fakeStackLister) List(_ context.Context, _ string, _ bool) ([]*stackdomain.Stack, error) {
	return f.stacks, f.err
}

type fakeKubeconfigProvider struct {
	err error
}

func (f *fakeKubeconfigProvider) GetKubeconfig(_ context.Context, _ string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte("kubeconfig"), nil
}

func harborStack(ns string) *stackdomain.Stack {
	return &stackdomain.Stack{
		ID:        "stk-1",
		ClusterID: "cluster-1",
		Namespace: ns,
		State:     stackdomain.StateCompleted,
		Config: stackdomain.StackConfig{
			Artifacts: stackdomain.ArtifactsConfig{
				SourceRepository:  stackdomain.ToolSelection{Name: "GitLab CE", Version: "17.7.0", Enabled: true},
				ContainerRegistry: stackdomain.ToolSelection{Name: "Harbor", Version: "2.11.0", Enabled: true},
			},
		},
	}
}

func newReader(stacks []*stackdomain.Stack, pods map[string][]obskube.PodInfo) *Reader {
	r := New(&fakeStackLister{stacks: stacks}, &fakeKubeconfigProvider{})
	r.listPods = func(_ context.Context, _ []byte, ns string) ([]obskube.PodInfo, error) {
		return pods[ns], nil
	}
	return r
}

func healthByName(items []obsdomain.ToolHealth) map[string]obsdomain.ToolHealth {
	out := map[string]obsdomain.ToolHealth{}
	for _, item := range items {
		out[item.Name] = item
	}
	return out
}

func TestReader_ReportsRunningWhenEveryPodIsReady(t *testing.T) {
	r := newReader([]*stackdomain.Stack{harborStack("ns-a")}, map[string][]obskube.PodInfo{
		"ns-a": {
			{Name: "gitlab-webservice-default-0", Phase: "Running", Ready: true},
			{Name: "harbor-core-abc", Phase: "Running", Ready: true},
			{Name: "harbor-registry-def", Phase: "Running", Ready: true},
		},
	})

	got, err := r.ListToolHealth(context.Background(), "org-1")
	require.NoError(t, err)

	byName := healthByName(got)
	require.Contains(t, byName, "Harbor")
	assert.Equal(t, "running", byName["Harbor"].Status)
	assert.Equal(t, "2.11.0", byName["Harbor"].Version)
	assert.Equal(t, "running", byName["GitLab CE"].Status)
}

func TestReader_ReportsWarningWhenAPodIsNotReady(t *testing.T) {
	r := newReader([]*stackdomain.Stack{harborStack("ns-a")}, map[string][]obskube.PodInfo{
		"ns-a": {
			{Name: "harbor-core-abc", Phase: "Running", Ready: true},
			{Name: "harbor-registry-def", Phase: "Running", Ready: false},
		},
	})

	got, err := r.ListToolHealth(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "warning", healthByName(got)["Harbor"].Status)
}

func TestReader_ReportsErrorOnCrashLoop(t *testing.T) {
	r := newReader([]*stackdomain.Stack{harborStack("ns-a")}, map[string][]obskube.PodInfo{
		"ns-a": {
			{Name: "harbor-core-abc", Phase: "Running", Ready: false, Status: "CrashLoopBackOff"},
		},
	})

	got, err := r.ListToolHealth(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "error", healthByName(got)["Harbor"].Status)
}

// 설치했다고 선언된 도구에 파드가 하나도 없으면 정상이 아니다.
func TestReader_ReportsWarningWhenToolHasNoPods(t *testing.T) {
	r := newReader([]*stackdomain.Stack{harborStack("ns-a")}, map[string][]obskube.PodInfo{
		"ns-a": {{Name: "gitlab-webservice-default-0", Phase: "Running", Ready: true}},
	})

	got, err := r.ListToolHealth(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "warning", healthByName(got)["Harbor"].Status)
}

// 마이그레이션 Job 은 Succeeded 로 끝나며 Ready 가 아니다. 건강도에서 빼지 않으면
// 정상인 스택이 영구히 warning 으로 보인다.
func TestReader_IgnoresCompletedOneShotJobPods(t *testing.T) {
	r := newReader([]*stackdomain.Stack{harborStack("ns-a")}, map[string][]obskube.PodInfo{
		"ns-a": {
			{Name: "gitlab-webservice-default-0", Phase: "Running", Ready: true},
			{Name: "gitlab-migrations-abc", Phase: "Succeeded", Ready: false},
		},
	})

	got, err := r.ListToolHealth(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "running", healthByName(got)["GitLab CE"].Status)
}

// 같은 도구가 여러 스택에 있으면 한 줄로 합치되, 가장 나쁜 상태를 보여준다.
func TestReader_MergesSameToolAcrossStacksWorstStatusWins(t *testing.T) {
	second := harborStack("ns-b")
	second.ID = "stk-2"

	r := newReader([]*stackdomain.Stack{harborStack("ns-a"), second}, map[string][]obskube.PodInfo{
		"ns-a": {{Name: "harbor-core-abc", Phase: "Running", Ready: true}},
		"ns-b": {{Name: "harbor-core-xyz", Phase: "Running", Ready: false, Status: "CrashLoopBackOff"}},
	})

	got, err := r.ListToolHealth(context.Background(), "org-1")
	require.NoError(t, err)

	harbors := 0
	for _, item := range got {
		if item.Name == "Harbor" {
			harbors++
		}
	}
	assert.Equal(t, 1, harbors, "the same tool must collapse into one row")
	assert.Equal(t, "error", healthByName(got)["Harbor"].Status)
}

// 아직 안 끝난 스택의 파드는 아직 뜨는 중이라 건강도 판단 대상이 아니다.
func TestReader_SkipsStacksThatAreNotCompleted(t *testing.T) {
	installing := harborStack("ns-a")
	installing.State = stackdomain.StateInstalling

	r := newReader([]*stackdomain.Stack{installing}, map[string][]obskube.PodInfo{"ns-a": nil})

	got, err := r.ListToolHealth(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestReader_PropagatesStackListFailure(t *testing.T) {
	r := New(&fakeStackLister{err: errors.New("boom")}, &fakeKubeconfigProvider{})

	_, err := r.ListToolHealth(context.Background(), "org-1")
	assert.Error(t, err)
}

// 클러스터 한 곳에 못 붙어도 나머지 스택 상태는 계속 보여준다.
func TestReader_DegradesWhenOneClusterIsUnreachable(t *testing.T) {
	r := New(&fakeStackLister{stacks: []*stackdomain.Stack{harborStack("ns-a")}},
		&fakeKubeconfigProvider{err: errors.New("unreachable")})

	got, err := r.ListToolHealth(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Empty(t, got)
}
