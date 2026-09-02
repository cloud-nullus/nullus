package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupRun_상태전이(t *testing.T) {
	run := NewBackupRun("org-1", ModeFull, TriggerManual, []Component{ComponentPlatformDB, ComponentVolume})
	assert.Equal(t, StatusPending, run.Status)

	require.NoError(t, run.Start())
	assert.Equal(t, StatusRunning, run.Status)
	assert.NotNil(t, run.StartedAt)

	assert.Error(t, run.Start(), "running 을 다시 start 할 수 없다")
}

func TestBackupRun_Complete_전부_성공하면_succeeded(t *testing.T) {
	run := NewBackupRun("org-1", ModeFull, TriggerManual, []Component{ComponentPlatformDB, ComponentVolume})
	require.NoError(t, run.Start())

	run.Complete(map[Component]error{ComponentPlatformDB: nil, ComponentVolume: nil})

	assert.Equal(t, StatusSucceeded, run.Status)
	assert.Empty(t, run.Error)
	assert.NotNil(t, run.FinishedAt)
}

func TestBackupRun_Complete_일부만_성공하면_partial(t *testing.T) {
	// partial 이 failed 와 구분돼야 하는 이유: "부분적으로 쓸 수 있는 백업" 과
	// "아무것도 없는 상태" 는 복구 시점에 결정적으로 다르다.
	run := NewBackupRun("org-1", ModeFull, TriggerManual, []Component{ComponentPlatformDB, ComponentVolume})
	require.NoError(t, run.Start())

	run.Complete(map[Component]error{
		ComponentPlatformDB: nil,
		ComponentVolume:     assert.AnError,
	})

	assert.Equal(t, StatusPartial, run.Status)
	assert.Contains(t, run.Error, string(ComponentVolume))
}

func TestBackupRun_Complete_전부_실패하면_failed(t *testing.T) {
	run := NewBackupRun("org-1", ModeFull, TriggerManual, []Component{ComponentPlatformDB, ComponentVolume})
	require.NoError(t, run.Start())

	run.Complete(map[Component]error{
		ComponentPlatformDB: assert.AnError,
		ComponentVolume:     assert.AnError,
	})

	assert.Equal(t, StatusFailed, run.Status)
}

func TestBackupRun_Complete_결과가_없으면_failed(t *testing.T) {
	// 아무것도 시도하지 못한 것은 성공이 아니다.
	run := NewBackupRun("org-1", ModeFull, TriggerManual, []Component{ComponentPlatformDB})
	require.NoError(t, run.Start())

	run.Complete(nil)

	assert.Equal(t, StatusFailed, run.Status)
}

func TestBackupRun_정지창_기록(t *testing.T) {
	run := NewBackupRun("org-1", ModeStackOnly, TriggerSchedule, []Component{ComponentVolume})
	require.NoError(t, run.Start())

	run.BeginQuiesce()
	require.NotNil(t, run.QuiesceStartedAt)
	run.EndQuiesce()
	require.NotNil(t, run.QuiesceEndedAt)

	assert.False(t, run.QuiesceEndedAt.Before(*run.QuiesceStartedAt))
}

func TestModeComponents_모드별_대상(t *testing.T) {
	assert.ElementsMatch(t,
		[]Component{ComponentPlatformDB, ComponentKeycloakDB, ComponentOpenBaoKV},
		ModeComponents(ModePlatformOnly))

	assert.ElementsMatch(t,
		[]Component{ComponentNamespaceResources, ComponentVolume},
		ModeComponents(ModeStackOnly))

	assert.Len(t, ModeComponents(ModeFull), 5, "full 은 다섯 컴포넌트를 모두 담는다")
}

func TestBackupRun_RequiresQuiesce(t *testing.T) {
	// 볼륨을 뜨지 않는 백업은 정지 창을 열 이유가 없다 — 무중단이어야 한다.
	assert.False(t, NewBackupRun("o", ModePlatformOnly, TriggerManual, ModeComponents(ModePlatformOnly)).RequiresQuiesce())
	assert.True(t, NewBackupRun("o", ModeStackOnly, TriggerManual, ModeComponents(ModeStackOnly)).RequiresQuiesce())
	assert.True(t, NewBackupRun("o", ModeFull, TriggerManual, ModeComponents(ModeFull)).RequiresQuiesce())
}
