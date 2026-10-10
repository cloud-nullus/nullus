package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

type fakeTrigger struct {
	job    string
	branch string
	runURL string
	err    error
}

func (f *fakeTrigger) TriggerBuild(_ context.Context, job, branch string) (string, error) {
	f.job, f.branch = job, branch
	return f.runURL, f.err
}

type fakeFactory struct {
	bundle *port.SCMBundle
	err    error
}

func (f *fakeFactory) For(context.Context, string) (*port.SCMBundle, error) {
	return f.bundle, f.err
}

// 실행 주소는 CI 플랫폼마다 모양이 다르다(Jenkins 는 /job/…, GitLab 은 /-/pipelines/…).
// 위임기가 지어내지 않고 트리거가 돌려준 것을 그대로 쓴다.
func TestDelegateBuild_TriggersJobAndReturnsRunURL(t *testing.T) {
	trigger := &fakeTrigger{runURL: "https://gitlab.nullus.local/acme/orders-api/-/pipelines/12"}
	d := NewDelegate(&fakeFactory{bundle: &port.SCMBundle{CITrigger: trigger}}, nil)

	runURL, err := d.DelegateBuild(context.Background(), port.DelegateBuildOpts{
		StackID: "stk-1", JobName: "orders-api", Branch: "main",
	})

	require.NoError(t, err)
	assert.Equal(t, "orders-api", trigger.job)
	assert.Equal(t, "main", trigger.branch)
	assert.Equal(t, "https://gitlab.nullus.local/acme/orders-api/-/pipelines/12", runURL)
}

// CI 플랫폼이 없는 스택에 실행을 넘기라고 하면, 무엇이 없는지 말하고 멈춘다.
// 넘길 수 있는 플랫폼을 모두 말한다 — Jenkins 만 말하면 GitLab 스택 사용자는 길을 잃는다.
func TestDelegateBuild_ReportsMissingCIPlatform(t *testing.T) {
	d := NewDelegate(&fakeFactory{bundle: &port.SCMBundle{}}, nil)

	_, err := d.DelegateBuild(context.Background(), port.DelegateBuildOpts{StackID: "stk-1", JobName: "orders-api"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Jenkins")
	assert.Contains(t, err.Error(), "GitLab")
}

// job 이 없다는 404 는 "프로비저닝이 끝나지 않았다" 는 뜻이다. 그대로 옮기면
// 사용자는 무엇을 해야 하는지 알 수 없다.
func TestDelegateBuild_ExplainsMissingJob(t *testing.T) {
	d := NewDelegate(&fakeFactory{bundle: &port.SCMBundle{
		CITrigger: &fakeTrigger{err: errors.New("jenkins POST /job/orders-api/job/main/build: 404 Not Found")},
	}}, nil)

	_, err := d.DelegateBuild(context.Background(), port.DelegateBuildOpts{StackID: "stk-1", JobName: "orders-api", Branch: "main"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "프로비저닝")
}
