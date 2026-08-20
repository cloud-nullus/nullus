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
	err    error
}

func (f *fakeTrigger) TriggerBuild(_ context.Context, job, branch string) error {
	f.job, f.branch = job, branch
	return f.err
}

type fakeFactory struct {
	bundle *port.SCMBundle
	err    error
}

func (f *fakeFactory) For(context.Context, string) (*port.SCMBundle, error) {
	return f.bundle, f.err
}

func TestDelegateBuild_TriggersJobAndReturnsRunURL(t *testing.T) {
	trigger := &fakeTrigger{}
	d := NewDelegate(&fakeFactory{bundle: &port.SCMBundle{
		CITrigger: trigger,
		CIBaseURL: "http://jenkins.nullus-stack.svc:8080",
	}}, nil)

	runURL, err := d.DelegateBuild(context.Background(), port.DelegateBuildOpts{
		StackID: "stk-1", JobName: "orders-api", Branch: "main",
	})

	require.NoError(t, err)
	assert.Equal(t, "orders-api", trigger.job)
	assert.Equal(t, "main", trigger.branch)
	assert.Equal(t, "http://jenkins.nullus-stack.svc:8080/job/orders-api/job/main/", runURL)
}

// CI 플랫폼이 없는 스택에 실행을 넘기라고 하면, 무엇이 없는지 말하고 멈춘다.
func TestDelegateBuild_ReportsMissingCIPlatform(t *testing.T) {
	d := NewDelegate(&fakeFactory{bundle: &port.SCMBundle{}}, nil)

	_, err := d.DelegateBuild(context.Background(), port.DelegateBuildOpts{StackID: "stk-1", JobName: "orders-api"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Jenkins")
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

// 주소를 모르면 링크를 지어내지 않는다 — 열리지 않는 링크가 화면에 남는다.
func TestJobRunURL_EmptyWhenBaseUnknown(t *testing.T) {
	assert.Equal(t, "", jobRunURL("", "orders-api", "main"))
	assert.Equal(t, "", jobRunURL("http://jenkins.local", "", "main"))
}
