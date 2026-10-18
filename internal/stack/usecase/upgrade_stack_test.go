package usecase_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stackrepo "github.com/cloud-nullus/draft/internal/stack/adapter/repository"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	"github.com/cloud-nullus/draft/internal/stack/port"
	"github.com/cloud-nullus/draft/internal/stack/usecase"
)

// ── 가짜들 ─────────────────────────────────────────────────────────────

type upgradeReleases struct {
	mu       sync.Mutex
	releases []port.ReleaseInfo
	values   map[string]any
	valueErr error
	listErr  error
}

func (f *upgradeReleases) ListReleases(context.Context, string) ([]port.ReleaseInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]port.ReleaseInfo(nil), f.releases...), nil
}

func (f *upgradeReleases) GetValues(context.Context, string, string) (map[string]any, error) {
	return f.values, f.valueErr
}

func (f *upgradeReleases) Upgrade(context.Context, port.HelmUpgradeRequest) (*port.HelmUpgradeResult, error) {
	return nil, errors.New("values 경로는 업그레이드가 쓰지 않는다")
}

func (f *upgradeReleases) set(status, chartVersion string, revision int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases[0].Status, f.releases[0].ChartVersion, f.releases[0].Revision = status, chartVersion, revision
}

type upgradeExecutor struct {
	releases    *upgradeReleases
	dryRuns     []port.ChartUpgradeRequest
	upgrades    []port.ChartUpgradeRequest
	upgradeCtx  []error
	dryRunErr   error
	upgradeErr  error
	resultState string
	// 업그레이드 실패 때 릴리스가 남는 모양. 기본은 대상 차트로 failed(리비전 4).
	atomicRestored bool // Helm --atomic 이 이미 출발 버전으로 되돌려 놓았다(리비전 5)
	untouched      bool // Helm 이 릴리스를 건드리기 전에 실패했다(리비전 3 그대로)
	pendingAfter   bool // 다른 작업이 릴리스를 잡고 있다(pending-upgrade)
	listErrAfter   error
	// block 이 있으면 업그레이드가 그 채널이 닫힐 때까지 멈춘다.
	block       chan struct{}
	rollbacks   []int
	rollbackErr error
}

func (f *upgradeExecutor) UpgradeChart(ctx context.Context, req port.ChartUpgradeRequest) (*port.ChartUpgradeResult, error) {
	if req.DryRun {
		f.dryRuns = append(f.dryRuns, req)
		return &port.ChartUpgradeResult{}, f.dryRunErr
	}
	f.upgrades = append(f.upgrades, req)
	f.upgradeCtx = append(f.upgradeCtx, ctx.Err())
	if f.block != nil {
		<-f.block
	}
	if f.upgradeErr != nil {
		switch {
		case f.atomicRestored:
			f.releases.set("deployed", "7.7.16", 5)
		case f.untouched:
		case f.pendingAfter:
			f.releases.set("pending-upgrade", "7.7.16", 4)
		default:
			f.releases.set("failed", req.Version, 4)
		}
		if f.listErrAfter != nil {
			f.releases.listErr = f.listErrAfter
		}
		return nil, f.upgradeErr
	}
	state := f.resultState
	if state == "" {
		state = "deployed"
	}
	f.releases.set(state, req.Version, 4)
	return &port.ChartUpgradeResult{Revision: 4, Status: state}, nil
}

func (f *upgradeExecutor) RollbackRelease(_ context.Context, _ string, _ string, revision int) error {
	f.rollbacks = append(f.rollbacks, revision)
	if f.rollbackErr != nil {
		return f.rollbackErr
	}
	f.releases.set("deployed", "7.7.16", 5)
	return nil
}

type upgradeHealth struct {
	verified []domain.UpgradeBundle
	errs     []error
	during   func()
}

func (f *upgradeHealth) Verify(_ context.Context, _ []byte, _ string, b domain.UpgradeBundle) error {
	f.verified = append(f.verified, b)
	if f.during != nil {
		f.during()
	}
	if len(f.errs) == 0 {
		return nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return err
}

type upgradeBackups struct {
	finishedAt time.Time
	found      bool
	err        error
}

func (f upgradeBackups) LatestRestorableBackup(context.Context, string) (time.Time, bool, error) {
	return f.finishedAt, f.found, f.err
}

type memUpgradeRepo struct {
	mu        sync.Mutex
	bundles   []domain.UpgradeBundle
	runs      []*domain.UpgradeRun
	createErr error
	updates   int
	// beforeCreate 는 저장 직전에 한 번 돈다 — 다른 요청이 먼저 저장한 경합을 흉내 낸다.
	beforeCreate func()
	// updateErrs 는 UpdateRun 이 차례로 돌려줄 일시 오류다.
	updateErrs []error
	heartbeats int
}

func clone(r *domain.UpgradeRun) *domain.UpgradeRun { c := *r; return &c }

func (m *memUpgradeRepo) ListVerifiedBundles(_ context.Context, tool, source string) ([]domain.UpgradeBundle, error) {
	var out []domain.UpgradeBundle
	for _, b := range m.bundles {
		if b.Tool == tool && b.SourceChartVersion == source && b.Status == domain.UpgradeBundleVerified {
			out = append(out, b)
		}
	}
	return out, nil
}

func (m *memUpgradeRepo) GetVerifiedBundle(_ context.Context, id string) (*domain.UpgradeBundle, error) {
	for _, b := range m.bundles {
		if b.ID == id && b.Status == domain.UpgradeBundleVerified {
			b := b
			return &b, nil
		}
	}
	return nil, domain.ErrUpgradeBundleNotFound
}

func (m *memUpgradeRepo) CreateRun(_ context.Context, run *domain.UpgradeRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.beforeCreate != nil {
		m.beforeCreate()
		m.beforeCreate = nil
	}
	if m.createErr != nil {
		return m.createErr
	}
	for _, r := range m.runs {
		if r.StackID == run.StackID && r.IdempotencyKey == run.IdempotencyKey {
			return port.ErrUpgradeIdempotencyConflict
		}
		if r.StackID == run.StackID && r.Active() && run.Active() {
			return domain.ErrUpgradeInProgress
		}
	}
	m.runs = append(m.runs, clone(run))
	return nil
}

func (m *memUpgradeRepo) UpdateRun(_ context.Context, run *domain.UpgradeRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates++
	if len(m.updateErrs) > 0 {
		err := m.updateErrs[0]
		m.updateErrs = m.updateErrs[1:]
		return err
	}
	for i, r := range m.runs {
		if r.ID == run.ID {
			if !r.Active() {
				return domain.ErrUpgradeRunClosed
			}
			m.runs[i] = clone(run)
			return nil
		}
	}
	return domain.ErrUpgradeRunNotFound
}

func (m *memUpgradeRepo) Heartbeat(_ context.Context, runID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runs {
		if r.ID == runID {
			if !r.Active() {
				return domain.ErrUpgradeRunClosed
			}
			r.UpdatedAt = at
			m.heartbeats++
			return nil
		}
	}
	return domain.ErrUpgradeRunNotFound
}

func (m *memUpgradeRepo) CloseStaleRun(_ context.Context, run *domain.UpgradeRun, staleBefore time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.runs {
		if r.ID == run.ID && r.Active() && r.UpdatedAt.Before(staleBefore) {
			m.runs[i] = clone(run)
			return true, nil
		}
	}
	return false, nil
}

// interrupt 는 다른 레플리카가 이 실행을 중단으로 닫은 것을 흉내 낸다.
func (m *memUpgradeRepo) interrupt(id string, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runs {
		if r.ID == id {
			r.Interrupt(at)
		}
	}
}

func (m *memUpgradeRepo) GetRun(_ context.Context, stackID, runID string) (*domain.UpgradeRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runs {
		if r.StackID == stackID && r.ID == runID {
			return clone(r), nil
		}
	}
	return nil, domain.ErrUpgradeRunNotFound
}

func (m *memUpgradeRepo) FindRunByIdempotencyKey(_ context.Context, stackID, key string) (*domain.UpgradeRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runs {
		if r.StackID == stackID && r.IdempotencyKey == key {
			return clone(r), nil
		}
	}
	return nil, nil
}

func (m *memUpgradeRepo) ActiveRun(_ context.Context, stackID string) (*domain.UpgradeRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runs {
		if r.StackID == stackID && r.Active() {
			return clone(r), nil
		}
	}
	return nil, nil
}

func (m *memUpgradeRepo) LatestRun(_ context.Context, stackID, tool string) (*domain.UpgradeRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.runs) - 1; i >= 0; i-- {
		if m.runs[i].StackID == stackID && m.runs[i].Tool == tool && m.runs[i].Status != domain.UpgradeStatusBlocked {
			return clone(m.runs[i]), nil
		}
	}
	return nil, nil
}

func (m *memUpgradeRepo) stored(id string) *domain.UpgradeRun {
	r, _ := m.GetRun(context.Background(), "stk_1", id)
	return r
}

// ── 조립 ─────────────────────────────────────────────────────────────

var upNow = time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

type upgradeFixture struct {
	uc       *usecase.UpgradeStack
	stacks   *stackrepo.MemoryStackRepository
	releases *upgradeReleases
	exec     *upgradeExecutor
	health   *upgradeHealth
	repo     *memUpgradeRepo
	backups  *upgradeBackups
	now      time.Time
	launched []func()
}

func argoBundle(id, target string) domain.UpgradeBundle {
	return domain.UpgradeBundle{ID: id, Tool: domain.UpgradeToolArgoCD, ReleaseName: "argo-cd", ChartName: "argo-cd",
		RepoURL: "https://argoproj.github.io/argo-helm", SourceChartVersion: "7.7.16", SourceAppVersion: "v2.13.3",
		TargetChartVersion: target, TargetAppVersion: "v2.13.5", RiskLevel: "low", RequiresBackup: true,
		Status: domain.UpgradeBundleVerified}
}

func newUpgradeFixture(t *testing.T) *upgradeFixture {
	t.Helper()
	f := &upgradeFixture{now: upNow}
	f.stacks = stackrepo.NewMemoryStackRepository()
	require.NoError(t, f.stacks.Create(context.Background(), &domain.Stack{
		ID: "stk_1", Name: "prod", ClusterID: "cls_1", Namespace: "nullus-prod", State: domain.StateCompleted,
	}))
	f.releases = &upgradeReleases{
		releases: []port.ReleaseInfo{
			{ReleaseName: "argo-cd", ChartName: "argo-cd", ChartVersion: "7.7.16", AppVersion: "v2.13.3", Revision: 3, Status: "deployed"},
			{ReleaseName: "gitlab", ChartName: "gitlab", ChartVersion: "8.0.0", Revision: 1, Status: "deployed"},
		},
		values: map[string]any{"server": map[string]any{"replicas": 1}},
	}
	f.exec = &upgradeExecutor{releases: f.releases}
	f.health = &upgradeHealth{}
	f.repo = &memUpgradeRepo{bundles: []domain.UpgradeBundle{argoBundle("b-18", "7.7.18"), argoBundle("b-20", "7.7.20")}}
	f.backups = &upgradeBackups{finishedAt: upNow.Add(-time.Hour), found: true}
	f.uc = f.rebuild(nil)
	return f
}

// rebuild 는 같은 가짜들로 유스케이스를 다시 조립한다. 바꿀 것만 change 에서 고친다.
func (f *upgradeFixture) rebuild(change func(d *usecase.UpgradeStackDeps)) *usecase.UpgradeStack {
	d := usecase.UpgradeStackDeps{
		Stacks:       f.stacks,
		Runs:         f.repo,
		Kubeconfigs:  fakeKubeconfigProvider{},
		Releases:     func([]byte) port.HelmReleaseManager { return f.releases },
		Executors:    func([]byte) port.ChartUpgradeExecutor { return f.exec },
		Backups:      f.backups,
		BackupMaxAge: 24 * time.Hour,
		Health:       f.health,
		Now:          func() time.Time { return f.now },
		Launch:       func(job func()) { f.launched = append(f.launched, job) },
		RetryBackoff: time.Nanosecond,
	}
	if change != nil {
		change(&d)
	}
	return usecase.NewUpgradeStack(d)
}

// runLaunched 는 백그라운드로 넘긴 실행을 지금 돌린다.
func (f *upgradeFixture) runLaunched() {
	jobs := f.launched
	f.launched = nil
	for _, job := range jobs {
		job()
	}
}

func (f *upgradeFixture) start(t *testing.T, key string) *domain.UpgradeRun {
	t.Helper()
	run, err := f.uc.Start(context.Background(), usecase.StartUpgradeInput{
		StackID: "stk_1", BundleID: "b-20", RequestedBy: "devops@nullus", Reason: "CVE 패치", IdempotencyKey: key,
	})
	require.NoError(t, err)
	return run
}

func checkByName(checks []domain.UpgradeCheck, name string) *domain.UpgradeCheck {
	for i := range checks {
		if checks[i].Name == name {
			return &checks[i]
		}
	}
	return nil
}

// ── 후보 ─────────────────────────────────────────────────────────────

func TestUpgradeStack_Candidates_ShowsInstalledArgoCDWithHighestVerifiedTarget(t *testing.T) {
	f := newUpgradeFixture(t)

	items, err := f.uc.Candidates(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, items, 1, "업그레이드 대상 도구의 릴리스만 나온다")
	c := items[0]
	assert.Equal(t, domain.UpgradeToolArgoCD, c.Tool)
	assert.Equal(t, "7.7.16", c.CurrentChartVersion)
	assert.Equal(t, "v2.13.3", c.CurrentAppVersion)
	require.NotNil(t, c.Bundle)
	assert.Equal(t, "b-20", c.Bundle.ID)
	assert.Empty(t, c.BlockedReason)
	assert.Equal(t, upNow, c.CheckedAt)
}

func TestUpgradeStack_Candidates_NoVerifiedPathHasNoBundle(t *testing.T) {
	f := newUpgradeFixture(t)
	f.repo.bundles = nil

	items, err := f.uc.Candidates(context.Background(), "stk_1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Nil(t, items[0].Bundle)
}

func TestUpgradeStack_Candidates_StackNotFound(t *testing.T) {
	f := newUpgradeFixture(t)
	_, err := f.uc.Candidates(context.Background(), "nope")
	assert.ErrorIs(t, err, usecase.ErrStackNotFound)
}

// 화면을 다시 열어도 진행 중인 실행을 이어 보여 준다(수용 기준 5).
func TestUpgradeStack_Candidates_IncludesLastRun(t *testing.T) {
	f := newUpgradeFixture(t)
	run := f.start(t, "k1")

	items, err := f.uc.Candidates(context.Background(), "stk_1")
	require.NoError(t, err)
	require.NotNil(t, items[0].LastRun)
	assert.Equal(t, run.ID, items[0].LastRun.ID)
	assert.Equal(t, domain.UpgradeStatusUpgrading, items[0].LastRun.Status)
}

// 진행 중에 들어온 다른 요청은 막힌 실행으로 남는다. 그 막힌 시도가 더 나중이어도 화면에는
// 진행 중인 실행을 보여야 한다 — 아니면 진행 표시가 사라지고 업그레이드 버튼이 다시 켜진다
// (실클러스터 리허설에서 발견).
func TestUpgradeStack_Candidates_PrefersActiveRunOverLaterBlockedAttempt(t *testing.T) {
	f := newUpgradeFixture(t)
	running := f.start(t, "k1")
	f.now = f.now.Add(time.Minute)
	blocked := f.start(t, "k2")
	require.Equal(t, domain.UpgradeStatusBlocked, blocked.Status)

	items, err := f.uc.Candidates(context.Background(), "stk_1")
	require.NoError(t, err)
	require.NotNil(t, items[0].LastRun)
	assert.Equal(t, running.ID, items[0].LastRun.ID)
	assert.Equal(t, domain.UpgradeStatusUpgrading, items[0].LastRun.Status)
}

// 막힌 시도는 그 자리(대화 상자)에서 이유를 보여 준다. 행의 "마지막 실행" 은 실제로 돈 실행이다 —
// 실패 리허설 중 들어온 막힌 시도가 "실패해 되돌렸다" 를 가리던 것을 실화면에서 보았다.
func TestUpgradeStack_Candidates_LastRunSkipsBlockedAttempts(t *testing.T) {
	f := newUpgradeFixture(t)
	f.health.errs = []error{errors.New("health failed")}
	failed := f.start(t, "k1")
	f.now = f.now.Add(time.Minute)
	f.start(t, "k2") // 진행 중이라 막힌다
	f.runLaunched()

	items, err := f.uc.Candidates(context.Background(), "stk_1")
	require.NoError(t, err)
	require.NotNil(t, items[0].LastRun)
	assert.Equal(t, failed.ID, items[0].LastRun.ID)
	assert.Equal(t, domain.UpgradeStatusRolledBack, items[0].LastRun.Status)
}

// API 가 작업 중에 죽은 실행은 오래 갱신이 없으면 중단으로 닫는다 — 아니면 그 스택은 영원히 막힌다.
func TestUpgradeStack_Candidates_ClosesStaleRun(t *testing.T) {
	f := newUpgradeFixture(t)
	run := f.start(t, "k1")
	f.now = f.now.Add(domain.UpgradeHeartbeatTimeout + time.Minute)

	items, err := f.uc.Candidates(context.Background(), "stk_1")
	require.NoError(t, err)
	assert.Equal(t, domain.UpgradeStatusManual, items[0].LastRun.Status)
	assert.Equal(t, domain.UpgradeStatusManual, f.repo.stored(run.ID).Status)
}

// ── 사전 점검 ──────────────────────────────────────────────────────────

func TestUpgradeStack_Preflight_AllPassRunsServerDryRunWithInstalledValues(t *testing.T) {
	f := newUpgradeFixture(t)

	checks, err := f.uc.Preflight(context.Background(), "stk_1", "b-20")
	require.NoError(t, err)
	assert.False(t, domain.UpgradeChecksBlocked(checks), "%+v", checks)
	for _, name := range []string{"stack_state", "active_run", "release_status", "version_path", "backup", "helm_dry_run"} {
		assert.NotNil(t, checkByName(checks, name), name)
	}
	require.Len(t, f.exec.dryRuns, 1)
	dry := f.exec.dryRuns[0]
	assert.Equal(t, "7.7.20", dry.Version)
	assert.Equal(t, "nullus-prod", dry.Namespace)
	assert.Equal(t, f.releases.values, dry.Values, "설치 때 넘긴 values 를 새 차트에 그대로 얹는다")
	assert.Empty(t, f.exec.upgrades, "사전 점검은 클러스터를 바꾸지 않는다")
}

func TestUpgradeStack_Preflight_Blocks(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *upgradeFixture)
		check string
	}{
		{"스택이 완료 상태가 아니다", func(f *upgradeFixture) {
			s, _ := f.stacks.GetByID(context.Background(), "stk_1")
			s.State = domain.StateInstalling
			_ = f.stacks.Update(context.Background(), s)
		}, "stack_state"},
		{"릴리스가 deployed 가 아니다", func(f *upgradeFixture) { f.releases.set("pending-upgrade", "7.7.16", 3) }, "release_status"},
		{"설치 버전이 번들 출발점과 다르다", func(f *upgradeFixture) { f.releases.set("deployed", "7.7.17", 3) }, "version_path"},
		{"백업이 없다", func(f *upgradeFixture) { f.backups.found = false }, "backup"},
		{"백업이 오래됐다", func(f *upgradeFixture) { f.backups.finishedAt = upNow.Add(-48 * time.Hour) }, "backup"},
		{"백업 조회 실패", func(f *upgradeFixture) { f.backups.err = errors.New("db down") }, "backup"},
		{"dry-run 실패", func(f *upgradeFixture) { f.exec.dryRunErr = errors.New("render failed") }, "helm_dry_run"},
		{"values 조회 실패", func(f *upgradeFixture) { f.releases.valueErr = errors.New("no values") }, "helm_dry_run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newUpgradeFixture(t)
			tc.setup(f)
			checks, err := f.uc.Preflight(context.Background(), "stk_1", "b-20")
			require.NoError(t, err)
			c := checkByName(checks, tc.check)
			require.NotNil(t, c, "%+v", checks)
			assert.Equal(t, domain.UpgradeCheckBlocked, c.Status, "%+v", c)
			assert.NotEmpty(t, c.Message)
		})
	}
}

// 백업 기능이 꺼져 있으면 안전 조건을 완화하지 않고 막는다(설계 §8).
func TestUpgradeStack_Preflight_BlocksWhenBackupModuleIsOff(t *testing.T) {
	f := newUpgradeFixture(t)
	f.uc = usecase.NewUpgradeStack(usecase.UpgradeStackDeps{
		Stacks: f.stacks, Runs: f.repo, Kubeconfigs: fakeKubeconfigProvider{},
		Releases:  func([]byte) port.HelmReleaseManager { return f.releases },
		Executors: func([]byte) port.ChartUpgradeExecutor { return f.exec },
		Health:    f.health, Now: func() time.Time { return f.now },
	})
	checks, err := f.uc.Preflight(context.Background(), "stk_1", "b-20")
	require.NoError(t, err)
	assert.Equal(t, domain.UpgradeCheckBlocked, checkByName(checks, "backup").Status)
}

func TestUpgradeStack_Preflight_SkipsDryRunWhenAlreadyBlocked(t *testing.T) {
	f := newUpgradeFixture(t)
	f.backups.found = false
	_, err := f.uc.Preflight(context.Background(), "stk_1", "b-20")
	require.NoError(t, err)
	assert.Empty(t, f.exec.dryRuns)
}

func TestUpgradeStack_Preflight_UnknownBundle(t *testing.T) {
	f := newUpgradeFixture(t)
	_, err := f.uc.Preflight(context.Background(), "stk_1", "nope")
	assert.ErrorIs(t, err, domain.ErrUpgradeBundleNotFound)
}

func TestUpgradeStack_Preflight_ReleaseMissing(t *testing.T) {
	f := newUpgradeFixture(t)
	f.releases.releases = f.releases.releases[1:]
	_, err := f.uc.Preflight(context.Background(), "stk_1", "b-20")
	assert.ErrorIs(t, err, domain.ErrUpgradeReleaseNotFound)
}

// ── 실행 ─────────────────────────────────────────────────────────────

func TestUpgradeStack_Start_RequiresReasonAndKeyBeforeTouchingCluster(t *testing.T) {
	f := newUpgradeFixture(t)
	_, err := f.uc.Start(context.Background(), usecase.StartUpgradeInput{StackID: "stk_1", BundleID: "b-20", Reason: " ", IdempotencyKey: "k"})
	assert.ErrorIs(t, err, domain.ErrUpgradeReasonRequired)
	_, err = f.uc.Start(context.Background(), usecase.StartUpgradeInput{StackID: "stk_1", BundleID: "b-20", Reason: "x"})
	assert.ErrorIs(t, err, domain.ErrUpgradeIdempotencyKeyRequired)
	assert.Empty(t, f.exec.dryRuns)
}

// 업그레이드는 요청 안에서 끝나지 않는다(Helm 대기 최대 30분). 실행을 만들어 돌려주고
// 백그라운드에서 이어 간다 — 브라우저를 닫아도 서버에서 끝난다(수용 기준 5).
func TestUpgradeStack_Start_ReturnsRunningRunAndFinishesInBackground(t *testing.T) {
	f := newUpgradeFixture(t)

	run := f.start(t, "k1")
	assert.Equal(t, domain.UpgradeStatusUpgrading, run.Status)
	assert.Equal(t, "devops@nullus", run.RequestedBy)
	assert.Equal(t, 3, run.PreviousRevision)
	assert.Empty(t, f.exec.upgrades, "요청 안에서는 Helm 을 돌리지 않는다")
	require.Len(t, f.launched, 1)

	f.runLaunched()
	require.Len(t, f.exec.upgrades, 1)
	up := f.exec.upgrades[0]
	assert.False(t, up.DryRun)
	assert.Equal(t, "7.7.20", up.Version)
	assert.Equal(t, f.releases.values, up.Values)

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusSucceeded, stored.Status)
	assert.Equal(t, 4, stored.ResultRevision)
	require.NotNil(t, stored.FinishedAt)
	require.Len(t, f.health.verified, 1)
	assert.Equal(t, "v2.13.5", f.health.verified[0].TargetAppVersion, "대상 버전으로 떴는지 본다")
}

// 요청이 끊겨도(ctx 취소) 백그라운드 실행은 계속된다.
func TestUpgradeStack_Start_BackgroundRunIgnoresRequestCancellation(t *testing.T) {
	f := newUpgradeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := f.uc.Start(ctx, usecase.StartUpgradeInput{StackID: "stk_1", BundleID: "b-20", Reason: "x", IdempotencyKey: "k1"})
	require.NoError(t, err)
	cancel()

	f.runLaunched()
	require.Len(t, f.exec.upgradeCtx, 1)
	assert.NoError(t, f.exec.upgradeCtx[0])
}

// 같은 키로 다시 부르면(타임아웃 뒤 재시도) 새 실행을 만들지 않는다(UPG-10).
func TestUpgradeStack_Start_SameIdempotencyKeyReturnsSameRun(t *testing.T) {
	f := newUpgradeFixture(t)
	first := f.start(t, "k1")
	f.runLaunched()

	again := f.start(t, "k1")
	assert.Equal(t, first.ID, again.ID)
	assert.Equal(t, domain.UpgradeStatusSucceeded, again.Status)
	assert.Empty(t, f.launched)
	assert.Len(t, f.exec.upgrades, 1)
}

// 키를 조회한 뒤 저장하기 전에 같은 키의 요청이 먼저 저장했으면, DB 가 가리고 그 실행을 돌려준다.
func TestUpgradeStack_Start_IdempotencyRaceReturnsWinner(t *testing.T) {
	f := newUpgradeFixture(t)
	winner := &domain.UpgradeRun{ID: "winner", StackID: "stk_1", BundleID: "b-20", Tool: domain.UpgradeToolArgoCD,
		IdempotencyKey: "k1", Status: domain.UpgradeStatusUpgrading, UpdatedAt: upNow}
	f.repo.beforeCreate = func() { f.repo.runs = append(f.repo.runs, winner) }

	run, err := f.uc.Start(context.Background(), usecase.StartUpgradeInput{StackID: "stk_1", BundleID: "b-20", Reason: "x", IdempotencyKey: "k1"})
	require.NoError(t, err)
	assert.Equal(t, "winner", run.ID)
	assert.Empty(t, f.launched)
}

// 동시에 들어온 두 요청 중 하나만 실행된다(수용 기준 4) — 늦은 쪽은 409 로 끝난다.
func TestUpgradeStack_Start_ConcurrentRunLosesWithInProgress(t *testing.T) {
	f := newUpgradeFixture(t)
	f.repo.createErr = domain.ErrUpgradeInProgress
	_, err := f.uc.Start(context.Background(), usecase.StartUpgradeInput{StackID: "stk_1", BundleID: "b-20", Reason: "x", IdempotencyKey: "k2"})
	assert.ErrorIs(t, err, domain.ErrUpgradeInProgress)
	assert.Empty(t, f.launched)
}

// 이미 진행 중인 실행이 있으면 사전 점검이 막고, 막힌 시도도 남긴다.
func TestUpgradeStack_Start_BlockedWhileAnotherRunIsActive(t *testing.T) {
	f := newUpgradeFixture(t)
	f.start(t, "k1")

	second := f.start(t, "k2")
	assert.Equal(t, domain.UpgradeStatusBlocked, second.Status)
	assert.Equal(t, domain.UpgradeCheckBlocked, checkByName(second.Checks, "active_run").Status)
	assert.Len(t, f.launched, 1, "막힌 실행은 백그라운드로 넘기지 않는다")
}

func TestUpgradeStack_Start_BlockedChecksAreRecordedWithoutUpgrading(t *testing.T) {
	f := newUpgradeFixture(t)
	f.backups.found = false

	run := f.start(t, "k1")
	assert.Equal(t, domain.UpgradeStatusBlocked, run.Status)
	assert.Empty(t, f.launched)
	assert.Equal(t, domain.UpgradeStatusBlocked, f.repo.stored(run.ID).Status)
}

// Helm --atomic 이 이미 되돌려 놓았으면 또 되돌리지 않는다 — 출발 버전으로 살아 있는지만 본다.
func TestUpgradeStack_Execute_HelmFailureRestoredByAtomic(t *testing.T) {
	f := newUpgradeFixture(t)
	f.exec.upgradeErr = errors.New("timed out waiting for the condition")
	f.exec.atomicRestored = true

	run := f.start(t, "k1")
	f.runLaunched()

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusRolledBack, stored.Status)
	assert.Contains(t, stored.Error, "timed out")
	assert.Empty(t, f.exec.rollbacks)
	require.Len(t, f.health.verified, 1)
	assert.Equal(t, "v2.13.3", f.health.verified[0].TargetAppVersion, "되돌린 뒤에는 출발 버전으로 확인한다")
	assert.Equal(t, "7.7.16", f.health.verified[0].TargetChartVersion)
}

func TestUpgradeStack_Execute_HelmFailureLeftReleaseFailedRollsBackExplicitly(t *testing.T) {
	f := newUpgradeFixture(t)
	f.exec.upgradeErr = errors.New("upgrade failed")

	run := f.start(t, "k1")
	f.runLaunched()

	assert.Equal(t, []int{3}, f.exec.rollbacks, "직전 리비전으로 되돌린다")
	assert.Equal(t, domain.UpgradeStatusRolledBack, f.repo.stored(run.ID).Status)
}

func TestUpgradeStack_Execute_NotDeployedResultRollsBack(t *testing.T) {
	f := newUpgradeFixture(t)
	f.exec.resultState = "failed"

	run := f.start(t, "k1")
	f.runLaunched()

	assert.Equal(t, []int{3}, f.exec.rollbacks)
	assert.Equal(t, domain.UpgradeStatusRolledBack, f.repo.stored(run.ID).Status)
}

// Helm 은 성공이라는데 도구가 대상 버전으로 살아 있지 않으면 되돌린다(UPG-07/08, 수용 기준 7).
func TestUpgradeStack_Execute_HealthFailureRollsBack(t *testing.T) {
	f := newUpgradeFixture(t)
	f.health.errs = []error{errors.New("argocd-server not ready (0/1)")}

	run := f.start(t, "k1")
	f.runLaunched()

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusRolledBack, stored.Status)
	assert.Contains(t, stored.Error, "argocd-server not ready")
	assert.Equal(t, 4, stored.ResultRevision)
	assert.Equal(t, []int{3}, f.exec.rollbacks)
	require.Len(t, f.health.verified, 2, "되돌린 뒤에도 같은 검사를 한다")
}

func TestUpgradeStack_Execute_RollbackFailureNeedsManualIntervention(t *testing.T) {
	f := newUpgradeFixture(t)
	f.health.errs = []error{errors.New("health failed")}
	f.exec.rollbackErr = errors.New("rollback failed")

	run := f.start(t, "k1")
	f.runLaunched()

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusManual, stored.Status)
	assert.Contains(t, stored.Error, "health failed")
	assert.Contains(t, stored.Error, "rollback failed")
}

func TestUpgradeStack_Execute_UnhealthyAfterRollbackNeedsManualIntervention(t *testing.T) {
	f := newUpgradeFixture(t)
	f.health.errs = []error{errors.New("health failed"), errors.New("still broken")}

	run := f.start(t, "k1")
	f.runLaunched()

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusManual, stored.Status)
	assert.Contains(t, stored.Error, "still broken")
}

// ── 조회 ─────────────────────────────────────────────────────────────

func TestUpgradeStack_GetRun(t *testing.T) {
	f := newUpgradeFixture(t)
	run := f.start(t, "k1")

	got, err := f.uc.GetRun(context.Background(), "stk_1", run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, got.ID)

	_, err = f.uc.GetRun(context.Background(), "stk_1", "nope")
	assert.ErrorIs(t, err, domain.ErrUpgradeRunNotFound)

	f.now = f.now.Add(domain.UpgradeHeartbeatTimeout + time.Minute)
	got, err = f.uc.GetRun(context.Background(), "stk_1", run.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UpgradeStatusManual, got.Status, "오래 갱신 없는 실행은 중단으로 닫힌다")
}

// ── 되돌리기 판단 · 수명 · 저장 (독립 리뷰 반영) ─────────────────────────────

// Helm 이 릴리스를 건드리기 전에 실패했으면(차트 내려받기 실패 등) 되돌릴 것이 없다 — 되돌리기를
// 부르면 직전 리비전으로 새 리비전을 하나 더 만들 뿐이다.
func TestUpgradeStack_Execute_FailureBeforeTouchingReleaseIsFailedWithoutRollback(t *testing.T) {
	f := newUpgradeFixture(t)
	f.exec.upgradeErr = errors.New("locate chart argo-cd: not found")
	f.exec.untouched = true

	run := f.start(t, "k1")
	f.runLaunched()

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusFailed, stored.Status)
	assert.Contains(t, stored.Error, "locate chart")
	assert.Empty(t, f.exec.rollbacks)
	assert.Empty(t, f.health.verified)
}

// 실패 뒤 릴리스를 다른 작업(값 편집 · 복구)이 잡고 있으면 되돌리지 않는다 — 되돌리기가 그 작업을 지운다.
func TestUpgradeStack_Execute_PendingReleaseAfterFailureIsNotRolledBack(t *testing.T) {
	f := newUpgradeFixture(t)
	f.exec.upgradeErr = errors.New("another operation (install/upgrade/rollback) is in progress")
	f.exec.pendingAfter = true

	run := f.start(t, "k1")
	f.runLaunched()

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusManual, stored.Status)
	assert.Contains(t, stored.Error, "pending-upgrade")
	assert.Empty(t, f.exec.rollbacks)
}

func TestUpgradeStack_Execute_UnknownReleaseStateIsNotRolledBack(t *testing.T) {
	f := newUpgradeFixture(t)
	f.exec.upgradeErr = errors.New("upgrade failed")
	f.exec.listErrAfter = errors.New("connection refused")

	run := f.start(t, "k1")
	f.runLaunched()

	assert.Equal(t, domain.UpgradeStatusManual, f.repo.stored(run.ID).Status)
	assert.Empty(t, f.exec.rollbacks)
}

// API 가 종료되면(수명 ctx 취소) Helm 은 --atomic 으로 스스로 되돌리기 시작한다. 그 위에서 또
// 되돌리지 않고 사람에게 넘긴다.
func TestUpgradeStack_Execute_ShutdownHandsOverWithoutRollback(t *testing.T) {
	f := newUpgradeFixture(t)
	life, stop := context.WithCancel(context.Background())
	f.uc = f.rebuild(func(d *usecase.UpgradeStackDeps) { d.Context = life })
	f.exec.upgradeErr = context.Canceled

	run := f.start(t, "k1")
	stop()
	f.runLaunched()

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusManual, stored.Status)
	assert.Contains(t, stored.Error, "종료")
	assert.Empty(t, f.exec.rollbacks)
}

// 다른 곳이 실행을 중단으로 닫았으면 백그라운드는 거기서 멈춘다 — 닫힌 실행을 되살리거나,
// 그 사이 시작된 새 실행 위로 되돌리기를 하지 않는다.
func TestUpgradeStack_Execute_StopsWhenRunWasClosedElsewhere(t *testing.T) {
	f := newUpgradeFixture(t)
	run := f.start(t, "k1")
	f.health.errs = []error{errors.New("not ready")}
	f.health.during = func() { f.repo.interrupt(run.ID, upNow.Add(time.Hour)) }

	f.runLaunched()

	stored := f.repo.stored(run.ID)
	assert.Equal(t, domain.UpgradeStatusManual, stored.Status)
	assert.Contains(t, stored.Error, "중단")
	assert.Empty(t, f.exec.rollbacks, "닫힌 실행은 더 손대지 않는다")
}

// 실행 중에는 박동을 남긴다 — Helm 이 오래 걸려도 다른 레플리카가 중단으로 닫지 않는다.
func TestUpgradeStack_Execute_SendsHeartbeatsWhileHelmRuns(t *testing.T) {
	f := newUpgradeFixture(t)
	f.uc = f.rebuild(func(d *usecase.UpgradeStackDeps) {
		d.HeartbeatInterval = time.Millisecond
		d.Launch = nil // 진짜 goroutine
	})
	f.exec.block = make(chan struct{})

	run := f.start(t, "k1")
	require.Eventually(t, func() bool {
		f.repo.mu.Lock()
		defer f.repo.mu.Unlock()
		return f.repo.heartbeats >= 3
	}, time.Second, time.Millisecond)
	close(f.exec.block)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, f.uc.Wait(ctx))
	assert.Equal(t, domain.UpgradeStatusSucceeded, f.repo.stored(run.ID).Status)
}

// 끝 상태 저장이 잠깐 실패해도 다시 시도한다 — 성공한 업그레이드가 박동 끊김으로 "중단" 이 되지 않게.
func TestUpgradeStack_Execute_RetriesTransientPersistFailure(t *testing.T) {
	f := newUpgradeFixture(t)
	run := f.start(t, "k1")
	f.repo.updateErrs = []error{errors.New("conn reset"), errors.New("conn reset")}

	f.runLaunched()
	assert.Equal(t, domain.UpgradeStatusSucceeded, f.repo.stored(run.ID).Status)
}

func TestUpgradeStack_Start_RejectsKeyReusedForAnotherBundle(t *testing.T) {
	f := newUpgradeFixture(t)
	f.start(t, "k1")
	_, err := f.uc.Start(context.Background(), usecase.StartUpgradeInput{StackID: "stk_1", BundleID: "b-18", Reason: "x", IdempotencyKey: "k1"})
	assert.ErrorIs(t, err, domain.ErrUpgradeIdempotencyKeyReused)
}

func TestUpgradeStack_Start_RejectsOversizedKeyBeforeTouchingCluster(t *testing.T) {
	f := newUpgradeFixture(t)
	_, err := f.uc.Start(context.Background(), usecase.StartUpgradeInput{StackID: "stk_1", BundleID: "b-20", Reason: "x",
		IdempotencyKey: strings.Repeat("k", domain.MaxUpgradeIdempotencyKeyLength+1)})
	assert.ErrorIs(t, err, domain.ErrUpgradeIdempotencyKeyInvalid)
	assert.Empty(t, f.exec.dryRuns)
}

// 진짜 goroutine 경로 — 돌려준 실행과 백그라운드 사본이 엉키지 않는지 -race 로 본다.
func TestUpgradeStack_Start_RealBackgroundPath(t *testing.T) {
	f := newUpgradeFixture(t)
	f.uc = f.rebuild(func(d *usecase.UpgradeStackDeps) { d.Launch = nil })

	run := f.start(t, "k1")
	assert.Equal(t, domain.UpgradeStatusUpgrading, run.Status)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, f.uc.Wait(ctx))
	assert.Equal(t, domain.UpgradeStatusUpgrading, run.Status, "돌려준 실행은 백그라운드가 바꾸지 않는다")
	assert.Equal(t, domain.UpgradeStatusSucceeded, f.repo.stored(run.ID).Status)
}
