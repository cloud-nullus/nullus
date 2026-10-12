package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// BackupFactory 는 스택(빈 값이면 플랫폼) 전용 백업 유스케이스를 만든다 — wiring.Module.Backup 과 같다.
type BackupFactory func(ctx context.Context, stackID string) (*BackupUseCase, error)

// BackupSetDeps 는 세트 백업의 의존이다.
type BackupSetDeps struct {
	Repo    port.BackupRepository
	Stacks  port.StackLister
	Factory BackupFactory
	Logger  *slog.Logger
}

// BackupSetUseCase 는 플랫폼 전체 — 플랫폼 + 조직의 모든 설치 완료 스택 — 를 한 세트로 뜬다
// (domain.PlanBackupSet). 멤버 하나가 실패해도 나머지는 계속 뜬다.
type BackupSetUseCase struct {
	d BackupSetDeps
}

func NewBackupSetUseCase(d BackupSetDeps) *BackupSetUseCase {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &BackupSetUseCase{d: d}
}

type RunBackupSetRequest struct {
	OrgID   string
	Trigger domain.Trigger
}

// BackupSetResult 는 세트의 실행들과 파생 상태다. Failed 는 실행조차 못 만든 멤버(유스케이스 조립
// 실패 등)의 사유다 — 실행이 만들어진 뒤의 실패는 그 실행의 Status·Error 에 있다.
type BackupSetResult struct {
	SetID  string
	Status domain.Status
	Runs   []*domain.BackupRun
	Failed []string
}

// Run 은 계획을 세우고 멤버를 순서대로 뜬다. 스택 목록을 못 읽으면 시작하지 않는다 — 무엇을 뜰지
// 모르는 채 플랫폼만 뜨고 "전체" 라고 하면 복구할 때에야 빠진 것이 드러난다.
func (uc *BackupSetUseCase) Run(ctx context.Context, req RunBackupSetRequest) (*BackupSetResult, error) {
	if uc.d.Stacks == nil || uc.d.Factory == nil {
		return nil, fmt.Errorf("백업 세트가 배선되지 않았습니다")
	}
	stacks, err := uc.d.Stacks.ListCompletedStacks(ctx, req.OrgID)
	if err != nil {
		return nil, fmt.Errorf("조직의 스택 목록 조회: %w", err)
	}
	trigger := req.Trigger
	if trigger == "" {
		trigger = domain.TriggerManual
	}
	res := &BackupSetResult{SetID: uuid.NewString()}
	uc.d.Logger.Info("백업 세트 시작", "set_id", res.SetID, "org_id", req.OrgID, "stacks", len(stacks))

	for _, m := range domain.PlanBackupSet(stacks) {
		member, err := uc.d.Factory(ctx, m.StackID)
		if err != nil {
			uc.d.Logger.Error("백업 세트 멤버를 준비하지 못했습니다 — 다음 멤버로 넘어갑니다",
				"set_id", res.SetID, "stack_id", m.StackID, "error", err)
			res.Failed = append(res.Failed, fmt.Sprintf("%s: %v", memberName(m.StackID), err))
			continue
		}
		run, runErr := member.Run(ctx, RunBackupRequest{
			OrgID: req.OrgID, StackID: m.StackID, Namespace: m.Namespace,
			Mode: m.Mode, Scope: m.Scope, Trigger: trigger, SetID: res.SetID,
		})
		if run != nil {
			res.Runs = append(res.Runs, run)
		}
		if runErr != nil {
			uc.d.Logger.Error("백업 세트 멤버 실패 — 다음 멤버로 넘어갑니다",
				"set_id", res.SetID, "stack_id", m.StackID, "error", runErr)
			if run == nil {
				res.Failed = append(res.Failed, fmt.Sprintf("%s: %v", memberName(m.StackID), runErr))
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	res.Status = domain.SetStatus(res.Runs)
	if len(res.Failed) > 0 && res.Status == domain.StatusSucceeded {
		res.Status = domain.StatusPartial
	}
	uc.d.Logger.Info("백업 세트 끝", "set_id", res.SetID, "status", res.Status, "runs", len(res.Runs), "failed", len(res.Failed))
	return res, nil
}

func memberName(stackID string) string {
	if stackID == "" {
		return "platform"
	}
	return stackID
}
