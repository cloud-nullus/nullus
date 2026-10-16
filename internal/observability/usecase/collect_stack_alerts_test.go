package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/observability/domain"
	sharednotify "github.com/cloud-nullus/draft/internal/shared/notification"
)

// ── 가짜들

type fakeMonitoredStacks struct{ stacks []domain.MonitoredStack }

func (f *fakeMonitoredStacks) ListMonitoredStacks(context.Context) ([]domain.MonitoredStack, error) {
	return f.stacks, nil
}

type fakeAlertSource struct {
	alerts map[string][]domain.SourceAlert
	errs   map[string]error
}

func (f *fakeAlertSource) ActiveAlerts(_ context.Context, s domain.MonitoredStack) ([]domain.SourceAlert, error) {
	if err := f.errs[s.ID]; err != nil {
		return nil, err
	}
	return f.alerts[s.ID], nil
}

type fakeStackAlertRepo struct {
	mu     sync.Mutex
	seq    int
	alerts map[string]*domain.StackAlert
}

func newFakeStackAlertRepo() *fakeStackAlertRepo {
	return &fakeStackAlertRepo{alerts: map[string]*domain.StackAlert{}}
}

func (r *fakeStackAlertRepo) open(stackID, fp string) *domain.StackAlert {
	for _, a := range r.alerts {
		if a.StackID == stackID && a.Fingerprint == fp && a.ResolvedAt == nil {
			return a
		}
	}
	return nil
}

func (r *fakeStackAlertRepo) Upsert(_ context.Context, a *domain.StackAlert) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur := r.open(a.StackID, a.Fingerprint); cur != nil {
		cur.LastSeenAt = a.LastSeenAt
		a.ID = cur.ID
		return false, nil
	}
	r.seq++
	a.ID = fmt.Sprintf("al-%d", r.seq)
	copied := *a
	r.alerts[a.ID] = &copied
	return true, nil
}

func (r *fakeStackAlertRepo) ClaimNotification(_ context.Context, id string, now time.Time, repeat time.Duration) (bool, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.alerts[id]
	if a == nil || a.ResolvedAt != nil {
		return false, false, nil
	}
	if a.LastNotifiedAt != nil && now.Sub(*a.LastNotifiedAt) < repeat {
		return false, false, nil
	}
	first := a.LastNotifiedAt == nil
	a.LastNotifiedAt = &now
	return true, first, nil
}

func (r *fakeStackAlertRepo) ListOpen(_ context.Context, stackID string) ([]*domain.StackAlert, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.StackAlert
	for _, a := range r.alerts {
		if a.StackID == stackID && a.ResolvedAt == nil {
			c := *a
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeStackAlertRepo) Resolve(_ context.Context, id string, at, staleBefore time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.alerts[id]
	if a == nil || a.ResolvedAt != nil || !a.LastSeenAt.Before(staleBefore) {
		return false, nil
	}
	a.ResolvedAt = &at
	return true, nil
}

func (r *fakeStackAlertRepo) CloseForMissingStacks(_ context.Context, keep []string, at, seenBefore time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := map[string]bool{}
	for _, k := range keep {
		kept[k] = true
	}
	n := 0
	for _, a := range r.alerts {
		if !kept[a.StackID] && a.ResolvedAt == nil && a.LastSeenAt.Before(seenBefore) {
			a.ResolvedAt = &at
			n++
		}
	}
	return n, nil
}

func (r *fakeStackAlertRepo) List(_ context.Context, orgID string, openOnly bool, limit int) ([]*domain.StackAlert, error) {
	return nil, nil
}

type recordingPublisher struct {
	mu      sync.Mutex
	events  []sharednotify.Event
	err     error
	ctxErrs []error
}

func (p *recordingPublisher) Publish(ctx context.Context, e sharednotify.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, e)
	p.ctxErrs = append(p.ctxErrs, ctx.Err())
	return p.err
}

type fakeLock struct {
	acquire  bool
	released int
}

func (l *fakeLock) TryAcquire(context.Context) (func(), bool, error) {
	if !l.acquire {
		return nil, false, nil
	}
	return func() { l.released++ }, true, nil
}

func (p *recordingPublisher) titles() []string {
	var out []string
	for _, e := range p.events {
		out = append(out, string(e.Type)+" "+e.Title)
	}
	sort.Strings(out)
	return out
}

// ── 시험

var stackA = domain.MonitoredStack{ID: "stk_a", OrgID: "org-1", Name: "gitlab", Namespace: "nullus-gitlab", ClusterID: "c1"}
var stackB = domain.MonitoredStack{ID: "stk_b", OrgID: "org-2", Name: "lite", Namespace: "nullus-lite", ClusterID: "c2"}

func crit(name string) domain.SourceAlert {
	return domain.SourceAlert{
		Fingerprint: "fp-" + name,
		Labels:      map[string]string{"alertname": name, "severity": "critical"},
		Annotations: map[string]string{"summary": name + " summary"},
		StartsAt:    time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC),
	}
}

type collectHarness struct {
	source *fakeAlertSource
	repo   *fakeStackAlertRepo
	pub    *recordingPublisher
	now    time.Time
	uc     *CollectStackAlerts
}

func newCollectHarness(stacks ...domain.MonitoredStack) *collectHarness {
	h := &collectHarness{
		source: &fakeAlertSource{alerts: map[string][]domain.SourceAlert{}, errs: map[string]error{}},
		repo:   newFakeStackAlertRepo(),
		pub:    &recordingPublisher{},
		now:    time.Date(2026, 10, 13, 2, 0, 0, 0, time.UTC),
	}
	h.uc = NewCollectStackAlerts(&fakeMonitoredStacks{stacks: stacks}, h.source, h.repo, h.pub)
	h.uc.now = func() time.Time { return h.now }
	return h
}

func (h *collectHarness) run(t *testing.T) CollectResult {
	t.Helper()
	res, err := h.uc.Run(context.Background())
	require.NoError(t, err)
	return res
}

func TestCollectStackAlerts_NotifiesNewCriticalOnceAndSkipsNoise(t *testing.T) {
	h := newCollectHarness(stackA)
	noise := crit("KubeSchedulerDown")
	warn := crit("KubePodCrashLooping")
	warn.Labels["severity"] = "warning"
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("NodeFilesystemAlmostOutOfSpace"), noise, warn}

	res := h.run(t)
	assert.Equal(t, 1, res.Fired)
	assert.Equal(t, []string{"alert_fired NodeFilesystemAlmostOutOfSpace — gitlab"}, h.pub.titles())
	assert.Equal(t, "org-1", h.pub.events[0].OrgID)

	h.now = h.now.Add(time.Minute)
	res = h.run(t)
	assert.Zero(t, res.Fired, "같은 경보를 매번 알리지 않는다")
	assert.Len(t, h.pub.events, 1)
}

// 계속 발생 중이면 다시 알린다(기본 4시간).
func TestCollectStackAlerts_RepeatsWhileStillFiring(t *testing.T) {
	h := newCollectHarness(stackA)
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	h.run(t)

	h.now = h.now.Add(DefaultAlertRepeatInterval - time.Minute)
	h.run(t)
	require.Len(t, h.pub.events, 1)

	h.now = h.now.Add(2 * time.Minute)
	res := h.run(t)
	assert.Equal(t, 1, res.Repeated)
	require.Len(t, h.pub.events, 2)
	assert.Equal(t, "alert_fired 계속 발생 중: KubePodNotReady — gitlab", h.pub.titles()[1])
}

func TestCollectStackAlerts_ResolvesWhenGone(t *testing.T) {
	h := newCollectHarness(stackA)
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	h.run(t)

	h.source.alerts["stk_a"] = nil
	h.now = h.now.Add(10 * time.Minute)
	res := h.run(t)

	assert.Equal(t, 1, res.Resolved)
	require.Len(t, h.pub.events, 2)
	assert.Equal(t, "alert_resolved 해제: KubePodNotReady — gitlab", h.pub.titles()[1])
	open, _ := h.repo.ListOpen(context.Background(), "stk_a")
	assert.Empty(t, open)

	// 다시 뜨면 새 발생이다.
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	h.now = h.now.Add(time.Minute)
	assert.Equal(t, 1, h.run(t).Fired)
}

// Alertmanager 에 닿지 못하면 아무것도 해제하지 않는다 — 모른다는 것과 없다는 것은 다르다.
func TestCollectStackAlerts_UnreachableStackKeepsOpenAlerts(t *testing.T) {
	h := newCollectHarness(stackA, stackB)
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	h.source.alerts["stk_b"] = []domain.SourceAlert{crit("NodeFilesystemAlmostOutOfSpace")}
	h.run(t)

	h.source.errs["stk_a"] = errors.New("service unavailable")
	h.source.alerts["stk_b"] = nil
	h.now = h.now.Add(DefaultResolveGrace + time.Minute)
	res := h.run(t)

	assert.Equal(t, 1, res.Unreachable)
	assert.Equal(t, 1, res.Resolved, "닿은 스택은 그대로 처리한다")
	open, _ := h.repo.ListOpen(context.Background(), "stk_a")
	assert.Len(t, open, 1)
}

// 지워진 스택의 열린 경보는 알리지 않고 닫는다.
func TestCollectStackAlerts_ClosesAlertsOfVanishedStacksSilently(t *testing.T) {
	h := newCollectHarness(stackA)
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	h.run(t)

	h.uc.stacks = &fakeMonitoredStacks{}
	h.now = h.now.Add(time.Minute)
	res := h.run(t)

	assert.Equal(t, 1, res.Closed)
	assert.Len(t, h.pub.events, 1, "해제 알림을 보내지 않는다 — 스택이 없어진 것이다")
}

// 발송이 실패해도 기록은 남고 다음 경보는 계속 처리한다.
func TestCollectStackAlerts_PublishFailureDoesNotStopCollection(t *testing.T) {
	h := newCollectHarness(stackA)
	h.pub.err = errors.New("no channel reachable")
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("A"), crit("B")}
	res := h.run(t)
	assert.Equal(t, 2, res.Fired)
	assert.Len(t, h.pub.events, 2)
}

// 발송기가 없어도(알림 모듈 미배선) 이력은 남는다.
func TestCollectStackAlerts_WorksWithoutPublisher(t *testing.T) {
	h := newCollectHarness(stackA)
	h.uc.publisher = nil
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("A")}
	h.run(t)
	open, _ := h.repo.ListOpen(context.Background(), "stk_a")
	assert.Len(t, open, 1)
}

// Alertmanager 가 재시작하면 Prometheus 가 다시 보낼 때까지 빈 목록을 준다. 잠깐 안 보였다고 해제하면
// "해제" 와 "발생" 이 한꺼번에 나간다 — 유예(3분) 동안 안 보였을 때만 해제한다.
func TestCollectStackAlerts_DoesNotResolveWithinGrace(t *testing.T) {
	h := newCollectHarness(stackA)
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	h.run(t)

	h.source.alerts["stk_a"] = nil
	h.now = h.now.Add(time.Minute)
	assert.Zero(t, h.run(t).Resolved, "1분 안 보인 것은 해제하지 않는다")

	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	h.now = h.now.Add(time.Minute)
	res := h.run(t)
	assert.Zero(t, res.Fired, "다시 보이면 같은 경보다 — 새 발생이 아니다")

	h.source.alerts["stk_a"] = nil
	h.now = h.now.Add(DefaultResolveGrace + time.Second)
	assert.Equal(t, 1, h.run(t).Resolved)
	assert.Len(t, h.pub.events, 2)
}

// 침묵 · 억제된 경보는 알리지 않지만 "보인 것" 이다 — 침묵을 걸었다고 해제 알림이 나가면 안 된다.
func TestCollectStackAlerts_SuppressedAlertsStayOpenWithoutNotifying(t *testing.T) {
	h := newCollectHarness(stackA)
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	h.run(t)

	silenced := crit("KubePodNotReady")
	silenced.Suppressed = true
	h.source.alerts["stk_a"] = []domain.SourceAlert{silenced}
	h.now = h.now.Add(DefaultResolveGrace + time.Minute)
	res := h.run(t)
	assert.Zero(t, res.Resolved)
	assert.Zero(t, res.Repeated)
	assert.Len(t, h.pub.events, 1)

	// 처음부터 침묵된 새 경보는 기록만 하고 알리지 않는다. 침묵이 풀리면 그때 처음 알린다.
	quiet := crit("NodeFilesystemAlmostOutOfSpace")
	quiet.Suppressed = true
	h.source.alerts["stk_a"] = []domain.SourceAlert{silenced, quiet}
	h.now = h.now.Add(time.Minute)
	assert.Zero(t, h.run(t).Fired)
	quiet.Suppressed = false
	h.source.alerts["stk_a"] = []domain.SourceAlert{silenced, quiet}
	h.now = h.now.Add(time.Minute)
	assert.Equal(t, 1, h.run(t).Fired)
}

// 레플리카 중 잠금을 잡은 하나만 돈다 — 서로 다른 시점의 목록으로 발생 · 해제가 엇갈리지 않게.
func TestCollectStackAlerts_OnlyLockHolderRuns(t *testing.T) {
	h := newCollectHarness(stackA)
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("KubePodNotReady")}
	lock := &fakeLock{acquire: false}
	h.uc.lock = lock

	res := h.run(t)
	assert.True(t, res.Skipped)
	assert.Empty(t, h.pub.events)

	lock.acquire = true
	res = h.run(t)
	assert.False(t, res.Skipped)
	assert.Equal(t, 1, res.Fired)
	assert.Equal(t, 1, lock.released)
}

// 발송은 수집 ctx 와 따로 간다 — 수집 시간이 다 차거나 종료 중이어도 차례를 잡은 알림은 나간다.
func TestCollectStackAlerts_PublishesWithItsOwnContext(t *testing.T) {
	h := newCollectHarness(stackA)
	h.source.alerts["stk_a"] = []domain.SourceAlert{crit("A")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.uc.Run(ctx)
	require.NoError(t, err)
	require.Len(t, h.pub.ctxErrs, 1)
	assert.NoError(t, h.pub.ctxErrs[0])
}

// 이번 주기에 본 경보는 목록 밖 스택이라도 닫지 않는다(그 사이 완료된 스택).
func TestCollectStackAlerts_DoesNotCloseAlertsSeenThisCycle(t *testing.T) {
	h := newCollectHarness(stackA)
	_, _ = h.repo.Upsert(context.Background(), &domain.StackAlert{
		StackID: "stk_new", Fingerprint: "fp", AlertName: "X", LastSeenAt: h.now,
	})
	assert.Zero(t, h.run(t).Closed)
}
