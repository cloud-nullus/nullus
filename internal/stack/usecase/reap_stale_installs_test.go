package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// 2026-08-20 운영에서 갇힌 상태를 재현한다.
//
// 설치 도중 API 파드가 교체되면 설치 고루틴이 사라진다. 아무도 실패를 기록하지
// 않아 스택은 installing 인 채로 남고, 그 상태에서는 continue 가 409 로 막힌다 —
// 지우고 다시 까는 길밖에 없었다.
func newReaper(t *testing.T, now time.Time, stacks ...*domain.Stack) (*ReapStaleInstalls, *stackrepo.MemoryStackRepository) {
	t.Helper()
	repo := stackrepo.NewMemoryStackRepository()
	for _, s := range stacks {
		require.NoError(t, repo.Create(context.Background(), s))
	}
	uc := NewReapStaleInstalls(repo)
	uc.now = func() time.Time { return now }
	return uc, repo
}

func TestReapStaleInstalls_MarksInterruptedInstallFailed(t *testing.T) {
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	stuck := &domain.Stack{
		ID:                "stk_stuck",
		Name:              "demo",
		State:             domain.StateInstalling,
		CurrentStep:       "installing_gateway",
		LastCompletedStep: "installing_jenkins",
		UpdatedAt:         now.Add(-2 * time.Hour),
	}
	uc, repo := newReaper(t, now, stuck)

	reaped, err := uc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, reaped)

	after, err := repo.GetByID(context.Background(), "stk_stuck")
	require.NoError(t, err)
	assert.Equal(t, domain.StateFailed, after.State)
	assert.Equal(t, "installing_gateway", after.LastFailedStep)
	// 이어서 진행하면 된다는 것까지 알려 준다 — "실패" 만 적으면 설치가 잘못된 줄 안다.
	assert.Contains(t, after.LastFailureReason, "이어서 진행")
	assert.Contains(t, after.LastFailureReason, "installing_gateway")
}

// 한 스텝이 오래 걸릴 수 있다(GitLab 은 helm --wait 만 15분). 살아 있는 설치를
// 죽이면 훨씬 나쁜 일이 된다.
func TestReapStaleInstalls_LeavesSlowButLiveInstalls(t *testing.T) {
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	live := &domain.Stack{
		ID:          "stk_live",
		Name:        "demo",
		State:       domain.StateInstalling,
		CurrentStep: "installing_gitlab",
		UpdatedAt:   now.Add(-12 * time.Minute),
	}
	uc, repo := newReaper(t, now, live)

	reaped, err := uc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, reaped)

	after, err := repo.GetByID(context.Background(), "stk_live")
	require.NoError(t, err)
	assert.Equal(t, domain.StateInstalling, after.State)
}

func TestReapStaleInstalls_IgnoresTerminalStacks(t *testing.T) {
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	done := &domain.Stack{
		ID:        "stk_done",
		Name:      "demo",
		State:     domain.StateCompleted,
		UpdatedAt: now.Add(-10 * time.Hour),
	}
	uc, _ := newReaper(t, now, done)

	reaped, err := uc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, reaped)
}

func TestReapStaleInstalls_EmptyClusterIsFine(t *testing.T) {
	uc, _ := newReaper(t, time.Now())

	reaped, err := uc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, reaped)
}
