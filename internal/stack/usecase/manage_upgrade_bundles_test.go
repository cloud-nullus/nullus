package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

func registerInput() usecase.RegisterUpgradeBundleInput {
	return usecase.RegisterUpgradeBundleInput{
		Tool: "argocd", ReleaseName: "argo-cd", ChartName: "argo-cd", RepoURL: "https://argoproj.github.io/argo-helm",
		SourceChartVersion: "7.7.16", SourceAppVersion: "v2.13.3", TargetChartVersion: "7.7.22", TargetAppVersion: "v2.13.4",
		RiskLevel: "low", ExpectedDisruption: "API 서버 재시작 약 1분", RequiresBackup: true,
		SupportedArchitectures: []string{"amd64", "arm64"}, Status: domain.UpgradeBundleVerified, RehearsalConfirmed: true,
	}
}

func newBundleFixture() (*usecase.ManageUpgradeBundles, *memUpgradeRepo) {
	repo := &memUpgradeRepo{bundles: []domain.UpgradeBundle{argoBundle("b-18", "7.7.18")}}
	uc := usecase.NewManageUpgradeBundles(repo, usecase.WithBundleClock(func() time.Time { return upNow }))
	return uc, repo
}

func TestManageUpgradeBundles_Register_PersistsVerifiedBundle(t *testing.T) {
	uc, repo := newBundleFixture()

	b, err := uc.Register(context.Background(), registerInput())
	require.NoError(t, err)
	assert.Equal(t, "argocd-7.7.16-7.7.22", b.ID)
	assert.Equal(t, domain.UpgradeBundleVerified, b.Status)
	assert.Equal(t, upNow, b.CreatedAt)

	stored, err := repo.GetVerifiedBundle(context.Background(), "argocd-7.7.16-7.7.22")
	require.NoError(t, err, "등록한 번들은 곧바로 후보 조회에 잡힌다")
	assert.Equal(t, "7.7.22", stored.TargetChartVersion)
}

func TestManageUpgradeBundles_Register_RejectsUnconfirmedVerified(t *testing.T) {
	uc, repo := newBundleFixture()
	in := registerInput()
	in.RehearsalConfirmed = false

	_, err := uc.Register(context.Background(), in)
	assert.ErrorIs(t, err, domain.ErrUpgradeBundleRehearsalRequired)
	assert.Len(t, repo.bundles, 1, "저장하지 않는다")
}

func TestManageUpgradeBundles_Register_DuplicateIsConflict(t *testing.T) {
	uc, _ := newBundleFixture()
	_, err := uc.Register(context.Background(), registerInput())
	require.NoError(t, err)

	_, err = uc.Register(context.Background(), registerInput())
	assert.ErrorIs(t, err, domain.ErrUpgradeBundleExists)
}

func TestManageUpgradeBundles_List_ReturnsEveryStatus(t *testing.T) {
	uc, repo := newBundleFixture()
	revoked := argoBundle("b-old", "7.7.17")
	revoked.Status = domain.UpgradeBundleRevoked
	repo.bundles = append(repo.bundles, revoked)

	items, err := uc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 2, "관리 목록은 verified 가 아닌 것도 보인다")
}

func TestManageUpgradeBundles_ChangeStatus(t *testing.T) {
	uc, repo := newBundleFixture()
	draft := argoBundle("b-draft", "7.7.21")
	draft.Status = domain.UpgradeBundleDraft
	repo.bundles = append(repo.bundles, draft)

	_, err := uc.ChangeStatus(context.Background(), "b-draft", domain.UpgradeBundleVerified, false)
	assert.ErrorIs(t, err, domain.ErrUpgradeBundleRehearsalRequired)

	b, err := uc.ChangeStatus(context.Background(), "b-draft", domain.UpgradeBundleVerified, true)
	require.NoError(t, err)
	assert.Equal(t, domain.UpgradeBundleVerified, b.Status)
	stored, err := repo.GetVerifiedBundle(context.Background(), "b-draft")
	require.NoError(t, err)
	assert.Equal(t, domain.UpgradeBundleVerified, stored.Status)

	b, err = uc.ChangeStatus(context.Background(), "b-18", domain.UpgradeBundleRevoked, false)
	require.NoError(t, err)
	assert.Equal(t, domain.UpgradeBundleRevoked, b.Status)
	_, err = repo.GetVerifiedBundle(context.Background(), "b-18")
	assert.ErrorIs(t, err, domain.ErrUpgradeBundleNotFound, "회수한 번들은 후보에서 빠진다")

	_, err = uc.ChangeStatus(context.Background(), "b-18", domain.UpgradeBundleVerified, true)
	assert.ErrorIs(t, err, domain.ErrUpgradeBundleStatusTransition, "회수는 끝이다")

	_, err = uc.ChangeStatus(context.Background(), "nope", domain.UpgradeBundleRevoked, false)
	assert.ErrorIs(t, err, domain.ErrUpgradeBundleUnknown)
}

// ── 실행 목록 ─────────────────────────────────────────────────────────

func TestUpgradeStack_ListRuns_NewestFirstIncludingBlockedAttempts(t *testing.T) {
	f := newUpgradeFixture(t)
	first := f.start(t, "key-1")
	f.runLaunched()
	f.now = f.now.Add(time.Hour)
	// 사전 점검에서 막힌 시도도 실행 목록에는 남는다 — 감사 기록이다.
	f.backups.found = false
	blocked := f.start(t, "key-2")
	require.Equal(t, domain.UpgradeStatusBlocked, blocked.Status)

	runs, err := f.uc.ListRuns(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, blocked.ID, runs[0].ID, "최근 것부터")
	assert.Equal(t, first.ID, runs[1].ID)
	assert.Equal(t, domain.UpgradeStatusSucceeded, runs[1].Status)
}

func TestUpgradeStack_ListRuns_ClosesStaleRun(t *testing.T) {
	f := newUpgradeFixture(t)
	run := f.start(t, "key-1")
	// 백그라운드를 돌리지 않은 채 박동이 끊긴 지 오래됐다.
	f.now = f.now.Add(domain.UpgradeHeartbeatTimeout + time.Minute)

	runs, err := f.uc.ListRuns(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, run.ID, runs[0].ID)
	assert.Equal(t, domain.UpgradeStatusManual, runs[0].Status)
}

func TestUpgradeStack_ListRuns_StackNotFound(t *testing.T) {
	f := newUpgradeFixture(t)
	_, err := f.uc.ListRuns(context.Background(), "stk_missing")
	assert.ErrorIs(t, err, usecase.ErrStackNotFound)
}

func TestUpgradeStack_ListRuns_EmptyIsEmptyList(t *testing.T) {
	f := newUpgradeFixture(t)
	runs, err := f.uc.ListRuns(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.NotNil(t, runs)
	assert.Empty(t, runs)
}
