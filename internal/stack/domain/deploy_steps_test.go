package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 2026-08-20 운영에서 나온 증상.
//
// 시크릿까지만 깔린 시점에 막대가 50% 를 넘겼다. 서버의 스텝→퍼센트 표에
// provisioning_secrets 가 없어 0 이 나왔고, 화면이 다른 근거로 값을 지어냈다.
func TestStepProgress_EarlyStepsStayEarly(t *testing.T) {
	secrets := StepProgress("provisioning_secrets")

	require.Greater(t, secrets, 0)
	assert.Less(t, secrets, 25, "시크릿은 초반 스텝이다 — 절반을 넘길 수 없다")
}

// 스텝 하나가 끝나면 딱 그만큼 오른다. 시간이 아니라 한 일에 맞춘다.
func TestStepProgress_RisesWithStepOrder(t *testing.T) {
	previous := StepProgress(InstallStepOrder[0])
	for _, step := range InstallStepOrder[1:] {
		current := StepProgress(step)
		assert.GreaterOrEqual(t, current, previous, "스텝 %s 에서 진행률이 뒤로 갔다", step)
		previous = current
	}
}

func TestStepProgress_StaysInsideInstallBand(t *testing.T) {
	for _, step := range InstallStepOrder {
		value := StepProgress(step)
		assert.GreaterOrEqual(t, value, 5)
		assert.Less(t, value, 90)
	}
}

func TestStepProgress_CoversLifecycleSteps(t *testing.T) {
	assert.Equal(t, 5, StepProgress("validate"))
	assert.Equal(t, 96, StepProgress("health_check"))
	assert.Equal(t, 100, StepProgress("completed"))
}

// 0 을 돌려주면 "아직 시작 전" 과 구분되지 않는다.
func TestStepProgress_UnknownStepIsNegative(t *testing.T) {
	assert.Equal(t, -1, StepProgress("no_such_step"))
	assert.Equal(t, -1, StepProgress(""))
}

// 화면은 이 값을 상한으로 삼아 스텝 안에서만 움직인다.
func TestStepProgressCeiling_IsTheNextStepsFloor(t *testing.T) {
	for index, step := range InstallStepOrder {
		ceiling := StepProgressCeiling(step)
		assert.Greater(t, ceiling, StepProgress(step), "스텝 %s", step)
		if index+1 < len(InstallStepOrder) {
			assert.Equal(t, StepProgress(InstallStepOrder[index+1]), ceiling)
		}
	}
}

// 새로고침해도 같은 값이어야 한다. 예전에는 화면이 상태를 뭉뚱그린 표로 채워서
// 새로고침할 때마다 퍼센트가 튀었다.
func TestStackProgress_MatchesTheStepBasedValue(t *testing.T) {
	progress, ceiling := StackProgress(StateInstalling, "provisioning_secrets", "installing_external_secrets")

	assert.Equal(t, StepProgress("provisioning_secrets"), progress)
	assert.Equal(t, StepProgressCeiling("provisioning_secrets"), ceiling)
}

// 진행 중인 스텝이 비면 마지막으로 끝낸 스텝까지는 갔다는 뜻이다.
func TestStackProgress_FallsBackToLastCompletedStep(t *testing.T) {
	progress, ceiling := StackProgress(StateInstalling, "", "installing_postgresql")

	assert.Equal(t, StepProgressCeiling("installing_postgresql"), progress)
	assert.Equal(t, progress, ceiling)
}

func TestStackProgress_CompletedIsAlwaysFull(t *testing.T) {
	progress, ceiling := StackProgress(StateCompleted, "", "")

	assert.Equal(t, 100, progress)
	assert.Equal(t, 100, ceiling)
}

func TestStackProgress_UnknownStateStartsAtZero(t *testing.T) {
	progress, ceiling := StackProgress(StatePending, "", "")

	assert.Equal(t, 0, progress)
	assert.Equal(t, 0, ceiling)
}
