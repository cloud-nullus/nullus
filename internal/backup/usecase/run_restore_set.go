package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// RestoreFactory 는 스택(빈 값이면 플랫폼) 전용 복구 유스케이스를 만든다 — wiring.Module.Restore 와 같다.
type RestoreFactory func(ctx context.Context, stackID string) (*RestoreUseCase, error)

type RestoreSetDeps struct {
	Repo    port.BackupRepository
	Factory RestoreFactory
	Logger  *slog.Logger
}

// RestoreSetUseCase 는 백업 세트를 플랫폼 → 스택 순서로 되돌린다(domain.PlanSetRestore).
//
// 플랫폼 복구가 실패하면 멈춘다 — 스택 복구는 플랫폼 DB 의 스택 기록(클러스터)으로 그 클러스터에 붙으므로
// 플랫폼이 없으면 어디로 가야 할지도 모른다. 스택 하나의 실패는 다른 스택을 막지 않는다.
type RestoreSetUseCase struct {
	d RestoreSetDeps
	// runner 는 멤버 복구 하나를 돈다. 기본은 RestoreUseCase.Run — 테스트가 가로챈다.
	runner func(ctx context.Context, uc *RestoreUseCase, req RunRestoreRequest) (*domain.RestoreRun, error)
}

func NewRestoreSetUseCase(d RestoreSetDeps) *RestoreSetUseCase {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &RestoreSetUseCase{d: d, runner: func(ctx context.Context, uc *RestoreUseCase, req RunRestoreRequest) (*domain.RestoreRun, error) {
		return uc.Run(ctx, req)
	}}
}

type RunRestoreSetRequest struct {
	SetID string
	OrgID string
}

// RestoreSetResult 는 멤버별 복구 기록과, 되돌리지 못한 멤버의 사유, 산출물이 없어 건너뛴 백업 ID 다.
type RestoreSetResult struct {
	SetID    string
	Restores []*domain.RestoreRun
	Failed   []string
	Skipped  []string
}

func (uc *RestoreSetUseCase) Run(ctx context.Context, req RunRestoreSetRequest) (*RestoreSetResult, error) {
	res := &RestoreSetResult{SetID: req.SetID}
	if uc.d.Repo == nil {
		return res, fmt.Errorf("백업 세트 복구가 배선되지 않았습니다")
	}
	runs, err := uc.d.Repo.ListRunsBySet(ctx, req.SetID)
	if err != nil {
		return res, fmt.Errorf("백업 세트 %s 조회: %w", req.SetID, err)
	}
	if len(runs) == 0 {
		return res, fmt.Errorf("백업 세트 %s 를 찾을 수 없습니다", req.SetID)
	}
	steps, skipped := domain.PlanSetRestore(runs)
	res.Skipped = skipped
	for _, id := range skipped {
		uc.d.Logger.Warn("실패한 백업 멤버는 되돌릴 산출물이 없어 건너뜁니다", "set_id", req.SetID, "backup_run_id", id)
	}
	if uc.d.Factory == nil {
		return res, fmt.Errorf("백업 세트 복구가 배선되지 않았습니다")
	}

	for _, step := range steps {
		member, err := uc.d.Factory(ctx, step.StackID)
		if err != nil {
			res.Failed = append(res.Failed, fmt.Sprintf("%s: %v", memberName(step.StackID), err))
			if step.Mode == domain.ModePlatformOnly {
				return res, fmt.Errorf("플랫폼 복구를 준비하지 못했습니다: %w", err)
			}
			continue
		}
		rr, runErr := uc.runner(ctx, member, RunRestoreRequest{
			BackupRunID: step.BackupRunID, OrgID: req.OrgID, Namespace: step.Namespace, Mode: step.Mode,
		})
		if rr != nil {
			res.Restores = append(res.Restores, rr)
		}
		if runErr != nil {
			name := memberName(step.StackID)
			if step.Namespace != "" {
				name = step.StackID + "(" + step.Namespace + ")"
			}
			res.Failed = append(res.Failed, fmt.Sprintf("%s: %v", name, runErr))
			if step.Mode == domain.ModePlatformOnly {
				// 스택 복구는 플랫폼 DB 의 스택 기록을 본다 — 플랫폼이 없으면 갈 곳을 모른다.
				return res, fmt.Errorf("플랫폼 복구 실패 — 스택은 되돌리지 않았습니다: %w", runErr)
			}
			uc.d.Logger.Error("세트의 스택 복구 실패 — 다음 스택으로 넘어갑니다", "set_id", req.SetID, "stack_id", step.StackID, "error", runErr)
		}
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
	}
	return res, nil
}
