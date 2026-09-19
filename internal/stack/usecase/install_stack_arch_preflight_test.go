package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// DGX Spark(arm64)에서 Harbor 가 installing_harbor 단계에서 뜨지 않았다. 공식 이미지가
// amd64 뿐인데 설치는 노드를 보지 않았고, 헬름은 성공한 뒤 파드만 크래시했다(#270).
// 설치를 시작하기 전에 노드 아키텍처를 읽어, 뜰 이미지가 없는 도구가 있으면 멈춘다.
type archPreflightSpyExecutor struct {
	port.StepExecutor
	notices  []string
	err      error
	calls    int
	stackIDs []string
	executed []string
}

func (e *archPreflightSpyExecutor) PreflightArchitecture(_ context.Context, stackID string) ([]string, error) {
	e.calls++
	e.stackIDs = append(e.stackIDs, stackID)
	return e.notices, e.err
}

func (e *archPreflightSpyExecutor) ExecuteStep(_ context.Context, _ string, step string, _ string) error {
	e.executed = append(e.executed, step)
	return nil
}

func TestInstallStack_StopsWhenNodeArchitectureUnsupported(t *testing.T) {
	stack := &domain.Stack{ID: "stk_arch_block", Namespace: "nullus-demo", State: domain.StatePending}
	repo := newFakeStackRepo(stack)
	executor := &archPreflightSpyExecutor{err: errors.New("Harbor(installing_harbor) 는 노드 아키텍처 s390x 에서 뜰 이미지가 없습니다")}

	uc := NewInstallStack(repo, &fakeStreamer{}, WithExecutor(executor))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{StackID: "stk_arch_block"}))

	waitForState(t, repo, "stk_arch_block", domain.StateFailed)
	assert.Equal(t, 1, executor.calls)
	assert.Empty(t, executor.executed, "검사에서 멈췄으면 설치 단계는 하나도 돌지 않는다")
}

// 대체 이미지를 쓰는 사실은 설치 로그에 남아야 한다 — 공식이 아닌 이미지가 깔린다.
func TestInstallStack_LogsArchitectureNotices(t *testing.T) {
	stack := &domain.Stack{ID: "stk_arch_notice", Namespace: "nullus-demo", State: domain.StatePending}
	repo := newFakeStackRepo(stack)
	notice := "Harbor: 공식 이미지가 노드 아키텍처 arm64 를 내지 않아 ghcr.io/dasomel/goharbor/*:v2.15.0-build.32 멀티아키 이미지로 설치합니다"
	executor := &archPreflightSpyExecutor{notices: []string{notice}}
	streamer := &fakeStreamer{}

	uc := NewInstallStack(repo, streamer, WithExecutor(executor))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{StackID: "stk_arch_notice"}))

	waitForState(t, repo, "stk_arch_notice", domain.StateCompleted)
	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	var found bool
	for _, e := range streamer.entries {
		if e.Message == notice {
			found = true
			assert.Equal(t, "validate", e.Step)
			assert.Equal(t, "info", e.Level)
		}
	}
	assert.True(t, found, "대체 이미지 알림이 설치 로그에 없다")
}

// 잔여 볼륨 검사와 달리 아키텍처 검사는 이어서 진행할 때도 돈다. 노드를 읽어야 남은
// 단계(예: installing_harbor)가 노드에 맞는 이미지를 받는다.
func TestInstallStack_ChecksArchitectureWhenContinuing(t *testing.T) {
	stack := &domain.Stack{
		ID:             "stk_arch_continue",
		Namespace:      "nullus-demo",
		State:          domain.StateFailed,
		LastFailedStep: "installing_harbor",
	}
	repo := newFakeStackRepo(stack)
	executor := &archPreflightSpyExecutor{}

	uc := NewInstallStack(repo, &fakeStreamer{}, WithExecutor(executor))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{
		StackID:        "stk_arch_continue",
		Continue:       true,
		ResumeFromStep: "installing_harbor",
	}))

	waitForState(t, repo, "stk_arch_continue", domain.StateCompleted)
	assert.Equal(t, 1, executor.calls)
	assert.Equal(t, []string{"stk_arch_continue"}, executor.stackIDs, "재개 지점을 알도록 스택을 넘긴다")
}

// 이어서 진행하다 검사에서 멈추면 재개 지점은 그대로 두고 실패 사유만 새로 적는다.
// 실패 단계를 validate 로 바꾸면 다음 Continue 가 처음부터 다시 돌고, 사유를 그대로 두면
// 화면에는 이전 실패(예: Harbor 크래시)가 계속 보인다.
func TestInstallStack_ContinueArchitectureFailureRecordsReason(t *testing.T) {
	stack := &domain.Stack{
		ID:                "stk_arch_continue_fail",
		Namespace:         "nullus-demo",
		State:             domain.StateFailed,
		LastFailedStep:    "installing_harbor",
		LastFailureReason: "harbor-core CrashLoopBackOff",
	}
	repo := newFakeStackRepo(stack)
	executor := &archPreflightSpyExecutor{err: errors.New("노드 아키텍처를 읽지 못해 Harbor(installing_harbor) 이(가) 쓸 이미지를 고를 수 없습니다")}

	uc := NewInstallStack(repo, &fakeStreamer{}, WithExecutor(executor))
	require.NoError(t, uc.Execute(context.Background(), InstallStackInput{
		StackID:        "stk_arch_continue_fail",
		Continue:       true,
		ResumeFromStep: "installing_harbor",
	}))

	waitForState(t, repo, "stk_arch_continue_fail", domain.StateFailed)
	got := repo.getStack("stk_arch_continue_fail")
	assert.Equal(t, "installing_harbor", got.LastFailedStep)
	assert.Contains(t, got.LastFailureReason, "노드 아키텍처를 읽지 못해")
	assert.Empty(t, executor.executed)
}
