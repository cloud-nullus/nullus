package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 백업 세트 — 플랫폼 전체를 한 번에 뜬다.
//
// 한 번의 백업은 플랫폼 + 스택 하나였다. 스택이 여럿이면 백업 시각이 어긋나고, 스택 A 의 full
// 복구가 플랫폼 DB 를 A 시점으로 되돌려 그 뒤에 만든 스택 B 의 기록을 지운다. 세트는 플랫폼
// 한 번 + 스택마다 한 번을 같은 ID 로 묶어, 복구가 플랫폼 → 스택 순서로 되돌릴 수 있게 한다.
func TestPlanBackupSet_플랫폼_먼저_그_다음_스택마다_하나(t *testing.T) {
	members := PlanBackupSet([]StackRef{
		{ID: "stk_b", Namespace: "nullus-b"},
		{ID: "stk_a", Namespace: "nullus-a"},
	})
	require.Len(t, members, 3)

	assert.Equal(t, "", members[0].StackID, "플랫폼이 먼저다 — DB 가 스택보다 오래된 쪽이 안전하다(복구는 플랫폼 → 스택)")
	assert.Equal(t, ModePlatformOnly, members[0].Mode)
	assert.Equal(t, ModeComponents(ModePlatformOnly), members[0].Scope)

	assert.Equal(t, []string{"stk_a", "stk_b"}, []string{members[1].StackID, members[2].StackID}, "스택은 ID 순 — 같은 입력이면 같은 계획")
	assert.Equal(t, "nullus-a", members[1].Namespace)
	assert.Equal(t, ModeStackOnly, members[1].Mode)
	// 스택 금고는 모드 분류상 "플랫폼" 쪽이지만 스택마다 배포된다 — 스택 멤버가 담아야 한다.
	assert.Contains(t, members[1].Scope, ComponentOpenBaoKV)
	assert.Contains(t, members[1].Scope, ComponentVolume)
	assert.Contains(t, members[1].Scope, ComponentNamespaceResources)
	assert.Contains(t, members[1].Scope, ComponentSharedResources)
	assert.NotContains(t, members[1].Scope, ComponentPlatformDB, "플랫폼 DB 는 플랫폼 멤버가 한 번만 뜬다")
}

func TestPlanBackupSet_스택이_없으면_플랫폼만(t *testing.T) {
	members := PlanBackupSet(nil)
	require.Len(t, members, 1)
	assert.Equal(t, ModePlatformOnly, members[0].Mode)
}

// 세트의 상태는 멤버에서 파생된다. partial 이 따로 있는 이유는 백업 실행과 같다 — "일부는 쓸 수
// 있는 세트" 와 "아무것도 없는 세트" 가 복구 시점에 다르다.
func TestSetStatus(t *testing.T) {
	mk := func(s Status) *BackupRun { return &BackupRun{Status: s} }
	assert.Equal(t, StatusSucceeded, SetStatus([]*BackupRun{mk(StatusSucceeded), mk(StatusSucceeded)}))
	assert.Equal(t, StatusPartial, SetStatus([]*BackupRun{mk(StatusSucceeded), mk(StatusFailed)}))
	assert.Equal(t, StatusPartial, SetStatus([]*BackupRun{mk(StatusPartial), mk(StatusSucceeded)}))
	assert.Equal(t, StatusFailed, SetStatus([]*BackupRun{mk(StatusFailed), mk(StatusFailed)}))
	assert.Equal(t, StatusRunning, SetStatus([]*BackupRun{mk(StatusSucceeded), mk(StatusRunning)}))
	assert.Equal(t, StatusFailed, SetStatus(nil), "멤버가 하나도 없으면 뜬 것이 없다")
}

// 복구는 플랫폼을 먼저, 스택을 그 다음에 되돌린다. 스택 복구가 플랫폼 DB 를 다시 덮으면 안 되므로
// 스택은 stack_only 다. 네임스페이스는 백업 매니페스트가 안다.
func TestPlanSetRestore_플랫폼_platform_only_먼저_스택은_stack_only(t *testing.T) {
	runs := []*BackupRun{
		{ID: "r-b", StackID: "stk_b", Status: StatusSucceeded, Manifest: map[string]any{"namespace": "nullus-b"}},
		{ID: "r-p", StackID: "", Status: StatusSucceeded},
		{ID: "r-a", StackID: "stk_a", Status: StatusSucceeded, Manifest: map[string]any{"namespace": "nullus-a"}},
		{ID: "r-x", StackID: "stk_x", Status: StatusFailed, Manifest: map[string]any{"namespace": "nullus-x"}},
	}
	steps, skipped := PlanSetRestore(runs)
	require.Len(t, steps, 3)
	assert.Equal(t, "r-p", steps[0].BackupRunID)
	assert.Equal(t, ModePlatformOnly, steps[0].Mode)
	assert.Equal(t, []string{"r-a", "r-b"}, []string{steps[1].BackupRunID, steps[2].BackupRunID})
	assert.Equal(t, ModeStackOnly, steps[1].Mode)
	assert.Equal(t, "nullus-a", steps[1].Namespace)
	assert.Equal(t, "stk_a", steps[1].StackID)
	assert.Equal(t, []string{"r-x"}, skipped, "실패한 멤버는 되돌릴 것이 없다 — 건너뛰고 알린다")
}

func TestPlanSetRestore_플랫폼_멤버가_없으면_스택만(t *testing.T) {
	steps, _ := PlanSetRestore([]*BackupRun{
		{ID: "r-a", StackID: "stk_a", Status: StatusPartial, Manifest: map[string]any{"namespace": "nullus-a"}},
	})
	require.Len(t, steps, 1)
	assert.Equal(t, ModeStackOnly, steps[0].Mode, "partial 도 쓸 수 있는 산출물이 있다")
}
