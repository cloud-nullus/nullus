package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
)

// 세트 복구 — 플랫폼(platform_only) → 스택마다(stack_only). 스택 복구가 플랫폼 DB 를 다시 덮지 않는다.
func TestRunRestoreSet_플랫폼_먼저_그_다음_스택마다_stack_only(t *testing.T) {
	// 세트를 하나 뜬다
	buc, repo, _, _ := seedSet(t, []domain.StackRef{{ID: "stk_a", Namespace: "nullus-a"}, {ID: "stk_b", Namespace: "nullus-b"}})
	set, err := buc.Run(context.Background(), RunBackupSetRequest{OrgID: "org-1"})
	require.NoError(t, err)

	var calls []string
	factory := func(_ context.Context, stackID string) (*RestoreUseCase, error) {
		return &RestoreUseCase{d: RestoreDeps{}}, nil // 실제 복구는 아래 runner 대역이 가로챈다
	}
	uc := NewRestoreSetUseCase(RestoreSetDeps{Repo: repo, Factory: factory})
	uc.runner = func(_ context.Context, _ *RestoreUseCase, req RunRestoreRequest) (*domain.RestoreRun, error) {
		calls = append(calls, string(req.Mode)+":"+req.BackupRunID+":"+req.Namespace)
		return &domain.RestoreRun{BackupRunID: req.BackupRunID, Status: domain.StatusSucceeded}, nil
	}

	res, err := uc.Run(context.Background(), RunRestoreSetRequest{SetID: set.SetID, OrgID: "org-1"})
	require.NoError(t, err)
	require.Len(t, calls, 3)
	assert.Equal(t, "platform_only:"+set.Runs[0].ID+":", calls[0], "플랫폼 DB·Keycloak 먼저")
	assert.Equal(t, "stack_only:"+set.Runs[1].ID+":nullus-a", calls[1], "스택은 매니페스트의 네임스페이스로")
	assert.Equal(t, "stack_only:"+set.Runs[2].ID+":nullus-b", calls[2])
	assert.Len(t, res.Restores, 3)
	assert.Empty(t, res.Failed)
}

// 플랫폼 복구가 실패하면 스택은 되돌리지 않는다 — 스택 복구는 플랫폼 DB 의 스택 기록(클러스터)을 본다.
func TestRunRestoreSet_플랫폼이_실패하면_멈춘다(t *testing.T) {
	buc, repo, _, _ := seedSet(t, []domain.StackRef{{ID: "stk_a", Namespace: "nullus-a"}})
	set, err := buc.Run(context.Background(), RunBackupSetRequest{OrgID: "org-1"})
	require.NoError(t, err)

	var calls int
	uc := NewRestoreSetUseCase(RestoreSetDeps{Repo: repo, Factory: func(context.Context, string) (*RestoreUseCase, error) {
		return &RestoreUseCase{}, nil
	}})
	uc.runner = func(_ context.Context, _ *RestoreUseCase, req RunRestoreRequest) (*domain.RestoreRun, error) {
		calls++
		return &domain.RestoreRun{BackupRunID: req.BackupRunID, Status: domain.StatusFailed}, errors.New("pg_restore 실패")
	}
	res, err := uc.Run(context.Background(), RunRestoreSetRequest{SetID: set.SetID})
	require.Error(t, err)
	assert.Equal(t, 1, calls, "플랫폼에서 멈춘다")
	require.Len(t, res.Failed, 1)
}

// 스택 하나의 복구 실패는 다른 스택을 막지 않는다.
func TestRunRestoreSet_스택_하나가_실패해도_다른_스택은_되돌린다(t *testing.T) {
	buc, repo, _, _ := seedSet(t, []domain.StackRef{{ID: "stk_a", Namespace: "nullus-a"}, {ID: "stk_b", Namespace: "nullus-b"}})
	set, err := buc.Run(context.Background(), RunBackupSetRequest{OrgID: "org-1"})
	require.NoError(t, err)

	var calls []string
	uc := NewRestoreSetUseCase(RestoreSetDeps{Repo: repo, Factory: func(context.Context, string) (*RestoreUseCase, error) {
		return &RestoreUseCase{}, nil
	}})
	uc.runner = func(_ context.Context, _ *RestoreUseCase, req RunRestoreRequest) (*domain.RestoreRun, error) {
		calls = append(calls, req.Namespace)
		if req.Namespace == "nullus-a" {
			return &domain.RestoreRun{BackupRunID: req.BackupRunID, Status: domain.StatusFailed}, errors.New("볼륨 복원 실패")
		}
		return &domain.RestoreRun{BackupRunID: req.BackupRunID, Status: domain.StatusSucceeded}, nil
	}
	res, err := uc.Run(context.Background(), RunRestoreSetRequest{SetID: set.SetID})
	require.NoError(t, err, "스택 실패는 결과에 남고 오류로 돌려주지 않는다")
	assert.Equal(t, []string{"", "nullus-a", "nullus-b"}, calls)
	require.Len(t, res.Failed, 1)
	assert.Contains(t, res.Failed[0], "nullus-a")
}

func TestRunRestoreSet_없는_세트(t *testing.T) {
	uc := NewRestoreSetUseCase(RestoreSetDeps{Repo: newFakeRepo()})
	_, err := uc.Run(context.Background(), RunRestoreSetRequest{SetID: "nope"})
	require.Error(t, err)
}
