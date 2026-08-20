package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// 이전 설치의 볼륨이 남아 있으면 새 설치가 옛 데이터베이스를 물려받는다. 그 안의
// 비밀번호는 이번에 새로 만든 Secret 과 다르고, 그 사실은 스무 단계쯤 뒤에 엉뚱한
// 도구의 오류로 드러난다 — Gitea 의 28P01 과 Harbor 의 401 로 두 번 나왔고,
// 매번 20분을 태운 뒤였다. 몇 초 만에 멈춰야 한다.
type preflightSpyExecutor struct {
	port.StepExecutor
	err      error
	calls    []string
	executed []string
}

func (e *preflightSpyExecutor) PreflightNamespace(_ context.Context, namespace string) error {
	e.calls = append(e.calls, namespace)
	return e.err
}

func (e *preflightSpyExecutor) ExecuteStep(_ context.Context, _ string, step string, _ string) error {
	e.executed = append(e.executed, step)
	return nil
}

func waitForState(t *testing.T, repo *fakeStackRepo, id string, want domain.DeploymentState) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if repo.getState(id) == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("상태가 %s 가 되지 않았다 (현재 %s)", want, repo.getState(id))
}

func TestInstallStack_StopsWhenPreviousVolumesRemain(t *testing.T) {
	stack := &domain.Stack{ID: "stk_preflight", Namespace: "nullus-demo", State: domain.StatePending}
	repo := newFakeStackRepo(stack)
	executor := &preflightSpyExecutor{err: errors.New("이전 설치의 볼륨이 남아 있습니다")}

	uc := NewInstallStack(repo, &fakeStreamer{}, WithExecutor(executor))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{StackID: "stk_preflight"}))

	waitForState(t, repo, "stk_preflight", domain.StateFailed)
	assert.Equal(t, []string{"nullus-demo"}, executor.calls)
	assert.Empty(t, executor.executed, "검사에서 멈췄으면 설치 단계는 하나도 돌지 않는다")
}

func TestInstallStack_RunsWhenNamespaceIsClean(t *testing.T) {
	stack := &domain.Stack{ID: "stk_clean", Namespace: "nullus-demo", State: domain.StatePending}
	repo := newFakeStackRepo(stack)
	executor := &preflightSpyExecutor{}

	uc := NewInstallStack(repo, &fakeStreamer{}, WithExecutor(executor))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{StackID: "stk_clean"}))

	waitForState(t, repo, "stk_clean", domain.StateCompleted)
	assert.Equal(t, []string{"nullus-demo"}, executor.calls)
	assert.NotEmpty(t, executor.executed)
}

// 이어서 진행할 때 남아 있는 볼륨은 지금 하고 있는 설치가 만든 것이다. 지우면 안 되고,
// 검사에 걸려서도 안 된다.
func TestInstallStack_SkipsPreflightWhenContinuing(t *testing.T) {
	stack := &domain.Stack{
		ID:             "stk_continue",
		Namespace:      "nullus-demo",
		State:          domain.StateFailed,
		LastFailedStep: "installing_postgresql",
	}
	repo := newFakeStackRepo(stack)
	executor := &preflightSpyExecutor{err: errors.New("이전 설치의 볼륨이 남아 있습니다")}

	uc := NewInstallStack(repo, &fakeStreamer{}, WithExecutor(executor))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{
		StackID:        "stk_continue",
		Continue:       true,
		ResumeFromStep: "installing_postgresql",
	}))

	waitForState(t, repo, "stk_continue", domain.StateCompleted)
	assert.Empty(t, executor.calls, "이어서 진행할 때는 검사하지 않는다")
}
