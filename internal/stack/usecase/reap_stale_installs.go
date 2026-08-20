package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
)

// ReapStaleInstalls 는 끊긴 설치를 실패로 표시한다.
//
// 설치는 API 프로세스 안의 고루틴이 돌린다. 파드가 교체되면 그 고루틴이 사라지고
// 아무도 실패를 기록하지 않는다 — 스택은 installing 인 채로 영원히 남는다.
// 그 상태에서는 이어서 진행(continue)조차 막히므로(failed/pending 만 허용)
// 사용자에게는 지우고 다시 까는 길밖에 없다. 2026-08-20 운영에서 실제로 그렇게
// 갇혔고, 몇 시간 뒤에야 발견됐다.
//
// 살아 있는 설치와 구분할 방법은 시간뿐이다. 레플리카가 여럿인 환경에서는 "이
// 프로세스가 안 돌리고 있다" 가 "아무도 안 돌리고 있다" 를 뜻하지 않는다.
type ReapStaleInstalls struct {
	stacks port.StackRepository
	now    func() time.Time
}

func NewReapStaleInstalls(stacks port.StackRepository) *ReapStaleInstalls {
	return &ReapStaleInstalls{stacks: stacks, now: time.Now}
}

// Run 은 한 번 훑고 끊긴 설치를 실패로 옮긴다. 옮긴 개수를 돌려준다.
func (uc *ReapStaleInstalls) Run(ctx context.Context) (int, error) {
	stacks, err := uc.stacks.ListInFlight(ctx)
	if err != nil {
		return 0, fmt.Errorf("list in-flight stacks: %w", err)
	}

	now := uc.now()
	reaped := 0
	for _, stack := range stacks {
		if stack == nil || !domain.IsStaleInstall(stack.State, stack.UpdatedAt, now) {
			continue
		}

		stuckAt := firstNonBlank(stack.CurrentStep, stack.LastCompletedStep)
		stack.State = domain.StateFailed
		stack.LastFailedStep = stuckAt
		stack.LastFailureReason = staleInstallReason(stuckAt, now.Sub(stack.UpdatedAt))
		stack.UpdatedAt = now

		if err := uc.stacks.Update(ctx, stack); err != nil {
			slog.Warn("stale install reap failed", "stack_id", stack.ID, "error", err)
			continue
		}
		slog.Warn("stale install marked failed",
			"stack_id", stack.ID, "step", stuckAt, "idle", now.Sub(stack.UpdatedAt))
		reaped++
	}
	return reaped, nil
}

// staleInstallReason 은 무엇이 일어났고 무엇을 하면 되는지까지 적는다.
//
// "실패" 라고만 적으면 사용자는 설치가 잘못된 줄 안다. 실제로는 서버가 재시작돼
// 진행이 끊긴 것이고, 이어서 진행하면 그 자리부터 계속된다.
func staleInstallReason(step string, idle time.Duration) string {
	where := step
	if where == "" {
		where = "설치 중"
	}
	return fmt.Sprintf(
		"배포가 %s 에서 %d분째 진전이 없어 끊긴 것으로 표시했습니다. "+
			"설치 도중 서버가 재시작되면 진행이 이어지지 않습니다 — "+
			"이어서 진행(continue)하면 그 자리부터 계속됩니다.",
		where, int(idle.Minutes()))
}
