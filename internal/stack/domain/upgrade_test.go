package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgradeT0 = time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

func argoBundle(id, target string) UpgradeBundle {
	return UpgradeBundle{ID: id, Tool: UpgradeToolArgoCD, ReleaseName: "argo-cd", ChartName: "argo-cd",
		SourceChartVersion: "7.7.16", TargetChartVersion: target, Status: UpgradeBundleVerified}
}

func passing() []UpgradeCheck {
	return []UpgradeCheck{PassedCheck("release_status", "deployed")}
}

func newRun(t *testing.T, checks []UpgradeCheck) *UpgradeRun {
	t.Helper()
	run, err := NewUpgradeRun(NewUpgradeRunInput{
		ID: "run-1", StackID: "stk_1", Bundle: argoBundle("b1", "7.7.20"),
		SourceChartVersion: "7.7.16", PreviousRevision: 3,
		RequestedBy: "devops@nullus", Reason: "CVE 패치", IdempotencyKey: "k1",
		Checks: checks, Now: upgradeT0,
	})
	require.NoError(t, err)
	return run
}

func TestSelectUpgradeBundle_PicksHighestSemverTarget(t *testing.T) {
	got := SelectUpgradeBundle([]UpgradeBundle{argoBundle("a", "7.7.18"), argoBundle("b", "7.8.0"), argoBundle("c", "7.7.20")})
	require.NotNil(t, got)
	assert.Equal(t, "b", got.ID)
}

func TestSelectUpgradeBundle_ParsableVersionBeatsUnparsable(t *testing.T) {
	got := SelectUpgradeBundle([]UpgradeBundle{argoBundle("x", "nightly"), argoBundle("y", "7.7.17")})
	require.NotNil(t, got)
	assert.Equal(t, "y", got.ID)
}

func TestSelectUpgradeBundle_NoneWhenEmpty(t *testing.T) {
	assert.Nil(t, SelectUpgradeBundle(nil))
}

// 검증 경로가 없다는 것은 "최신" 이 아니다 — 화면이 그렇게 말하면 운영자가 패치를 미룬다.
func TestUpgradeCandidate_NoBundleIsNotUpToDate(t *testing.T) {
	c := NewUpgradeCandidate(UpgradeToolArgoCD, "argo-cd", "7.7.16", "v2.13.3", 3, "deployed", nil, upgradeT0)
	assert.Nil(t, c.Bundle)
	assert.Empty(t, c.BlockedReason)
}

func TestUpgradeCandidate_NotDeployedReleaseIsBlocked(t *testing.T) {
	b := argoBundle("b1", "7.7.20")
	c := NewUpgradeCandidate(UpgradeToolArgoCD, "argo-cd", "7.7.16", "v2.13.3", 3, "pending-upgrade", &b, upgradeT0)
	assert.NotEmpty(t, c.BlockedReason)
}

func TestUpgradeChecks_Blocked(t *testing.T) {
	assert.False(t, UpgradeChecksBlocked(passing()))
	assert.True(t, UpgradeChecksBlocked(append(passing(), BlockedCheck("backup", "백업 없음", "백업을 먼저 받으세요"))))
	assert.False(t, UpgradeChecksBlocked([]UpgradeCheck{{Name: "w", Status: UpgradeCheckWarning}}))
}

func TestNewUpgradeRun_RequiresReasonAndIdempotencyKey(t *testing.T) {
	_, err := NewUpgradeRun(NewUpgradeRunInput{ID: "r", StackID: "s", Bundle: argoBundle("b", "7.7.20"), Reason: " ", IdempotencyKey: "k", Now: upgradeT0})
	assert.ErrorIs(t, err, ErrUpgradeReasonRequired)
	_, err = NewUpgradeRun(NewUpgradeRunInput{ID: "r", StackID: "s", Bundle: argoBundle("b", "7.7.20"), Reason: "x", IdempotencyKey: "", Now: upgradeT0})
	assert.ErrorIs(t, err, ErrUpgradeIdempotencyKeyRequired)
}

func TestNewUpgradeRun_PassingChecksStartUpgrading(t *testing.T) {
	run := newRun(t, passing())
	assert.Equal(t, UpgradeStatusUpgrading, run.Status)
	assert.Equal(t, UpgradeStepHelmUpgrade, run.CurrentStep)
	assert.True(t, run.Active())
	assert.False(t, run.Terminal())
	assert.Equal(t, "7.7.16", run.SourceChartVersion)
	assert.Equal(t, "7.7.20", run.TargetChartVersion)
	assert.Equal(t, 3, run.PreviousRevision)
	assert.Equal(t, upgradeT0, run.UpdatedAt)
	assert.Nil(t, run.FinishedAt)
}

// 차단된 실행도 남긴다 — 누가 언제 무엇을 시도했다가 왜 막혔는지가 감사 기록이다.
func TestNewUpgradeRun_BlockedChecksFinishImmediately(t *testing.T) {
	run := newRun(t, append(passing(), BlockedCheck("backup", "백업 없음", "")))
	assert.Equal(t, UpgradeStatusBlocked, run.Status)
	assert.True(t, run.Terminal())
	assert.False(t, run.Active())
	require.NotNil(t, run.FinishedAt)
	assert.NotEmpty(t, run.Error)
}

func TestUpgradeRun_SuccessPath(t *testing.T) {
	run := newRun(t, passing())
	run.EnterHealthCheck(4, upgradeT0.Add(time.Minute))
	assert.Equal(t, UpgradeStepHealthCheck, run.CurrentStep)
	assert.Equal(t, 4, run.ResultRevision)
	assert.True(t, run.Active())

	run.Succeed(upgradeT0.Add(2 * time.Minute))
	assert.Equal(t, UpgradeStatusSucceeded, run.Status)
	assert.Equal(t, UpgradeStepComplete, run.CurrentStep)
	assert.True(t, run.Terminal())
	require.NotNil(t, run.FinishedAt)
	assert.Equal(t, upgradeT0.Add(2*time.Minute), *run.FinishedAt)
}

func TestUpgradeRun_RollbackPath(t *testing.T) {
	run := newRun(t, passing())
	run.BeginRollback(errors.New("argocd-server not ready"), upgradeT0.Add(time.Minute))
	assert.Equal(t, UpgradeStatusRollingBack, run.Status)
	assert.Equal(t, UpgradeStepRollback, run.CurrentStep)
	assert.True(t, run.Active())
	assert.Contains(t, run.Error, "argocd-server not ready")

	run.RolledBack(upgradeT0.Add(2 * time.Minute))
	assert.Equal(t, UpgradeStatusRolledBack, run.Status)
	assert.True(t, run.Terminal())
	assert.Contains(t, run.Error, "argocd-server not ready", "원인은 복구 뒤에도 남는다")
}

func TestUpgradeRun_ManualInterventionKeepsBothCauses(t *testing.T) {
	run := newRun(t, passing())
	run.BeginRollback(errors.New("upgrade failed"), upgradeT0)
	run.RequireManualIntervention(errors.New("rollback failed"), upgradeT0.Add(time.Minute))
	assert.Equal(t, UpgradeStatusManual, run.Status)
	assert.True(t, run.Terminal())
	assert.Contains(t, run.Error, "upgrade failed")
	assert.Contains(t, run.Error, "rollback failed")
}

// API 가 작업 중에 죽으면 실행은 "진행 중" 으로 남아 그 스택의 업그레이드를 영원히 막는다.
// 마지막 갱신 뒤 한 단계의 최대 시간이 지나면 중단된 것으로 본다.
func TestUpgradeRun_StaleActiveRunIsInterrupted(t *testing.T) {
	run := newRun(t, passing())
	assert.False(t, run.Stale(upgradeT0.Add(UpgradeStepMaxDuration)))
	assert.True(t, run.Stale(upgradeT0.Add(UpgradeStepMaxDuration+time.Second)))

	run.Interrupt(upgradeT0.Add(UpgradeStepMaxDuration + time.Second))
	assert.Equal(t, UpgradeStatusManual, run.Status)
	assert.Contains(t, run.Error, "중단")

	done := newRun(t, passing())
	done.Succeed(upgradeT0)
	assert.False(t, done.Stale(upgradeT0.Add(24*time.Hour)), "끝난 실행은 중단될 수 없다")
}

func TestEvaluateUpgradeBackup(t *testing.T) {
	maxAge := 24 * time.Hour
	check := EvaluateUpgradeBackup(time.Time{}, false, upgradeT0, maxAge)
	assert.Equal(t, UpgradeCheckBlocked, check.Status)
	assert.NotEmpty(t, check.Remediation)

	check = EvaluateUpgradeBackup(upgradeT0.Add(-25*time.Hour), true, upgradeT0, maxAge)
	assert.Equal(t, UpgradeCheckBlocked, check.Status)
	assert.Contains(t, check.Message, "25h")

	check = EvaluateUpgradeBackup(upgradeT0.Add(-time.Hour), true, upgradeT0, maxAge)
	assert.Equal(t, UpgradeCheckPass, check.Status)
}
